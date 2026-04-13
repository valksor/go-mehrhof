// Package backup provides backup and restore operations for kvelmo state.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Result contains information about a completed backup.
type Result struct {
	Path  string `json:"path"`
	Size  int64  `json:"size"`
	Files int    `json:"files"`
}

// BackupInfo describes an existing backup archive.
type BackupInfo struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	CreatedAt string `json:"created_at"`
}

// Create creates a tar.gz backup archive of the given base directory.
// If outputPath is empty, a timestamped filename is generated in the current directory.
func Create(baseDir, outputPath string) (*Result, error) {
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory does not exist: %s", baseDir)
	}

	if outputPath == "" {
		outputPath = fmt.Sprintf("kvelmo-backup-%s.tar.gz", time.Now().Format("20060102-150405"))
	}

	absOutput, err := filepath.Abs(outputPath)
	if err != nil {
		return nil, fmt.Errorf("resolve output path: %w", err)
	}

	fileCount, err := writeArchive(absOutput, baseDir)
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(absOutput)
	if err != nil {
		return nil, fmt.Errorf("stat archive: %w", err)
	}

	return &Result{
		Path:  absOutput,
		Size:  stat.Size(),
		Files: fileCount,
	}, nil
}

// List returns existing backup archives in the given directory.
// It looks for files matching the pattern kvelmo-backup-*.tar.gz.
func List(dir string) ([]BackupInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("read dir: %w", err)
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "kvelmo-backup-") || !strings.HasSuffix(name, ".tar.gz") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		backups = append(backups, BackupInfo{
			Name:      name,
			Path:      filepath.Join(dir, name),
			Size:      info.Size(),
			CreatedAt: info.ModTime().Format(time.RFC3339),
		})
	}

	return backups, nil
}

// writeArchive creates the tar.gz archive and returns the file count.
func writeArchive(absOutput, baseDir string) (int, error) {
	outFile, err := os.Create(absOutput)
	if err != nil {
		return 0, fmt.Errorf("create archive: %w", err)
	}

	gzWriter := gzip.NewWriter(outFile)
	tarWriter := tar.NewWriter(gzWriter)

	fileCount := 0

	root, err := os.OpenRoot(baseDir)
	if err != nil {
		closeWriters(tarWriter, gzWriter, outFile)
		_ = os.Remove(absOutput)

		return 0, fmt.Errorf("open root directory: %w", err)
	}
	defer func() { _ = root.Close() }()

	walkErr := filepath.WalkDir(baseDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		if IsTransientFile(path) {
			return nil
		}

		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return fmt.Errorf("compute relative path: %w", err)
		}

		info, err := d.Info()
		if err != nil {
			return fmt.Errorf("stat %s: %w", relPath, err)
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("create tar header for %s: %w", relPath, err)
		}
		header.Name = relPath

		if err := tarWriter.WriteHeader(header); err != nil {
			return fmt.Errorf("write tar header for %s: %w", relPath, err)
		}

		if d.IsDir() {
			return nil
		}

		f, err := root.Open(relPath)
		if err != nil {
			return fmt.Errorf("open %s: %w", relPath, err)
		}
		defer func() { _ = f.Close() }()

		if _, err := io.Copy(tarWriter, f); err != nil {
			return fmt.Errorf("write %s to archive: %w", relPath, err)
		}

		fileCount++

		return nil
	})
	if walkErr != nil {
		closeWriters(tarWriter, gzWriter, outFile)
		_ = os.Remove(absOutput)

		return 0, fmt.Errorf("walk directory: %w", walkErr)
	}

	if err := tarWriter.Close(); err != nil {
		_ = os.Remove(absOutput)

		return 0, fmt.Errorf("finalize tar: %w", err)
	}
	if err := gzWriter.Close(); err != nil {
		_ = os.Remove(absOutput)

		return 0, fmt.Errorf("finalize gzip: %w", err)
	}
	if err := outFile.Close(); err != nil {
		_ = os.Remove(absOutput)

		return 0, fmt.Errorf("close archive: %w", err)
	}

	return fileCount, nil
}

// closeWriters closes tar, gzip, and file writers, ignoring errors (used for cleanup on failure).
func closeWriters(tw *tar.Writer, gz *gzip.Writer, f *os.File) {
	_ = tw.Close()
	_ = gz.Close()
	_ = f.Close()
}

// RestoreResult contains information about a completed restore operation.
type RestoreResult struct {
	Target  string `json:"target"`
	Files   int    `json:"files"`
	Dirs    int    `json:"dirs"`
	Skipped int    `json:"skipped"`
}

const maxRestoreFileSize = 1 << 30 // 1 GB safety limit

// Restore extracts a backup archive to the given target directory.
// If targetDir is empty, the archive is restored to the same directory structure.
func Restore(archivePath, targetDir string) (*RestoreResult, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	defer func() { _ = f.Close() }()

	gzReader, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("read gzip: %w", err)
	}
	defer func() { _ = gzReader.Close() }()

	tarReader := tar.NewReader(gzReader)

	absTarget, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, fmt.Errorf("resolve target path: %w", err)
	}

	result := &RestoreResult{Target: absTarget}

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read tar entry: %w", err)
		}

		// Safety: reject path traversal
		if strings.Contains(header.Name, "..") {
			return nil, fmt.Errorf("unsafe path in archive: %s", header.Name)
		}

		cleanName := filepath.Clean(header.Name)

		if IsTransientFile(cleanName) {
			result.Skipped++

			continue
		}

		// Skip symbolic links
		if header.Typeflag == tar.TypeSymlink || header.Typeflag == tar.TypeLink {
			result.Skipped++

			continue
		}

		// Skip oversized files
		if header.Size > maxRestoreFileSize {
			result.Skipped++

			continue
		}

		destPath := filepath.Join(absTarget, cleanName)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, 0o750); err != nil {
				return nil, fmt.Errorf("create directory %s: %w", cleanName, err)
			}
			result.Dirs++

		case tar.TypeReg:
			if err := restoreFile(tarReader, destPath, header.Size, cleanName); err != nil {
				return nil, err
			}
			result.Files++

		default:
			result.Skipped++
		}
	}

	return result, nil
}

// restoreFile extracts a single regular file from the tar stream.
func restoreFile(tarReader *tar.Reader, destPath string, size int64, cleanName string) error {
	if err := os.MkdirAll(filepath.Dir(destPath), 0o750); err != nil {
		return fmt.Errorf("create parent dir for %s: %w", cleanName, err)
	}

	perm := restoreFilePermission(cleanName)

	outFile, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("create file %s: %w", cleanName, err)
	}
	defer func() { _ = outFile.Close() }()

	if _, err := io.Copy(outFile, io.LimitReader(tarReader, size)); err != nil {
		return fmt.Errorf("extract %s: %w", cleanName, err)
	}

	return nil
}

// restoreFilePermission returns the appropriate permission for a restored file.
func restoreFilePermission(name string) os.FileMode {
	base := filepath.Base(name)
	if base == ".env" || filepath.Ext(base) == ".env" {
		return 0o600
	}

	return 0o644
}

// IsTransientFile returns true for files that should be excluded from backup/restore.
func IsTransientFile(name string) bool {
	ext := filepath.Ext(name)

	return ext == ".sock" || ext == ".lock"
}

// FormatBytes formats a byte count as a human-readable string.
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}

	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	//nolint:mnd // Standard SI binary unit suffixes
	return fmt.Sprintf("%.1f %ciB", float64(b)/float64(div), "KMGTPE"[exp])
}
