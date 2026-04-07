package build

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/priyanshu/docksmith/internal/cache"
	"github.com/priyanshu/docksmith/internal/image"
	"github.com/priyanshu/docksmith/internal/layer"
	"github.com/priyanshu/docksmith/internal/parser"
	"github.com/priyanshu/docksmith/internal/runtime"
	"github.com/priyanshu/docksmith/stubs"
)

// Options configures build behavior.
type Options struct {
	ImageRef string
	Context  string
	NoCache  bool
}

// Result returns basic build metadata.
type Result struct {
	ImageRef string
	Digest   string
}

// Engine orchestrates Docksmith builds.
type Engine struct {
	imageStore *image.Store
	layerStore *layer.Store
	extractor  *layer.Extractor
	cacheMgr   *cache.Manager
}

func NewEngine(imagesDir, layersDir, cacheDir string) (*Engine, error) {
	imgStore, err := image.NewStore(imagesDir)
	if err != nil {
		return nil, err
	}
	layerStore, err := layer.NewStore(layersDir)
	if err != nil {
		return nil, err
	}
	cacheMgr, err := cache.NewManager(cacheDir)
	if err != nil {
		return nil, err
	}
	return &Engine{
		imageStore: imgStore,
		layerStore: layerStore,
		extractor:  layer.NewExtractor(layerStore),
		cacheMgr:   cacheMgr,
	}, nil
}

func (e *Engine) Build(ctx context.Context, opts Options) (*Result, error) {
	name, tag, err := splitRef(opts.ImageRef)
	if err != nil {
		return nil, err
	}
	instructions, err := parser.ParseFile(opts.Context)
	if err != nil {
		return nil, err
	}

	manifest := image.NewManifest(name, tag)
	previousManifest, _ := e.imageStore.Load(name, tag)
	buildCtx := cache.NewBuildContext(opts.Context)

	var prevDigest cache.LayerDigest
	var forceMiss bool
	allLayerStepsHit := true

	totalSteps := len(instructions)
	buildStart := time.Now()
	for i, inst := range instructions {
		switch v := inst.(type) {
		case *stubs.FromInstruction:
			fmt.Printf("Step %d/%d : %s\n", i+1, totalSteps, inst.Text())
			base, err := e.imageStore.Load(v.Image, v.Tag)
			if err != nil {
				return nil, fmt.Errorf("FROM %s:%s: %w", v.Image, v.Tag, err)
			}
			manifest.Layers = append([]*layer.LayerInfo{}, base.Layers...)
			manifest.Config = base.Config.Clone()
			buildCtx.Workdir = manifest.Config.WorkingDir
			for k, val := range manifest.Config.Env {
				buildCtx.Env[k] = val
			}
			prevDigest = cache.LayerDigest(strings.TrimPrefix(base.Digest, "sha256:"))

		case *stubs.WorkdirInstruction:
			fmt.Printf("Step %d/%d : %s\n", i+1, totalSteps, inst.Text())
			buildCtx.ApplyWorkdir(v.Path)
			manifest.Config.SetWorkingDir(buildCtx.Workdir)

		case *stubs.EnvInstruction:
			fmt.Printf("Step %d/%d : %s\n", i+1, totalSteps, inst.Text())
			buildCtx.ApplyEnv(v.Key, v.Value)
			manifest.Config.SetEnv(v.Key, v.Value)

		case *stubs.CmdInstruction:
			fmt.Printf("Step %d/%d : %s\n", i+1, totalSteps, inst.Text())
			manifest.Config.SetCmd(v.Command)

		case *stubs.CopyInstruction, *stubs.RunInstruction:
			stepStart := time.Now()
			layerDigest, size, status, key, err := e.executeLayerStep(ctx, inst, buildCtx, prevDigest, opts.NoCache, forceMiss, manifest)
			if err != nil {
				return nil, err
			}
			if status == "CACHE MISS" {
				forceMiss = true
				allLayerStepsHit = false
			}
			manifest.AddLayer(layer.NewLayerInfo("sha256:"+string(layerDigest), size, inst.Text()))
			prevDigest = layerDigest

			if status == "CACHE MISS" && !opts.NoCache {
				entry := &cache.CacheEntry{
					Key:         key,
					LayerDigest: layerDigest,
					CreatedAt:   time.Now().UTC(),
					Size:        size,
					Metadata: cache.EntryMetadata{
						InstructionType: string(inst.Type()),
						InstructionText: inst.Text(),
					},
				}
				if err := e.cacheMgr.Store(ctx, entry); err != nil {
					return nil, fmt.Errorf("store cache entry: %w", err)
				}
			}

			fmt.Printf("Step %d/%d : %s [%s] %.2fs\n", i+1, totalSteps, inst.Text(), status, time.Since(stepStart).Seconds())
		}
	}

	if allLayerStepsHit && previousManifest != nil {
		manifest.Created = previousManifest.Created
	}

	manifest.ComputeDigest()
	if err := e.imageStore.Save(manifest); err != nil {
		return nil, fmt.Errorf("save image manifest: %w", err)
	}

	displayDigest := manifest.Digest
	if strings.HasPrefix(displayDigest, "sha256:") && len(displayDigest) >= 19 {
		displayDigest = displayDigest[:19]
	}
	fmt.Printf("Successfully built %s %s (%.2fs)\n", displayDigest, manifest.Reference(), time.Since(buildStart).Seconds())
	return &Result{ImageRef: manifest.Reference(), Digest: manifest.Digest}, nil
}

