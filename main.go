package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

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
	case "ps":
		if err := runPs(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	case "start":
		exitCode, err := runStart(os.Args[2:])
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
		os.Exit(exitCode)
	case "rm":
		if err := runRm(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "Error:", err)
			os.Exit(1)
		}
	default:
		printUsage()
		os.Exit(1)
	}
}

func stateDirs() (string, string, string, string, string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", "", "", err
	}
	stateRoot := filepath.Join(home, ".docksmith")
	return filepath.Join(stateRoot, "images"),
		filepath.Join(stateRoot, "layers"),
		filepath.Join(stateRoot, "cache"),
		filepath.Join(stateRoot, "containers"),
		filepath.Join(stateRoot, "layerfs"),
		nil
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
	imagesDir, layersDir, cacheDir, _, _, err := stateDirs()
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
	fmt.Println("  docksmith run [--name CONTAINER_NAME] [-e KEY=VALUE]... <name:tag> [cmd...]")
	fmt.Println("  docksmith ps [-a]")
	fmt.Println("  docksmith start <container-id|container-name>")
	fmt.Println("  docksmith rm <container-id|container-name> [...]")
}

func runImages() error {
	imagesDir, _, _, _, _, err := stateDirs()
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

	imagesDir, layersDir, _, _, _, err := stateDirs()
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
	containerName := fs.String("name", "", "container name")
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
	if *containerName != "" {
		if err := validateContainerName(*containerName); err != nil {
			return 1, err
		}
	}

	imagesDir, layersDir, _, containersDir, layerfsDir, err := stateDirs()
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
	container := runtime.NewContainer(manifest, extractor, containersDir, layerfsDir)
	if *containerName != "" {
		container.SetNameOverride(*containerName)
	}

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

var containerNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

func validateContainerName(name string) error {
	if !containerNamePattern.MatchString(name) {
		return fmt.Errorf("invalid container name %q: use [a-zA-Z0-9_.-], starting with alphanumeric", name)
	}
	return nil
}

func splitNameTag(ref string) (string, string, error) {
	parts := strings.Split(ref, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid image ref %q, expected name:tag", ref)
	}
	return parts[0], parts[1], nil
}

func runPs(args []string) error {
	fs := flag.NewFlagSet("ps", flag.ContinueOnError)
	all := fs.Bool("a", false, "show all containers")
	if err := fs.Parse(args); err != nil {
		return err
	}

	_, _, _, containersDir, _, err := stateDirs()
	if err != nil {
		return err
	}
	items, err := runtime.ListContainerStatuses(containersDir)
	if err != nil {
		return err
	}

	fmt.Printf("%-10s %-18s %-24s %-26s %-22s\n", "ID", "IMAGE", "COMMAND", "STATUS", "NAME")
	for _, c := range items {
		if !*all && !c.Running {
			continue
		}
		fmt.Printf(
			"%-10s %-18s %-24s %-26s %-22s\n",
			c.ID,
			truncate(c.Image, 18),
			truncate(strings.Join(c.Command, " "), 24),
			truncate(formatContainerStatus(c), 26),
			truncate(c.Name, 22),
		)
	}
	return nil
}

type persistedContainerConfig struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Image   string   `json:"image"`
	Command []string `json:"command"`
	PID     int      `json:"pid,omitempty"`
}

func loadContainerConfigByIdentifier(containersDir, target string) (*persistedContainerConfig, error) {
	entries, err := os.ReadDir(containersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("container %q not found", target)
		}
		return nil, fmt.Errorf("read containers dir: %w", err)
	}

	var match *persistedContainerConfig
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cfgPath := filepath.Join(containersDir, e.Name(), "config.json")
		raw, readErr := os.ReadFile(cfgPath)
		if readErr != nil {
			continue
		}
		var cfg persistedContainerConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			continue
		}
		if cfg.Name == "" && cfg.ID != "" {
			cfg.Name = "ctr-" + cfg.ID
		}
		if cfg.ID != target && cfg.Name != target {
			continue
		}
		if match != nil {
			return nil, fmt.Errorf("container %q is ambiguous", target)
		}
		cfgCopy := cfg
		match = &cfgCopy
	}

	if match == nil {
		return nil, fmt.Errorf("container %q not found", target)
	}
	return match, nil
}

