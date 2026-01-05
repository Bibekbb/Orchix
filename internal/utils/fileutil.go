package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileInfo represents file information
type FileInfo struct {
	Name    string      `json:"name"`
	Path    string      `json:"path"`
	Size    int64       `json:"size"`
	Mode    fs.FileMode `json:"mode"`
	ModTime time.Time   `json:"modTime"`
	IsDir   bool        `json:"isDir"`
	Hash    string      `json:"hash,omitempty"`
}

// CopyOptions defines options for file copying
type CopyOptions struct {
	Overwrite      bool
	PreserveMode   bool
	PreserveTimestamps bool
	BufferSize     int64
	FollowSymlinks bool
	Exclude        []string
	Include        []string
}

// DefaultCopyOptions returns default copy options
func DefaultCopyOptions() CopyOptions {
	return CopyOptions{
		Overwrite:      true,
		PreserveMode:   true,
		PreserveTimestamps: true,
		BufferSize:     32 * 1024, // 32KB
		FollowSymlinks: false,
	}
}

// CopyFile copies a file from source to destination
func CopyFile(src, dst string, options CopyOptions) error {
	// Check if source exists
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("source file not found: %w", err)
	}

	// Check if destination exists
	dstInfo, err := os.Stat(dst)
	if err == nil && !options.Overwrite {
		return fmt.Errorf("destination file already exists: %s", dst)
	}

	// If source is a directory
	if srcInfo.IsDir() {
		return CopyDirectory(src, dst, options)
	}

	// Check if source is a symlink
	if srcInfo.Mode()&os.ModeSymlink != 0 {
		if options.FollowSymlinks {
			// Follow symlink
			linkTarget, err := os.Readlink(src)
			if err != nil {
				return fmt.Errorf("failed to read symlink: %w", err)
			}
			return CopyFile(linkTarget, dst, options)
		}
		// Copy symlink itself
		return copySymlink(src, dst)
	}

	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination directory if it doesn't exist
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy content
	buffer := make([]byte, options.BufferSize)
	_, err = io.CopyBuffer(dstFile, srcFile, buffer)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	// Preserve file mode
	if options.PreserveMode {
		if err := os.Chmod(dst, srcInfo.Mode()); err != nil {
			return fmt.Errorf("failed to preserve file mode: %w", err)
		}
	}

	// Preserve timestamps
	if options.PreserveTimestamps {
		if err := os.Chtimes(dst, time.Now(), srcInfo.ModTime()); err != nil {
			return fmt.Errorf("failed to preserve timestamps: %w", err)
		}
	}

	return nil
}

// CopyDirectory copies a directory recursively
func CopyDirectory(src, dst string, options CopyOptions) error {
	// Get source info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("source directory not found: %w", err)
	}

	if !srcInfo.IsDir() {
		return fmt.Errorf("source is not a directory: %s", src)
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	// Copy each entry
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		// Check if entry should be excluded
		if shouldExclude(srcPath, options.Exclude, options.Include) {
			continue
		}

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := CopyDirectory(srcPath, dstPath, options); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := CopyFile(srcPath, dstPath, options); err != nil {
				return err
			}
		}
	}

	return nil
}

// shouldExclude checks if a path should be excluded
func shouldExclude(path string, exclude, include []string) bool {
	// Check inclusion list first
	if len(include) > 0 {
		included := false
		for _, pattern := range include {
			if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
				included = true
				break
			}
		}
		if !included {
			return true
		}
	}

	// Check exclusion list
	for _, pattern := range exclude {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}

	return false
}

// copySymlink copies a symbolic link
func copySymlink(src, dst string) error {
	target, err := os.Readlink(src)
	if err != nil {
		return fmt.Errorf("failed to read symlink: %w", err)
	}

	return os.Symlink(target, dst)
}

// File operations

// ReadFile reads a file and returns its content
func ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// WriteFile writes content to a file
func WriteFile(path string, content []byte, perm os.FileMode) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(path, content, perm)
}