func (e *Engine) executeLayerStep(
	ctx context.Context,
	inst stubs.Instruction,
	buildCtx *cache.BuildContext,
	prevDigest cache.LayerDigest,
	noCache bool,
	forceMiss bool,
	manifest *image.Manifest,
) (cache.LayerDigest, int64, string, cache.CacheKey, error) {
	key, err := e.cacheMgr.ComputeKey(prevDigest, inst, buildCtx)
	if err != nil {
		return "", 0, "", "", fmt.Errorf("compute cache key: %w", err)
	}

	if !noCache && !forceMiss {
		entry, err := e.cacheMgr.Lookup(ctx, key)
		if err == nil {
			if e.layerStore.Exists(string(entry.LayerDigest)) {
				st, statErr := os.Stat(e.layerStore.Path(string(entry.LayerDigest)))
				if statErr != nil {
					return "", 0, "", "", statErr
				}
				return entry.LayerDigest, st.Size(), "CACHE HIT", key, nil
			}
		}
	}

	digest, size, err := e.createLayer(inst, buildCtx, manifest)
	if err != nil {
		return "", 0, "", "", err
	}
	return digest, size, "CACHE MISS", key, nil
}

func (e *Engine) createLayer(inst stubs.Instruction, buildCtx *cache.BuildContext, manifest *image.Manifest) (cache.LayerDigest, int64, error) {
	switch v := inst.(type) {
	case *stubs.CopyInstruction:
		return e.createCopyLayer(v, buildCtx)
	case *stubs.RunInstruction:
		return e.createRunLayer(v, buildCtx, manifest)
	default:
		return "", 0, fmt.Errorf("unexpected layer-producing instruction: %T", inst)
	}
}

func (e *Engine) createCopyLayer(inst *stubs.CopyInstruction, buildCtx *cache.BuildContext) (cache.LayerDigest, int64, error) {
	tmpDir, err := os.MkdirTemp("", "docksmith-layer-src-")
	if err != nil {
		return "", 0, err
	}
	defer os.RemoveAll(tmpDir)

	if err := materializeCopy(inst, buildCtx, tmpDir); err != nil {
		return "", 0, err
	}

	tarCreator := layer.NewTarCreator()
	digest, tarPath, _, err := tarCreator.CreateTar(tmpDir)
	if err != nil {
		return "", 0, fmt.Errorf("create layer tar: %w", err)
	}
	defer os.Remove(tarPath)

	storedDigest, size, err := e.layerStore.StoreFromPath(tarPath)
	if err != nil {
		return "", 0, fmt.Errorf("store layer: %w", err)
	}
	if storedDigest != digest {
		return "", 0, fmt.Errorf("layer digest mismatch")
	}
	return cache.LayerDigest(storedDigest), size, nil
}

