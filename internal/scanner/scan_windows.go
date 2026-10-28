//go:build windows

package scanner

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
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

// normalizeAcquiredImage decodes the raw image from the scanner, converts it to standard JPEG,
// generates a base64 preview, and removes the raw file.
func normalizeAcquiredImage(acquiredFile string) (*ScannedPage, error) {
	st, err := os.Stat(acquiredFile)
	if err != nil || st.Size() == 0 {
		_ = os.Remove(acquiredFile)
		return nil, fmt.Errorf("تعذر العثور على ملف الصورة الممسوحة ضوئياً")
	}

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

	if acquiredFile != normalizedPath {
		_ = os.Remove(acquiredFile)
	}

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

// ScanPage captures a single document page (Flatbed mode) using Windows WIA.
func ScanPage() (*ScannedPage, error) {
	rawTarget := GenerateTempScanPath(".tmp")
	escapedTarget := strings.ReplaceAll(rawTarget, "'", "''")

	psScript := fmt.Sprintf(`[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = 'Stop'
try {
    $dialog = New-Object -ComObject WIA.CommonDialog
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
    Write-Output "STATUS:ERROR:${hresult}:$($_.Exception.Message)"
    exit 1
}
`, escapedTarget)

	encodedCommand := encodePowerShellCommand(psScript)

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encodedCommand)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
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

	return normalizeAcquiredImage(acquiredFile)
}

