	package layer

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Extractor handles layer extraction
type Extractor struct {
	store *Store
}

// NewExtractor creates a new Extractor
func NewExtractor(store *Store) *Extractor {
	return &Extractor{store: store}
}

// Extract extracts a tar archive to a destination directory
func Extract(tarReader io.Reader, destDir string) error {
	tr := tar.NewReader(tarReader)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar header: %w", err)
		}

		// Construct full path
		target := filepath.Join(destDir, header.Name)

		// Ensure we don't escape the destination directory
		if !filepath.HasPrefix(target, filepath.Clean(destDir)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid tar path: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create dir %s: %w", target, err)
			}

		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("create parent dir for %s: %w", target, err)
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file %s: %w", target, err)
			}

			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("write file %s: %w", target, err)
			}
			f.Close()

		case tar.TypeSymlink:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("create parent dir for symlink %s: %w", target, err)
			}

			// Remove existing file/symlink if present
			os.Remove(target)

			if err := os.Symlink(header.Linkname, target); err != nil {
				return fmt.Errorf("create symlink %s: %w", target, err)
			}

		case tar.TypeLink:
			// Hard link
			linkTarget := filepath.Join(destDir, header.Linkname)
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("create parent dir for link %s: %w", target, err)
			}

			// Remove existing file if present
			os.Remove(target)

			if err := os.Link(linkTarget, target); err != nil {
				return fmt.Errorf("create hard link %s: %w", target, err)
			}
		}
	}

	return nil
}

// ExtractLayer extracts a single layer by digest to the destination directory
func (e *Extractor) ExtractLayer(digest string, destDir string) error {
	reader, err := e.store.Get(digest)
	if err != nil {
		return fmt.Errorf("get layer %s: %w", digest, err)
	}
	defer reader.Close()

	return Extract(reader, destDir)
}

// ExtractLayers extracts multiple layers in order to the destination directory
// Layers are applied in order, with later layers overwriting earlier ones
func (e *Extractor) ExtractLayers(digests []string, destDir string) error {
	// Ensure destination exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	for _, digest := range digests {
		if err := e.ExtractLayer(digest, destDir); err != nil {
			return fmt.Errorf("extract layer %s: %w", digest, err)
		}
	}

	return nil
}