func (e *Engine) createRunLayer(inst *stubs.RunInstruction, buildCtx *cache.BuildContext, manifest *image.Manifest) (cache.LayerDigest, int64, error) {
	// 1. Assemble the current filesystem from all accumulated layers
	rootFS, err := os.MkdirTemp("", "docksmith-run-rootfs-")
	if err != nil {
		return "", 0, fmt.Errorf("create rootfs temp dir: %w", err)
	}
	defer func() {
		runtime.CleanupRootFS(rootFS)
		os.RemoveAll(rootFS)
	}()

	digests := make([]string, len(manifest.Layers))
	for i, l := range manifest.Layers {
		digests[i] = strings.TrimPrefix(l.Digest, "sha256:")
	}

	if err := e.extractor.ExtractLayers(digests, rootFS); err != nil {
		return "", 0, fmt.Errorf("extract layers for RUN: %w", err)
	}

	// 2. Take a snapshot of the filesystem state before RUN
	beforeFiles, err := snapshotFS(rootFS)
	if err != nil {
		return "", 0, fmt.Errorf("snapshot before RUN: %w", err)
	}

	// 3. Run the command inside the assembled filesystem with isolation
	env := make(map[string]string)
	for k, v := range buildCtx.Env {
		env[k] = v
	}

	cfg := runtime.RunConfig{
		RootFS:  rootFS,
		Command: []string{"sh", "-c", inst.Command},
		Env:     env,
		WorkDir: buildCtx.Workdir,
	}

	exitCode, err := runtime.RunIsolated(cfg)
	if err != nil {
		return "", 0, fmt.Errorf("RUN %q failed: %w", inst.Command, err)
	}
	if exitCode != 0 {
		return "", 0, fmt.Errorf("RUN %q exited with code %d", inst.Command, exitCode)
	}

	// 4. Diff the filesystem to find changes (delta layer)
	deltaDir, err := os.MkdirTemp("", "docksmith-run-delta-")
	if err != nil {
		return "", 0, fmt.Errorf("create delta temp dir: %w", err)
	}
	defer os.RemoveAll(deltaDir)

	if err := computeDelta(rootFS, beforeFiles, deltaDir); err != nil {
		return "", 0, fmt.Errorf("compute delta: %w", err)
	}

	// 5. Create tar from the delta
	tarCreator := layer.NewTarCreator()
	digest, tarPath, _, err := tarCreator.CreateTar(deltaDir)
	if err != nil {
		return "", 0, fmt.Errorf("create layer tar: %w", err)
	}
	defer os.Remove(tarPath)

	storedDigest, size, err := e.layerStore.StoreFromPath(tarPath)
	if err != nil {
		return "", 0, fmt.Errorf("store layer: %w", err)
	}
	if storedDigest != digest {
		return "", 0, fmt.Errorf("layer digest mismatch")
	}
	return cache.LayerDigest(storedDigest), size, nil
}

// fileSnapshot records file metadata for diffing.
type fileSnapshot struct {
	size    int64
	modTime time.Time
	isDir   bool
}

// snapshotFS creates a snapshot of all files/dirs in a rootfs.
func snapshotFS(rootFS string) (map[string]fileSnapshot, error) {
	snapshot := make(map[string]fileSnapshot)
	err := filepath.Walk(rootFS, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		rel, err := filepath.Rel(rootFS, p)
		if err != nil {
			return nil
		}
		if rel == "." {
			return nil
		}
		// skip /proc /dev /sys /tmp mounts
		if rel == "proc" || rel == "dev" || rel == "sys" || rel == "tmp" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(rel, "proc/") || strings.HasPrefix(rel, "dev/") || strings.HasPrefix(rel, "sys/") || strings.HasPrefix(rel, "tmp/") {
			return nil
		}
		snapshot[rel] = fileSnapshot{
			size:    info.Size(),
			modTime: info.ModTime(),
			isDir:   info.IsDir(),
		}
		return nil
	})
	return snapshot, err
}

