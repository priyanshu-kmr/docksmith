package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/priyanshu/docksmith/internal/build"
	"github.com/priyanshu/docksmith/internal/image"
	"github.com/priyanshu/docksmith/internal/layer"
	"github.com/priyanshu/docksmith/internal/runtime"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Handle re-exec for child process isolation.
	// When RunIsolated re-executes this binary, "_ child" is the first arg.
	if os.Args[1] == "_child" {
		runtime.ChildMain(os.Args[1:])
		return
	}

	switch os.Args[1] {
	case "build":
		if err := runBuild(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	case "images":
		if err := runImages(); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	case "rmi":
		if err := runRmi(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	case "run":
		exitCode, err := runRun(os.Args[2:])
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		os.Exit(exitCode)
	default:
		printUsage()
		os.Exit(1)
	}
}

func stateDirs() (string, string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", err
	}
	stateRoot := filepath.Join(home, ".docksmith")
	return filepath.Join(stateRoot, "images"), filepath.Join(stateRoot, "layers"), filepath.Join(stateRoot, "cache"), nil
}

func runBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ContinueOnError)
	imageRef := fs.String("t", "", "image name:tag")
	noCache := fs.Bool("no-cache", false, "disable cache")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *imageRef == "" {
		return fmt.Errorf("build requires -t name:tag")
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("build requires exactly one context directory")
	}

	contextDir := fs.Arg(0)
	imagesDir, layersDir, cacheDir, err := stateDirs()
	if err != nil {
		return err
	}

	engine, err := build.NewEngine(imagesDir, layersDir, cacheDir)
	if err != nil {
		return err
	}

	_, err = engine.Build(context.Background(), build.Options{
		ImageRef: *imageRef,
		Context:  contextDir,
		NoCache:  *noCache,
	})
	return err
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  docksmith build -t <name:tag> [--no-cache] <context>")
	fmt.Println("  docksmith images")
	fmt.Println("  docksmith rmi <name:tag>")
	fmt.Println("  docksmith run [-e KEY=VALUE]... <name:tag> [cmd...]")
}

func runImages() error {
	imagesDir, _, _, err := stateDirs()
	if err != nil {
		return err
	}
	store, err := image.NewStore(imagesDir)
	if err != nil {
		return err
	}
	items, err := store.List()
	if err != nil {
		return err
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].Tag < items[j].Tag
		}
		return items[i].Name < items[j].Name
	})

	fmt.Printf("%-24s %-12s %-14s %-30s\n", "NAME", "TAG", "ID", "CREATED")
	for _, m := range items {
		id := strings.TrimPrefix(m.Digest, "sha256:")
		if len(id) > 12 {
			id = id[:12]
		}
		fmt.Printf("%-24s %-12s %-14s %-30s\n", m.Name, m.Tag, id, m.Created.Format("2006-01-02T15:04:05Z"))
	}
	return nil
}

func runRmi(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("rmi requires exactly one image reference: name:tag")
	}
	name, tag, err := splitNameTag(args[0])
	if err != nil {
		return err
	}

	imagesDir, layersDir, _, err := stateDirs()
	if err != nil {
		return err
	}
	imgStore, err := image.NewStore(imagesDir)
	if err != nil {
		return err
	}
	layerStore, err := layer.NewStore(layersDir)
	if err != nil {
		return err
	}

	m, err := imgStore.Load(name, tag)
	if err != nil {
		return err
	}
	for _, l := range m.Layers {
		_ = layerStore.Delete(strings.TrimPrefix(l.Digest, "sha256:"))
	}
	if err := imgStore.Delete(name, tag); err != nil {
		return err
	}
	fmt.Printf("Removed image %s:%s\n", name, tag)
	return nil
}

// envSlice is a flag.Value that collects repeatable -e KEY=VALUE flags.
type envSlice []string

func (e *envSlice) String() string { return strings.Join(*e, ", ") }
func (e *envSlice) Set(val string) error {
	if !strings.Contains(val, "=") {
		return fmt.Errorf("invalid env format %q, expected KEY=VALUE", val)
	}
	*e = append(*e, val)
	return nil
}

func runRun(args []string) (int, error) {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	var envFlags envSlice
	fs.Var(&envFlags, "e", "environment variable override (KEY=VALUE), repeatable")
	if err := fs.Parse(args); err != nil {
		return 1, err
	}

	remaining := fs.Args()
	if len(remaining) < 1 {
		return 1, fmt.Errorf("run requires <name:tag> [cmd...]")
	}

	name, tag, err := splitNameTag(remaining[0])
	if err != nil {
		return 1, err
	}

	imagesDir, layersDir, _, err := stateDirs()
	if err != nil {
		return 1, err
	}

	imgStore, err := image.NewStore(imagesDir)
	if err != nil {
		return 1, err
	}
	layerStore, err := layer.NewStore(layersDir)
	if err != nil {
		return 1, err
	}

	manifest, err := imgStore.Load(name, tag)
	if err != nil {
		return 1, fmt.Errorf("image %s:%s not found", name, tag)
	}

	extractor := layer.NewExtractor(layerStore)
	container := runtime.NewContainer(manifest, extractor)

	// Apply env overrides
	for _, e := range envFlags {
		parts := strings.SplitN(e, "=", 2)
		container.SetEnvOverride(parts[0], parts[1])
	}

	// Apply command override if provided
	if len(remaining) > 1 {
		container.SetCmdOverride(remaining[1:])
	}

	exitCode, err := container.Run()
	if err != nil {
		return 1, err
	}

	fmt.Printf("Container exited with code %d\n", exitCode)
	return exitCode, nil
}

func splitNameTag(ref string) (string, string, error) {
	parts := strings.Split(ref, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid image ref %q, expected name:tag", ref)
	}
	return parts[0], parts[1], nil
}