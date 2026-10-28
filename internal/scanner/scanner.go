package scanner

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// ErrScanCanceled indicates that the user dismissed or canceled the scanner dialog.
var ErrScanCanceled = errors.New("scan_canceled")

// ScannedPage represents a single scanned document page ready for frontend preview and processing.
type ScannedPage struct {
	ID       string `json:"id"`
	TempPath string `json:"temp_path"`
	Base64   string `json:"base64"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	Rotation int    `json:"rotation"`
}

// FeederScanOptions defines parameters for multi-page ADF (Automatic Document Feeder) scanning.
type FeederScanOptions struct {
	Duplex   bool `json:"duplex"`
	MaxPages int  `json:"max_pages"`
}

var (
	tempDirOnce sync.Once
	tempDirRoot string
)

// GetTempScanDir returns the dedicated directory for temporary scan files.
func GetTempScanDir() string {
	tempDirOnce.Do(func() {
		dir := filepath.Join(os.TempDir(), "localarchive_scans")
		_ = os.MkdirAll(dir, 0755)
		tempDirRoot = dir
	})
	if tempDirRoot == "" {
		return os.TempDir()
	}
	return tempDirRoot
}

// GenerateTempScanPath creates a unique file path in the temp scan directory.
func GenerateTempScanPath(ext string) string {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	randBytes := make([]byte, 8)
	_, _ = rand.Read(randBytes)
	name := fmt.Sprintf("scan_%d_%s%s", time.Now().UnixNano(), hex.EncodeToString(randBytes), ext)
	return filepath.Join(GetTempScanDir(), name)
}

// RotatePage physically rotates the image on disk by degrees clockwise (90, 180, or 270)
// and returns an updated ScannedPage with the new base64 preview and dimensions.
func RotatePage(tempPath string, degrees int) (*ScannedPage, error) {
	cleanPath := filepath.Clean(tempPath)
	if _, err := os.Stat(cleanPath); err != nil {
		return nil, fmt.Errorf("ملف الصفحة المراد تدويرها غير موجود: %w", err)
	}

	f, err := os.Open(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("تعذر فتح ملف الصفحة: %w", err)
	}

	srcImg, _, err := image.Decode(f)
	_ = f.Close()
	if err != nil {
		return nil, fmt.Errorf("تعذر قراءة بيانات صورة الصفحة: %w", err)
	}

	normDeg := ((degrees % 360) + 360) % 360
	var rotatedImg image.Image

	switch normDeg {
	case 90:
		rotatedImg = imaging.Rotate270(srcImg) // imaging 270 is 90 deg clockwise
	case 180:
		rotatedImg = imaging.Rotate180(srcImg)
	case 270:
		rotatedImg = imaging.Rotate90(srcImg) // imaging 90 is 270 deg clockwise
	default:
		rotatedImg = srcImg
	}

	// Write rotated image to a new temp file first to ensure atomic replacement
	newTempPath := GenerateTempScanPath(filepath.Ext(cleanPath))
	outFile, err := os.Create(newTempPath)
	if err != nil {
		return nil, fmt.Errorf("تعذر إنشاء ملف الصورة بعد التدوير: %w", err)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(outFile, rotatedImg, &jpeg.Options{Quality: 90}); err != nil {
		_ = outFile.Close()
		_ = os.Remove(newTempPath)
		return nil, fmt.Errorf("فشل حفظ الصورة بعد التدوير: %w", err)
	}
	_ = outFile.Close()

	// Replace original file with the rotated version
	_ = os.Remove(cleanPath)
	if err := os.Rename(newTempPath, cleanPath); err != nil {
		// If rename fails, keep newTempPath as the target
		cleanPath = newTempPath
	}

	// Generate base64 data URI for frontend preview
	_ = jpeg.Encode(&buf, rotatedImg, &jpeg.Options{Quality: 85})
	b64 := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	bounds := rotatedImg.Bounds()
	return &ScannedPage{
		ID:       filepath.Base(cleanPath),
		TempPath: cleanPath,
		Base64:   b64,
		Width:    bounds.Dx(),
		Height:   bounds.Dy(),
		Rotation: 0,
	}, nil
}

// DeleteTempScan deletes an individual temporary scan file.
func DeleteTempScan(tempPath string) error {
	cleanPath := filepath.Clean(tempPath)
	if cleanPath == "" || cleanPath == "." {
		return nil
	}
	// Safety check: ensure we only delete files within the scan directory or system temp
	scanDir := GetTempScanDir()
	if !strings.HasPrefix(cleanPath, scanDir) && !strings.HasPrefix(cleanPath, os.TempDir()) {
		return fmt.Errorf("غير مسموح بحذف ملف خارج مجلد المسح المؤقت")
	}
	if err := os.Remove(cleanPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// CleanupTempScans removes a list of temporary scan files.
func CleanupTempScans(tempPaths []string) error {
	var firstErr error
	for _, p := range tempPaths {
		if err := DeleteTempScan(p); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// CleanupOrphanedScans removes temp scan files older than 24 hours to prevent disk leak.
func CleanupOrphanedScans() error {
	scanDir := GetTempScanDir()
	entries, err := os.ReadDir(scanDir)
	if err != nil {
		return nil
	}

	threshold := time.Now().Add(-24 * time.Hour)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err == nil && info.ModTime().Before(threshold) {
			_ = os.Remove(filepath.Join(scanDir, entry.Name()))
		}
	}
	return nil
}

// SaveToPDF compiles multiple image files into a single standard A4 PDF document using pure Go pdfcpu.
// It returns the path to the generated PDF. Upon success, intermediate scanned page image files are cleaned up.
func SaveToPDF(tempPaths []string, defaultFilename string) (string, error) {
	if len(tempPaths) == 0 {
		return "", errors.New("لم يتم تحديد أي صفحات ممسوحة لتجميعها في ملف PDF")
	}

	// Validate all input paths exist
	cleanPaths := make([]string, 0, len(tempPaths))
	for _, p := range tempPaths {
		cp := filepath.Clean(strings.TrimSpace(p))
		if cp == "" {
			continue
		}
		st, err := os.Stat(cp)
		if err != nil {
			return "", fmt.Errorf("أحد ملفات الصفحات الممسوحة غير موجود: %s", cp)
		}
		if st.IsDir() {
			return "", fmt.Errorf("المسار المحدد مجلد وليس ملف صورة: %s", cp)
		}
		cleanPaths = append(cleanPaths, cp)
	}

	if len(cleanPaths) == 0 {
		return "", errors.New("قائمة الملفات المحددة فارغة")
	}

	cleanFilename := strings.TrimSpace(defaultFilename)
	if cleanFilename == "" {
		cleanFilename = "scanned_document.pdf"
	}
	if !strings.HasSuffix(strings.ToLower(cleanFilename), ".pdf") {
		cleanFilename += ".pdf"
	}

	// Create output PDF in temp scans folder
	randBytes := make([]byte, 6)
	_, _ = rand.Read(randBytes)
	outPdfName := fmt.Sprintf("doc_%d_%s_%s", time.Now().Unix(), hex.EncodeToString(randBytes), cleanFilename)
	outPdfPath := filepath.Join(GetTempScanDir(), outPdfName)

	// Ensure any stale file with this name is removed
	_ = os.Remove(outPdfPath)

	// Configure pdfcpu for standard A4 page layout
	imp := pdfcpu.DefaultImportConfig()
	imp.PageDim = types.PaperSize["A4"]
	imp.PageSize = "A4"
	imp.Pos = types.Full

	// ImportImagesFile appends all image pages to outPdfPath
	if err := api.ImportImagesFile(cleanPaths, outPdfPath, imp, nil); err != nil {
		_ = os.Remove(outPdfPath)
		return "", fmt.Errorf("فشل تجميع الصفحات في ملف PDF: %w", err)
	}

	// Verify that the generated PDF is valid and has expected size
	pdfStat, err := os.Stat(outPdfPath)
	if err != nil || pdfStat.Size() == 0 {
		_ = os.Remove(outPdfPath)
		return "", errors.New("فشل إنشاء ملف PDF أو أن حجم الملف الناتج فارغ")
	}

	// Clean up intermediate scanned image pages after successful compilation
	for _, p := range cleanPaths {
		_ = os.Remove(p)
	}

	return outPdfPath, nil
}

// MapWIAError translates WIA COM HRESULT codes and messages into clear Arabic error messages.
func MapWIAError(errOutput string, exitCode int) error {
	lower := strings.ToLower(errOutput)

	switch {
	case strings.Contains(lower, "80210064") ||
		strings.Contains(lower, "800704c7") ||
		strings.Contains(lower, "canceled") ||
		strings.Contains(lower, "cancelled") ||
		strings.Contains(lower, "cancel"):
		return ErrScanCanceled

	case strings.Contains(lower, "80210015") ||
		strings.Contains(lower, "offline") ||
		strings.Contains(lower, "not ready"):
		return errors.New("الماسح الضوئي غير متصل بالحاسوب أو غير مشغل (Offline). يرجى التأكد من توصيل الكابل وتشغيل الجهاز.")

	case strings.Contains(lower, "80210006") ||
		strings.Contains(lower, "busy"):
		return errors.New("الماسح الضوئي مشغول حالياً بعملية أخرى. يرجى الانتظار والمحاولة ثانية.")

	case strings.Contains(lower, "80210003") ||
		strings.Contains(lower, "paper empty") ||
		strings.Contains(lower, "feeder empty"):
		return errors.New("وحدة سحب الأوراق (ADF) فارغة. يرجى وضع الورق في الماسح الضوئي.")

	case strings.Contains(lower, "80210005") ||
		strings.Contains(lower, "paper jam") ||
		strings.Contains(lower, "jammed"):
		return errors.New("حدث انحشار للورق داخل الماسح الضوئي. يرجى تفقد مسار الورق وإزالته.")

	case strings.Contains(lower, "80210008") ||
		strings.Contains(lower, "warming up"):
		return errors.New("الماسح الضوئي قيد الإحماء، يرجى المحاولة بعد لحظات.")

	case strings.Contains(lower, "8021000a") ||
		strings.Contains(lower, "cover open"):
		return errors.New("غطاء الماسح الضوئي مفتوح. يرجى إغلاقه قبل البدء.")

	case strings.Contains(lower, "no device") ||
		strings.Contains(lower, "no wia device") ||
		strings.Contains(lower, "8021001a"):
		return errors.New("لم يتم العثور على أي ماسح ضوئي متصل بهذا الحاسوب. يرجى التأكد من توصيل الجهاز وتثبيت تعريفه.")

	case strings.Contains(lower, "800401f3") ||
		strings.Contains(lower, "invalid class string") ||
		strings.Contains(lower, "regdb_e_classnotreg"):
		return errors.New("تعذر تشغيل خدمة الماسح الضوئي (WIA). يرجى التأكد من تشغيل خدمة Windows Image Acquisition في إدارة الخدمات، وإعادة تثبيت تعريف الماسح إذا استمرت المشكلة.")

	default:
		// Clean technical error trace for readability
		lines := strings.Split(errOutput, "\n")
		var cleanLines []string
		for _, l := range lines {
			l = strings.TrimSpace(l)
			if l != "" && !strings.HasPrefix(l, "At line:") && !strings.HasPrefix(l, "+") {
				cleanLines = append(cleanLines, l)
			}
		}
		summary := strings.Join(cleanLines, " ")
		if summary == "" {
			summary = fmt.Sprintf("رمز الخطأ %d", exitCode)
		}
		return fmt.Errorf("خطأ أثناء الاتصال بالماسح الضوئي: %s", summary)
	}
}