// AppendToFile appends content to a file
func AppendToFile(path string, content []byte) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(content)
	return err
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsDirectory checks if a path is a directory
func IsDirectory(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// IsFile checks if a path is a regular file
func IsFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// IsSymlink checks if a path is a symbolic link
func IsSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// GetFileInfo returns file information
func GetFileInfo(path string) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	return &FileInfo{
		Name:    filepath.Base(path),
		Path:    path,
		Size:    info.Size(),
		Mode:    info.Mode(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}, nil
}

// GetFileHash calculates the SHA256 hash of a file
func GetFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Directory operations

// ListFiles lists files in a directory
func ListFiles(dir string, recursive bool) ([]string, error) {
	var files []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			files = append(files, path)
		}

		if !recursive && d.IsDir() && path != dir {
			return filepath.SkipDir
		}

		return nil
	})

	return files, err
}

// ListDirectories lists directories in a directory
func ListDirectories(dir string, recursive bool) ([]string, error) {
	var dirs []string

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && path != dir {
			dirs = append(dirs, path)
		}

		if !recursive && d.IsDir() && path != dir {
			return filepath.SkipDir
		}

		return nil
	})

	return dirs, err
}

// CreateDirectory creates a directory and all parent directories
func CreateDirectory(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// RemoveDirectory removes a directory and all its contents
func RemoveDirectory(path string) error {
	return os.RemoveAll(path)
}

// CleanDirectory removes all files and subdirectories from a directory
func CleanDirectory(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}

	return nil
}

// Path operations

// NormalizePath normalizes a file path
func NormalizePath(path string) string {
	// Clean the path
	path = filepath.Clean(path)

	// Convert to absolute path if possible
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}

	// Handle home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	return path
}

// RelativePath returns the relative path from base to target
func RelativePath(base, target string) (string, error) {
	return filepath.Rel(base, target)
}

// EnsureExtension ensures a file has the given extension
func EnsureExtension(path, ext string) string {
	if !strings.HasSuffix(path, ext) {
		return path + ext
	}
	return path
}

// ChangeExtension changes the extension of a file
func ChangeExtension(path, newExt string) string {
	oldExt := filepath.Ext(path)
	if oldExt != "" {
		path = path[:len(path)-len(oldExt)]
	}
	return path + newExt
}

// Temp file operations

// CreateTempFile creates a temporary file
func CreateTempFile(content []byte, pattern string) (string, error) {
	tmpfile, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	defer tmpfile.Close()

	if _, err := tmpfile.Write(content); err != nil {
		return "", err
	}

	return tmpfile.Name(), nil
}

// CreateTempDir creates a temporary directory
func CreateTempDir(pattern string) (string, error) {
	return os.MkdirTemp("", pattern)
}

// File search

// FindFiles finds files matching a pattern
func FindFiles(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

// FindFilesRecursive finds files recursively matching a pattern
func FindFilesRecursive(root, pattern string) ([]string, error) {
	var matches []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			matched, err := filepath.Match(pattern, info.Name())
			if err != nil {
				return err
			}
			if matched {
				matches = append(matches, path)
			}
		}

		return nil
	})

	return matches, err
}

// File comparison

// FilesEqual compares two files for equality
func FilesEqual(file1, file2 string) (bool, error) {
	hash1, err := GetFileHash(file1)
	if err != nil {
		return false, err
	}

	hash2, err := GetFileHash(file2)
	if err != nil {
		return false, err
	}

	return hash1 == hash2, nil
}

// FileSize gets the size of a file in human-readable format
func FileSize(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}

	return formatFileSize(info.Size()), nil
}

func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Backup file

// BackupFile creates a backup of a file
func BackupFile(path string) (string, error) {
	if !FileExists(path) {
		return "", fmt.Errorf("file does not exist: %s", path)
	}

	backupPath := path + ".bak"
	counter := 1
	for FileExists(backupPath) {
		backupPath = fmt.Sprintf("%s.bak.%d", path, counter)
		counter++
	}

	if err := CopyFile(path, backupPath, DefaultCopyOptions()); err != nil {
		return "", err
	}

	return backupPath, nil
}

// RestoreFile restores a file from backup
func RestoreFile(path string) error {
	backupPath := path + ".bak"
	if !FileExists(backupPath) {
		return fmt.Errorf("backup file does not exist: %s", backupPath)
	}

	return CopyFile(backupPath, path, DefaultCopyOptions())
}