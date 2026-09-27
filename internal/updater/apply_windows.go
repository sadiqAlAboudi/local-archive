//go:build windows

package updater

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// RunInstallerAndExit launches the installer silently in a detached process and terminates the current app.
func RunInstallerAndExit(installerPath string) error {
	if _, err := os.Stat(installerPath); err != nil {
		return fmt.Errorf("ملف المثبت غير موجود: %w", err)
	}

	currentExe, err := os.Executable()
	if err != nil {
		currentExe = ""
	}

	batchPath := filepath.Join(os.TempDir(), "localarchive_updater.bat")
	batchScript := `@echo off
chcp 65001 >nul
ping 127.0.0.1 -n 3 >nul
start /wait "" "%~1" /S
if exist "%~2" (
    start "" "%~2"
)
del "%~f0"
`

	if err := os.WriteFile(batchPath, []byte(batchScript), 0755); err != nil {
		return fmt.Errorf("تعذر إنشاء أمر تشغيل التحديث: %w", err)
	}

	cmd := exec.Command("cmd.exe", "/c", batchPath, installerPath, currentExe)
	// CREATE_NO_WINDOW (0x08000000) and CREATE_NEW_PROCESS_GROUP (0x00000200)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000 | 0x00000200,
	}

	if err := cmd.Start(); err != nil {
		_ = os.Remove(batchPath)
		return fmt.Errorf("فشل بدء برنامج التحديث: %w", err)
	}

	// Exit the running application so NSIS installer can overwrite files
	go func() {
		time.Sleep(300 * time.Millisecond)
		os.Exit(0)
	}()

	return nil
}
