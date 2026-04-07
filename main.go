package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/priyanshu/docksmith/internal/build"
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
	default:
		printUsage()
		os.Exit(1)
	}
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
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	stateRoot := filepath.Join(home, ".docksmith")
	imagesDir := filepath.Join(stateRoot, "images")
	layersDir := filepath.Join(stateRoot, "layers")
	cacheDir := filepath.Join(stateRoot, "cache")

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
}