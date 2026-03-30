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
	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("create dest dir: %w", err)
	}

	// Open root directory - all operations confined to this root
	root, err := os.OpenRoot(destDir)
	if err != nil {
		return fmt.Errorf("open root directory: %w", err)
	}
	defer root.Close()

	tr := tar.NewReader(tarReader)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar header: %w", err)
		}

		// Use header.Name directly (relative path)
		// No manual path validation - root confines all operations

		switch header.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(header.Name, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("create dir %s: %w", header.Name, err)
			}

		case tar.TypeReg:
			// Ensure parent directory exists
			if err := root.MkdirAll(filepath.Dir(header.Name), 0755); err != nil {
				return fmt.Errorf("create parent dir for %s: %w", header.Name, err)
			}

			f, err := root.OpenFile(header.Name, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return fmt.Errorf("create file %s: %w", header.Name, err)
			}

			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return fmt.Errorf("write file %s: %w", header.Name, err)
			}
			f.Close()

		case tar.TypeSymlink:
			// Ensure parent directory exists
			if err := root.MkdirAll(filepath.Dir(header.Name), 0755); err != nil {
				return fmt.Errorf("create parent dir for symlink %s: %w", header.Name, err)
			}

			// Remove existing file/symlink if present
			root.Remove(header.Name)

			if err := root.Symlink(header.Linkname, header.Name); err != nil {
				return fmt.Errorf("create symlink %s: %w", header.Name, err)
			}

		case tar.TypeLink:
			// Hard link
			if err := root.MkdirAll(filepath.Dir(header.Name), 0755); err != nil {
				return fmt.Errorf("create parent dir for link %s: %w", header.Name, err)
			}

			// Remove existing file if present
			root.Remove(header.Name)

			if err := root.Link(header.Linkname, header.Name); err != nil {
				return fmt.Errorf("create hard link %s: %w", header.Name, err)
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
