//go:build windows

package scanner

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
)

// ScanPage captures a document page from an attached physical scanner using Windows WIA via PowerShell COM automation.
// It returns a ScannedPage containing a base64 string for immediate frontend preview and a temporary file path.
func ScanPage() (*ScannedPage, error) {
	rawTarget := GenerateTempScanPath(".tmp")

	// Escape single quotes for PowerShell script template
	escapedTarget := strings.ReplaceAll(rawTarget, "'", "''")

	psScript := fmt.Sprintf(`[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = 'Stop'
try {
    $dialog = New-Object -ComObject WIA.CommonDialog
    # DeviceType: 1 (Scanner), Intent: 0 (Unspecified), Bias: 131072 (MaximizeQuality)
    # FormatID: "" (driver native default), AlwaysSelectDevice: $false, UseDevicePage: $true, CancelError: $true
    $image = $dialog.ShowAcquireImage(1, 0, 131072, "", $false, $true, $true)
    if ($null -eq $image) {
        Write-Output "STATUS:CANCELED"
        exit 0
    }
    $target = '%s'
    $ext = $image.FileExtension
    if (-not [string]::IsNullOrWhiteSpace($ext)) {
        $target = [System.IO.Path]::ChangeExtension($target, $ext)
    }
    if (Test-Path -LiteralPath $target) {
        Remove-Item -LiteralPath $target -Force
    }
    $image.SaveFile($target)
    Write-Output "STATUS:SUCCESS:$target"
} catch {
    $hresult = ""
    if ($_.Exception -and $_.Exception.HResult) {
        $hresult = [System.String]::Format("0x{0:X8}", $_.Exception.HResult)
    }
    Write-Output "STATUS:ERROR:$hresult:$($_.Exception.Message)"
    exit 1
}
`, escapedTarget)

	encodedCommand := encodePowerShellCommand(psScript)

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encodedCommand)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	outputStr := strings.TrimSpace(stdout.String())
	errStr := strings.TrimSpace(stderr.String())

	if strings.Contains(outputStr, "STATUS:CANCELED") {
		return nil, ErrScanCanceled
	}

	if err != nil || strings.Contains(outputStr, "STATUS:ERROR:") {
		combined := outputStr + "\n" + errStr
		exitCode := 1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		return nil, MapWIAError(combined, exitCode)
	}

	var acquiredFile string
	lines := strings.Split(outputStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "STATUS:SUCCESS:") {
			acquiredFile = strings.TrimPrefix(line, "STATUS:SUCCESS:")
			break
		}
	}

	if acquiredFile == "" {
		acquiredFile = rawTarget
	}

	// Verify acquired file exists on disk
	st, err := os.Stat(acquiredFile)
	if err != nil || st.Size() == 0 {
		_ = os.Remove(acquiredFile)
		return nil, fmt.Errorf("تعذر العثور على ملف الصورة الممسوحة ضوئياً")
	}

	// Decode the acquired image (supports BMP, JPEG, PNG, TIFF)
	file, err := os.Open(acquiredFile)
	if err != nil {
		_ = os.Remove(acquiredFile)
		return nil, fmt.Errorf("تعذر فتح ملف الصورة الممسوحة: %w", err)
	}
	img, _, err := image.Decode(file)
	_ = file.Close()
	if err != nil {
		_ = os.Remove(acquiredFile)
		return nil, fmt.Errorf("تعذر فك ترميز صورة الماسح الضوئي: %w", err)
	}

	// Normalize into a standard high-quality JPEG
	normalizedPath := GenerateTempScanPath(".jpg")
	out, err := os.Create(normalizedPath)
	if err != nil {
		_ = os.Remove(acquiredFile)
		return nil, fmt.Errorf("تعذر إنشاء ملف المسح الموحد: %w", err)
	}

	var b64Buf bytes.Buffer
	if err := jpeg.Encode(out, img, &jpeg.Options{Quality: 88}); err != nil {
		_ = out.Close()
		_ = os.Remove(acquiredFile)
		_ = os.Remove(normalizedPath)
		return nil, fmt.Errorf("فشل حفظ ملف الصورة الممسوحة: %w", err)
	}
	_ = out.Close()

	// Clean up raw acquired file if different from normalized
	if acquiredFile != normalizedPath {
		_ = os.Remove(acquiredFile)
	}

	// Encode to base64 data URI
	_ = jpeg.Encode(&b64Buf, img, &jpeg.Options{Quality: 80})
	b64Data := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(b64Buf.Bytes())

	bounds := img.Bounds()
	return &ScannedPage{
		ID:       filepath.Base(normalizedPath),
		TempPath: normalizedPath,
		Base64:   b64Data,
		Width:    bounds.Dx(),
		Height:   bounds.Dy(),
		Rotation: 0,
	}, nil
}

func encodePowerShellCommand(psScript string) string {
	utf16Runes := utf16.Encode([]rune(psScript))
	bytes := make([]byte, len(utf16Runes)*2)
	for i, r := range utf16Runes {
		binary.LittleEndian.PutUint16(bytes[i*2:], r)
	}
	return base64.StdEncoding.EncodeToString(bytes)
}
