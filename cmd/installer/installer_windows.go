//go:build windows

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

func runInstall(silent bool) error {
	appData := os.Getenv("LOCALAPPDATA")
	if appData == "" {
		appData = os.Getenv("APPDATA")
	}
	if appData == "" {
		return fmt.Errorf("could not determine user app data directory")
	}

	installDir := filepath.Join(appData, "LocalArchive")
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to create installation folder: %w", err)
	}

	// 1. Extract payload: archive.exe
	exeTarget := filepath.Join(installDir, "archive.exe")
	if err := extractPayloadFile("payload/archive.exe", exeTarget); err != nil {
		return fmt.Errorf("failed to extract executable: %w", err)
	}

	// 2. Extract payload: favicon.ico
	iconTarget := filepath.Join(installDir, "favicon.ico")
	if err := extractPayloadFile("payload/favicon.ico", iconTarget); err != nil {
		return fmt.Errorf("failed to extract icon: %w", err)
	}

	// 3. Copy running installer as uninstaller
	if currentExe, err := os.Executable(); err == nil {
		uninstallerTarget := filepath.Join(installDir, "uninstall.exe")
		_ = copyFile(currentExe, uninstallerTarget)
	}

	// 4. Create Desktop Shortcut
	userProfile := os.Getenv("USERPROFILE")
	if userProfile != "" {
		desktopShortcut := filepath.Join(userProfile, "Desktop", "الأرشيف المحلي.lnk")
		_ = createShortcut(exeTarget, desktopShortcut, iconTarget, installDir)
	}

	// 5. Create Start Menu Shortcut
	startMenuDir := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs")
	if _, err := os.Stat(startMenuDir); err == nil {
		startMenuShortcut := filepath.Join(startMenuDir, "الأرشيف المحلي.lnk")
		_ = createShortcut(exeTarget, startMenuShortcut, iconTarget, installDir)
	}

	// 6. Register automatic startup on Windows boot via Registry (Run key)
	runKey, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err == nil {
		_ = runKey.SetStringValue("LocalArchive", fmt.Sprintf(`"%s" -no-browser`, exeTarget))
		runKey.Close()
	}

	// 7. Register in Windows Programs & Features (Uninstall Registry Key)
	uninstallKeyPath := `Software\Microsoft\Windows\CurrentVersion\Uninstall\LocalArchive`
	uninstallKey, _, err := registry.CreateKey(registry.CURRENT_USER, uninstallKeyPath, registry.SET_VALUE)
	if err == nil {
		_ = uninstallKey.SetStringValue("DisplayName", "الأرشيف المحلي (Local Archive)")
		_ = uninstallKey.SetStringValue("DisplayVersion", "1.1.0")
		_ = uninstallKey.SetStringValue("Publisher", "صادق العبودي")
		_ = uninstallKey.SetStringValue("InstallLocation", installDir)
		_ = uninstallKey.SetStringValue("DisplayIcon", iconTarget)
		_ = uninstallKey.SetStringValue("UninstallString", fmt.Sprintf(`"%s" -uninstall`, filepath.Join(installDir, "uninstall.exe")))
		uninstallKey.Close()
	}

	// 8. Launch the newly installed application
	cmd := exec.Command(exeTarget)
	cmd.Dir = installDir
	_ = cmd.Start()

	// 9. Display success message box if not silent
	if !silent {
		showMessageBox(
			"الأرشيف المحلي - اكتمال التثبيت بنجاح",
			"تم تثبيت برنامج الأرشيف المحلي بنجاح على جهازك!\n\n" +
				"• تم إنشاء اختصار للأرشيف على سطح المكتب وفي قائمة ابدأ.\n" +
				"• سيبدأ البرنامج بالعمل بهدوء في الخلفية عند بدء تشغيل ويندوز.\n" +
				"• يمكنك الوصول إليه دائماً عبر: http://localhost:8080\n\n" +
				"جاري فتح واجهة الأرشيف في متصفحك الآن...",
			0x00000040, // MB_ICONINFORMATION
		)
	}

	return nil
}

func runUninstall(silent bool) error {
	appData := os.Getenv("LOCALAPPDATA")
	if appData == "" {
		appData = os.Getenv("APPDATA")
	}
	installDir := filepath.Join(appData, "LocalArchive")

	// 1. Kill any running archive.exe instance
	_ = exec.Command("taskkill", "/f", "/im", "archive.exe").Run()

	// 2. Remove Desktop Shortcut
	userProfile := os.Getenv("USERPROFILE")
	if userProfile != "" {
		_ = os.Remove(filepath.Join(userProfile, "Desktop", "الأرشيف المحلي.lnk"))
	}

	// 3. Remove Start Menu Shortcut
	startMenuDir := filepath.Join(os.Getenv("APPDATA"), "Microsoft", "Windows", "Start Menu", "Programs")
	_ = os.Remove(filepath.Join(startMenuDir, "الأرشيف المحلي.lnk"))

	// 4. Remove Windows Startup entry
	runKey, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err == nil {
		_ = runKey.DeleteValue("LocalArchive")
		runKey.Close()
	}

	// 5. Remove Uninstall Registry Key
	_ = registry.DeleteKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall\LocalArchive`)

	// 6. Delete install folder files
	_ = os.Remove(filepath.Join(installDir, "archive.exe"))
	_ = os.Remove(filepath.Join(installDir, "favicon.ico"))

	// 7. Schedule self-deletion of uninstall.exe and directory
	selfDeleteCmd := fmt.Sprintf("timeout /t 2 /nobreak > NUL & rmdir /s /q \"%s\"", installDir)
	_ = exec.Command("cmd", "/c", selfDeleteCmd).Start()

	if !silent {
		showMessageBox(
			"الأرشيف المحلي - إزالة التثبيت",
			"تمت إزالة تثبيت برنامج الأرشيف المحلي بنجاح من جهازك.\n\nملاحظة: تم الحفاظ على مجلد البيانات data/ لحماية وثائقك من الحذف.",
			0x00000040,
		)
	}

	return nil
}

func extractPayloadFile(embeddedPath, targetPath string) error {
	data, err := payloadFS.ReadFile(embeddedPath)
	if err != nil {
		return err
	}
	return os.WriteFile(targetPath, data, 0755)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func createShortcut(targetPath, shortcutPath, iconPath, workingDir string) error {
	vbs := fmt.Sprintf(`Set oWS = CreateObject("WScript.Shell")
Set oLink = oWS.CreateShortcut("%s")
oLink.TargetPath = "%s"
oLink.WorkingDirectory = "%s"
oLink.IconLocation = "%s"
oLink.Save
`, shortcutPath, targetPath, workingDir, iconPath)

	tempVBS := filepath.Join(os.TempDir(), fmt.Sprintf("mkshortcut_%d.vbs", time.Now().UnixNano()))
	defer os.Remove(tempVBS)
	if err := os.WriteFile(tempVBS, []byte(vbs), 0644); err != nil {
		return err
	}
	return exec.Command("wscript", tempVBS).Run()
}

func showMessageBox(title, message string, flags uint32) {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBoxW := user32.NewProc("MessageBoxW")
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	messagePtr, _ := syscall.UTF16PtrFromString(message)
	messageBoxW.Call(0, uintptr(unsafe.Pointer(messagePtr)), uintptr(unsafe.Pointer(titlePtr)), uintptr(flags))
}