// ScanFeeder captures multiple document pages continuously from an Automatic Document Feeder (ADF).
// It streams pages in real time via onPageCallback as each sheet is scanned.
func ScanFeeder(opts FeederScanOptions, onPageCallback func(*ScannedPage)) ([]*ScannedPage, error) {
	tempDir := GetTempScanDir()
	escapedTempDir := strings.ReplaceAll(tempDir, "'", "''")

	duplexArg := "$false"
	if opts.Duplex {
		duplexArg = "$true"
	}
	maxPages := opts.MaxPages
	if maxPages <= 0 {
		maxPages = 100
	}

	psScript := fmt.Sprintf(`[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$ErrorActionPreference = 'Stop'
try {
    $dialog = New-Object -ComObject WIA.CommonDialog
    $device = $dialog.ShowSelectDevice(1, $false, $true)
    if ($null -eq $device) {
        [Console]::Out.WriteLine("STATUS:CANCELED")
        exit 0
    }

    $docSelectProp = $null
    $pagesProp = $null

    foreach ($prop in $device.Properties) {
        if ($prop.PropertyID -eq 3088) {
            $docSelectProp = $prop
        }
        if ($prop.PropertyID -eq 3096) {
            $pagesProp = $prop
        }
    }

    if ($null -ne $docSelectProp) {
        $selectVal = 1 # FEEDER
        if (%s) {
            $selectVal = 5 # FEEDER (1) | DUPLEX (4)
        }
        try {
            $docSelectProp.Value = $selectVal
        } catch {
            $docSelectProp.Value = 1
        }
    }

    if ($null -ne $pagesProp) {
        try { $pagesProp.Value = 1 } catch {}
    }

    $item = $device.Items.Item(1)
    $maxPages = %d
    $pageCount = 0

    while ($pageCount -lt $maxPages) {
        try {
            $image = $dialog.ShowTransfer($item, "", $false)
            if ($null -eq $image) { break }
            $pageCount++
            $pageTarget = [System.IO.Path]::Combine('%s', "feeder_raw_" + [System.Guid]::NewGuid().ToString("N") + ".tmp")
            $ext = $image.FileExtension
            if (-not [string]::IsNullOrWhiteSpace($ext)) {
                $pageTarget = [System.IO.Path]::ChangeExtension($pageTarget, $ext)
            }
            if (Test-Path -LiteralPath $pageTarget) { Remove-Item -LiteralPath $pageTarget -Force }
            $image.SaveFile($pageTarget)
            [Console]::Out.WriteLine("PAGE:" + $pageTarget)
            [Console]::Out.Flush()
        } catch {
            $hr = 0
            if ($_.Exception -and $_.Exception.HResult) {
                $hr = $_.Exception.HResult
            }
            # 0x80210003: WIA_ERROR_PAPER_EMPTY (-2145320957)
            if ($hr -eq -2145320957 -or $_.Exception.Message -match "80210003" -or $_.Exception.Message -match "paper empty") {
                [Console]::Out.WriteLine("STATUS:FEEDER_EMPTY")
                [Console]::Out.Flush()
                break
            }
            # 0x80210005: WIA_ERROR_PAPER_JAM (-2145320955)
            if ($hr -eq -2145320955 -or $_.Exception.Message -match "80210005" -or $_.Exception.Message -match "jam") {
                [Console]::Out.WriteLine("STATUS:JAM")
                [Console]::Out.Flush()
                break
            }
            $hresultStr = [System.String]::Format("0x{0:X8}", $hr)
            [Console]::Out.WriteLine("STATUS:ERROR:" + $hresultStr + ":" + $_.Exception.Message)
            [Console]::Out.Flush()
            exit 1
        }
    }
    [Console]::Out.WriteLine("STATUS:COMPLETED")
    [Console]::Out.Flush()
} catch {
    $hr = 0
    if ($_.Exception -and $_.Exception.HResult) {
        $hr = $_.Exception.HResult
    }
    $hresultStr = [System.String]::Format("0x{0:X8}", $hr)
    [Console]::Out.WriteLine("STATUS:ERROR:" + $hresultStr + ":" + $_.Exception.Message)
    [Console]::Out.Flush()
    exit 1
}
`, duplexArg, maxPages, escapedTempDir)

	encodedCommand := encodePowerShellCommand(psScript)

	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-EncodedCommand", encodedCommand)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000,
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("تعذر قراءة مخرجات الماسح: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("فشل بدء عملية المسح: %w", err)
	}

	var pages []*ScannedPage
	var lastErr error
	isCanceled := false
	sawPaperJam := false

	reader := bufio.NewReader(stdout)
	for {
		line, readErr := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "PAGE:") {
			rawPath := strings.TrimPrefix(line, "PAGE:")
			if page, normErr := normalizeAcquiredImage(rawPath); normErr == nil {
				pages = append(pages, page)
				if onPageCallback != nil {
					onPageCallback(page)
				}
			}
		} else if line == "STATUS:FEEDER_EMPTY" {
			// Normal end of stack in feeder tray
			break
		} else if line == "STATUS:JAM" {
			sawPaperJam = true
			break
		} else if line == "STATUS:CANCELED" {
			isCanceled = true
			break
		} else if strings.HasPrefix(line, "STATUS:ERROR:") {
			errPayload := strings.TrimPrefix(line, "STATUS:ERROR:")
			lastErr = MapWIAError(errPayload, 1)
			break
		}

		if readErr != nil {
			break
		}
	}

	_ = cmd.Wait()

	if isCanceled && len(pages) == 0 {
		return nil, ErrScanCanceled
	}

	if len(pages) == 0 {
		if lastErr != nil {
			return nil, lastErr
		}
		if sawPaperJam {
			return nil, errors.New("حدث انحشار للورق في وحدة التغذية التلقائية. يرجى إزالة الورق المنحشر وإعادة المحاولة.")
		}
		return nil, errors.New("وحدة سحب الأوراق (ADF) فارغة. يرجى وضع حزمة الأوراق في الماسح والتأكد من دعم الماسح لوحدة التغذية.")
	}

	return pages, nil
}

func encodePowerShellCommand(psScript string) string {
	utf16Runes := utf16.Encode([]rune(psScript))
	bytes := make([]byte, len(utf16Runes)*2)
	for i, r := range utf16Runes {
		binary.LittleEndian.PutUint16(bytes[i*2:], r)
	}
	return base64.StdEncoding.EncodeToString(bytes)
}
