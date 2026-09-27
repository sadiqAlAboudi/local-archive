package sysutil

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// AppDataPaths holds all persistent file system paths.
type AppDataPaths struct {
	BaseDir   string
	DBPath    string
	UploadDir string
}

// ResolveDataPaths determines the data directory path.
// Production on Windows: %LOCALAPPDATA%\LocalArchive
// Fallback / Development: ./data or ~/.local/share/local-archive
func ResolveDataPaths(isDev bool) (AppDataPaths, error) {
	var baseDir string

	if isDev {
		baseDir = filepath.Join(".", "data")
	} else if runtime.GOOS == "windows" {
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			baseDir = filepath.Join(localAppData, "LocalArchive")
		} else {
			userConfig, _ := os.UserConfigDir()
			baseDir = filepath.Join(userConfig, "LocalArchive")
		}
	} else {
		userConfig, _ := os.UserConfigDir()
		if userConfig != "" {
			baseDir = filepath.Join(userConfig, "local-archive")
		} else {
			baseDir = filepath.Join(".", "data")
		}
	}

	paths := AppDataPaths{
		BaseDir:   baseDir,
		DBPath:    filepath.Join(baseDir, "archive.db"),
		UploadDir: filepath.Join(baseDir, "uploads"),
	}

	// Ensure directories exist
	if err := os.MkdirAll(paths.UploadDir, 0755); err != nil {
		return paths, fmt.Errorf("failed to create data directory at %s: %w", paths.UploadDir, err)
	}

	return paths, nil
}

// GetDataDir determines the appropriate data directory path (legacy compatibility).
func GetDataDir() string {
	paths, err := ResolveDataPaths(false)
	if err == nil {
		return paths.BaseDir
	}

	if cwd, err := os.Getwd(); err == nil {
		return filepath.Join(cwd, "data")
	}

	return filepath.Join(".", "data")
}

// FormatBytes formats byte sizes into Arabic units.
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d بايت", b)
	}
	div, exp := int64(unit), 0
	units := []string{"كيلوبايت", "ميجابايت", "جيجابايت", "تيرابايت"}
	for n := b / unit; n >= unit && exp < len(units)-1; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), units[exp])
}

// SanitizeFilename removes illegal filename characters.
func SanitizeFilename(name string) string {
	invalid := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|", "\r", "\n", "\t"}
	for _, inv := range invalid {
		name = strings.ReplaceAll(name, inv, "-")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "وثيقة"
	}
	return name
}