func runStart(args []string) (int, error) {
	fs := flag.NewFlagSet("start", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return 1, err
	}
	if fs.NArg() != 1 {
		return 1, fmt.Errorf("start requires exactly one argument: <container-id|container-name>")
	}
	target := fs.Arg(0)

	imagesDir, layersDir, _, containersDir, layerfsDir, err := stateDirs()
	if err != nil {
		return 1, err
	}

	cfg, err := loadContainerConfigByIdentifier(containersDir, target)
	if err != nil {
		return 1, err
	}
	if cfg.PID > 0 {
		return 1, fmt.Errorf("container %q appears to be running (pid %d)", cfg.ID, cfg.PID)
	}

	name, tag, err := splitNameTag(cfg.Image)
	if err != nil {
		return 1, fmt.Errorf("invalid image reference in container config: %q", cfg.Image)
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
		return 1, fmt.Errorf("image %s not found for container %q", cfg.Image, cfg.ID)
	}

	extractor := layer.NewExtractor(layerStore)
	container := runtime.NewContainer(manifest, extractor, containersDir, layerfsDir)
	if len(cfg.Command) > 0 {
		container.SetCmdOverride(cfg.Command)
	}

	exitCode, err := container.Run()
	if err != nil {
		return 1, err
	}
	fmt.Printf("Container exited with code %d\n", exitCode)
	return exitCode, nil
}

func selectorMatchesContainer(selector string, c runtime.ContainerStatus) bool {
	return c.ID == selector || c.Name == selector
}

func runRm(args []string) error {
	fs := flag.NewFlagSet("rm", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("rm requires at least one selector")
	}

	_, _, _, containersDir, _, err := stateDirs()
	if err != nil {
		return err
	}
	items, err := runtime.ListContainerStatuses(containersDir)
	if err != nil {
		return err
	}

	matchedIDs := make(map[string]runtime.ContainerStatus)
	for _, selector := range fs.Args() {
		selectorMatched := false
		for _, c := range items {
			if selectorMatchesContainer(selector, c) {
				selectorMatched = true
				matchedIDs[c.ID] = c
			}
		}
		if !selectorMatched {
			return fmt.Errorf("no containers matched selector %q", selector)
		}
	}

	for _, c := range matchedIDs {
		if c.Running {
			return fmt.Errorf("cannot remove running container %q", c.ID)
		}
	}

	ids := make([]string, 0, len(matchedIDs))
	for id := range matchedIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		if err := os.RemoveAll(filepath.Join(containersDir, id)); err != nil {
			return fmt.Errorf("remove container %q: %w", id, err)
		}
		fmt.Printf("Removed container %s\n", id)
	}
	return nil
}

func formatContainerStatus(c runtime.ContainerStatus) string {
	now := time.Now().UTC()
	if c.Running {
		if c.StartedAt != nil {
			return "running " + humanDuration(now.Sub(*c.StartedAt))
		}
		return "running"
	}
	if c.ExitCode != nil {
		end := now
		if c.FinishedAt != nil {
			end = *c.FinishedAt
		}
		if c.StartedAt != nil {
			return fmt.Sprintf("exited(%d) %s", *c.ExitCode, humanDuration(now.Sub(end)))
		}
		return fmt.Sprintf("exited(%d)", *c.ExitCode)
	}
	return "created"
}

func humanDuration(d time.Duration) string {
	if d < time.Minute {
		sec := int(d.Seconds())
		if sec < 1 {
			sec = 1
		}
		return fmt.Sprintf("%ds ago", sec)
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}