// computeDelta copies new/modified files from rootFS to deltaDir.
func computeDelta(rootFS string, before map[string]fileSnapshot, deltaDir string) error {
	return filepath.Walk(rootFS, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(rootFS, p)
		if err != nil {
			return nil
		}
		if rel == "." {
			return nil
		}
		// Skip virtual filesystems
		if rel == "proc" || rel == "dev" || rel == "sys" || rel == "tmp" {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(rel, "proc/") || strings.HasPrefix(rel, "dev/") || strings.HasPrefix(rel, "sys/") || strings.HasPrefix(rel, "tmp/") {
			return nil
		}

		prev, existed := before[rel]
		isNew := !existed
		isModified := existed && !info.IsDir() && (info.Size() != prev.size || !info.ModTime().Equal(prev.modTime))

		if isNew || isModified {
			destPath := filepath.Join(deltaDir, rel)
			if info.IsDir() {
				return os.MkdirAll(destPath, 0755)
			}
			if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
				return err
			}
			return copyFile(p, destPath)
		}

		return nil
	})
}

func materializeCopy(inst *stubs.CopyInstruction, buildCtx *cache.BuildContext, root string) error {
	sources, err := expandSources(buildCtx.ContextDir, inst.Sources)
	if err != nil {
		return err
	}
	if len(sources) == 0 {
		return fmt.Errorf("COPY matched no sources")
	}

	// Resolve destination relative to WORKDIR
	dest := inst.Dest
	if !path.IsAbs(dest) {
		dest = path.Join(buildCtx.Workdir, dest)
	}

	multiSource := len(sources) > 1
	for _, relSrc := range sources {
		srcPath := filepath.Join(buildCtx.ContextDir, relSrc)
		info, err := os.Stat(srcPath)
		if err != nil {
			return err
		}

		target, err := resolveCopyTarget(dest, relSrc, multiSource, info.IsDir())
		if err != nil {
			return err
		}
		targetPath := filepath.Join(root, target)

		if info.IsDir() {
			if err := copyDir(srcPath, targetPath); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return err
		}
		if err := copyFile(srcPath, targetPath); err != nil {
			return err
		}
	}
	return nil
}

func expandSources(contextDir string, patterns []string) ([]string, error) {
	set := map[string]struct{}{}
	for _, pattern := range patterns {
		if strings.Contains(pattern, "**") {
			root := strings.Split(pattern, "**")[0]
			walkRoot := filepath.Join(contextDir, root)
			_ = filepath.Walk(walkRoot, func(p string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(contextDir, p)
				set[rel] = struct{}{}
				return nil
			})
			continue
		}

		if strings.ContainsAny(pattern, "*?") {
			matches, err := filepath.Glob(filepath.Join(contextDir, pattern))
			if err != nil {
				return nil, err
			}
			for _, m := range matches {
				rel, _ := filepath.Rel(contextDir, m)
				set[rel] = struct{}{}
			}
			continue
		}

		// Literal file or directory
		fullPath := filepath.Join(contextDir, pattern)
		info, err := os.Stat(fullPath)
		if err != nil {
			// Could be literal path — add as is
			set[pattern] = struct{}{}
			continue
		}

		if info.IsDir() {
			// Walk the directory and add all files
			_ = filepath.Walk(fullPath, func(p string, fi os.FileInfo, err error) error {
				if err != nil || fi.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(contextDir, p)
				set[rel] = struct{}{}
				return nil
			})
		} else {
			set[pattern] = struct{}{}
		}
	}

	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out, nil
}

func resolveCopyTarget(dest string, src string, multiSource bool, srcIsDir bool) (string, error) {
	normalizedDest := strings.TrimPrefix(path.Clean(dest), "/")
	if normalizedDest == "." {
		normalizedDest = ""
	}

	if strings.HasSuffix(dest, "/") || multiSource || srcIsDir {
		return filepath.Join(normalizedDest, filepath.Base(src)), nil
	}
	return normalizedDest, nil
}

func copyDir(src, dest string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return copyFile(p, target)
	})
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(0644)
}

func splitRef(ref string) (string, string, error) {
	parts := strings.Split(ref, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid image ref %q, expected name:tag", ref)
	}
	return parts[0], parts[1], nil
}
