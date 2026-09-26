package sysutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// GetDataDir determines the appropriate data directory path.
func GetDataDir() string {
	if exePath, err := os.Executable(); err == nil {
		clean := filepath.Clean(exePath)
		// "go run" executes binaries from temporary build directories
		if !strings.Contains(clean, "go-build") && !strings.HasPrefix(clean, os.TempDir()) {
			return filepath.Join(filepath.Dir(clean), "data")
		}
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

// OpenBrowserURL opens the specified URL in the system's default browser.
func OpenBrowserURL(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// ConfigureWindowsStartup adds or removes a startup script in the Windows Startup folder.
func ConfigureWindowsStartup(enable bool) error {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		return fmt.Errorf("APPDATA environment variable not found")
	}

	startupDir := filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup")
	vbsPath := filepath.Join(startupDir, "LocalArchive.vbs")

	if !enable {
		return os.Remove(vbsPath)
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	vbsContent := fmt.Sprintf(`Set WshShell = CreateObject("WScript.Shell")
WshShell.CurrentDirectory = "%s"
WshShell.Run """%s"" -no-browser", 0, False
`, filepath.Dir(exePath), exePath)

	return os.WriteFile(vbsPath, []byte(vbsContent), 0644)
}
