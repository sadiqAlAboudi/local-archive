//go:build !windows

package updater

import (
	"fmt"
)

// RunInstallerAndExit is a stub on non-Windows platforms.
func RunInstallerAndExit(installerPath string) error {
	return fmt.Errorf("التحديث التلقائي وتثبيت حزمة NSIS مدعوم فقط على بيئة تشغيل Windows")
}
