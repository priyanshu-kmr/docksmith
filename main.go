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
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
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

func splitNameTag(ref string) (string, string, error) {
	parts := strings.Split(ref, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid image ref %q, expected name:tag", ref)
	}
	return parts[0], parts[1], nil
}