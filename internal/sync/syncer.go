package sync

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/glidingnz/igc-sync/internal/api"
)

// LocalState maps local file paths (relative to outputDir) to their SHA-256 hash.
type LocalState map[string]string

// DiffResult holds files that need to be downloaded or re-downloaded.
type DiffResult struct {
	New     []api.IgcFile // files not yet present locally
	Updated []api.IgcFile // files where the hash has changed
}

// LocalPath returns the relative path for a file within the output directory.
// It sanitizes the filename to prevent path traversal attacks.
func LocalPath(f api.IgcFile) string {
	return filepath.Base(f.Filename)
}

// ScanLocal scans outputDir recursively for .igc files and computes their SHA-256 hash.
// Returns a map of relative path → hex hash.
func ScanLocal(outputDir string) (LocalState, error) {
	state := make(LocalState)

	err := filepath.WalkDir(outputDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}

		hash, err := hashFile(path)
		if err != nil {
			return fmt.Errorf("hashing %s: %w", path, err)
		}

		rel, err := filepath.Rel(outputDir, path)
		if err != nil {
			return err
		}
		state[rel] = hash
		return nil
	})

	return state, err
}

// Diff compares remote files against local state and returns what needs downloading.
func Diff(remote []api.IgcFile, local LocalState) DiffResult {
	var result DiffResult
	for _, f := range remote {
		rel := LocalPath(f)
		localHash, exists := local[rel]
		if !exists {
			result.New = append(result.New, f)
		} else if f.FileHash != "" && localHash != f.FileHash {
			result.Updated = append(result.Updated, f)
		}
	}
	return result
}

// Download downloads a single IGC file to outputDir/{filename},
// creating the directory as needed, and verifies the SHA-256 hash afterwards.
func Download(f api.IgcFile, outputDir string, client *http.Client) error {
	destPath := filepath.Join(outputDir, LocalPath(f))

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("creating directory for %s: %w", f.Filename, err)
	}

	resp, err := client.Get(f.URL)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", f.Filename, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s returned HTTP %d", f.Filename, resp.StatusCode)
	}

	tmpPath := destPath + ".tmp"
	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating temp file for %s: %w", f.Filename, err)
	}

	hasher := sha256.New()
	writer := io.MultiWriter(file, hasher)

	if _, err := io.Copy(writer, resp.Body); err != nil {
		file.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing %s: %w", f.Filename, err)
	}
	file.Close()

	// Verify hash if the API provided one.
	if f.FileHash != "" {
		gotHash := hex.EncodeToString(hasher.Sum(nil))
		if gotHash != f.FileHash {
			os.Remove(tmpPath)
			return fmt.Errorf("hash mismatch for %s: expected %s, got %s", f.Filename, f.FileHash, gotHash)
		}
	}

	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file for %s: %w", f.Filename, err)
	}

	return nil
}

// hashFile computes the SHA-256 hex digest of the file at path.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
