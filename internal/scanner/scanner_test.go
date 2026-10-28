package scanner

import (
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func createTestImage(t *testing.T, width, height int, col color.Color) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{col}, image.Point{}, draw.Src)

	p := GenerateTempScanPath(".jpg")
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("failed to create test image file: %v", err)
	}
	defer f.Close()

	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("failed to encode test jpeg: %v", err)
	}
	return p
}

func TestScanPage(t *testing.T) {
	page, err := ScanPage()
	if err != nil {
		t.Fatalf("ScanPage() returned error: %v", err)
	}
	defer DeleteTempScan(page.TempPath)

	if page.TempPath == "" {
		t.Errorf("expected non-empty TempPath")
	}
	if page.Base64 == "" {
		t.Errorf("expected non-empty Base64")
	}
	if page.Width <= 0 || page.Height <= 0 {
		t.Errorf("expected positive dimensions, got %dx%d", page.Width, page.Height)
	}
	if _, err := os.Stat(page.TempPath); err != nil {
		t.Errorf("expected temporary file to exist on disk: %v", err)
	}
}

func TestScanFeeder(t *testing.T) {
	callbackCount := 0
	pages, err := ScanFeeder(FeederScanOptions{Duplex: false, MaxPages: 2}, func(p *ScannedPage) {
		callbackCount++
		if p.TempPath == "" || p.Base64 == "" {
			t.Errorf("received invalid ScannedPage in callback")
		}
	})
	if err != nil {
		t.Fatalf("ScanFeeder failed: %v", err)
	}
	defer func() {
		for _, p := range pages {
			DeleteTempScan(p.TempPath)
		}
	}()

	if len(pages) != 2 {
		t.Errorf("expected 2 pages, got %d", len(pages))
	}
	if callbackCount != 2 {
		t.Errorf("expected callback to be called 2 times, got %d", callbackCount)
	}
}

func TestRotatePage(t *testing.T) {
	// Create an asymmetric image (300 wide, 150 high)
	p := createTestImage(t, 300, 150, color.RGBA{0, 100, 200, 255})
	defer DeleteTempScan(p)

	// Rotate 90 degrees
	rotated, err := RotatePage(p, 90)
	if err != nil {
		t.Fatalf("RotatePage(90) failed: %v", err)
	}

	// Width and height should be swapped (150 wide, 300 high)
	if rotated.Width != 150 || rotated.Height != 300 {
		t.Errorf("expected rotated dimensions 150x300, got %dx%d", rotated.Width, rotated.Height)
	}
	if rotated.Base64 == "" {
		t.Errorf("expected non-empty Base64 after rotation")
	}

	// Rotate another 90 degrees (180 total from original)
	rotated2, err := RotatePage(rotated.TempPath, 90)
	if err != nil {
		t.Fatalf("second RotatePage(90) failed: %v", err)
	}
	if rotated2.Width != 300 || rotated2.Height != 150 {
		t.Errorf("expected rotated dimensions 300x150, got %dx%d", rotated2.Width, rotated2.Height)
	}
}

func TestSaveToPDF(t *testing.T) {
	// Create 3 test pages
	p1 := createTestImage(t, 600, 800, color.RGBA{255, 0, 0, 255})
	p2 := createTestImage(t, 600, 800, color.RGBA{0, 255, 0, 255})
	p3 := createTestImage(t, 600, 800, color.RGBA{0, 0, 255, 255})

	pdfPath, err := SaveToPDF([]string{p1, p2, p3}, "custom_contract")
	if err != nil {
		t.Fatalf("SaveToPDF failed: %v", err)
	}
	defer os.Remove(pdfPath)

	// Intermediate page files should have been cleaned up automatically
	if _, err := os.Stat(p1); !os.IsNotExist(err) {
		t.Errorf("expected p1 to be cleaned up after SaveToPDF")
	}

	// Verify compiled PDF exists and is non-empty
	st, err := os.Stat(pdfPath)
	if err != nil {
		t.Fatalf("stat on generated PDF failed: %v", err)
	}
	if st.Size() == 0 {
		t.Fatalf("generated PDF is empty")
	}

	// Verify PDF page count with pdfcpu
	pageCount, err := api.PageCountFile(pdfPath)
	if err != nil {
		t.Fatalf("failed to read page count of generated PDF: %v", err)
	}
	if pageCount != 3 {
		t.Errorf("expected 3 pages in compiled PDF, got %d", pageCount)
	}

	// Verify filename ends with .pdf
	if filepath.Ext(pdfPath) != ".pdf" {
		t.Errorf("expected .pdf extension, got %s", filepath.Ext(pdfPath))
	}
}

func TestSaveToPDF_Errors(t *testing.T) {
	// Empty slice
	_, err := SaveToPDF([]string{}, "doc.pdf")
	if err == nil {
		t.Errorf("expected error for empty slice")
	}

	// Non-existent file
	_, err = SaveToPDF([]string{"/path/to/nonexistent/file.jpg"}, "doc.pdf")
	if err == nil {
		t.Errorf("expected error for non-existent file")
	}
}

func TestDeleteAndCleanupTempScans(t *testing.T) {
	p1 := createTestImage(t, 100, 100, color.White)
	p2 := createTestImage(t, 100, 100, color.White)

	if err := DeleteTempScan(p1); err != nil {
		t.Errorf("DeleteTempScan failed: %v", err)
	}
	if _, err := os.Stat(p1); !os.IsNotExist(err) {
		t.Errorf("file p1 should have been deleted")
	}

	if err := CleanupTempScans([]string{p2}); err != nil {
		t.Errorf("CleanupTempScans failed: %v", err)
	}
	if _, err := os.Stat(p2); !os.IsNotExist(err) {
		t.Errorf("file p2 should have been deleted")
	}
}

func TestMapWIAError(t *testing.T) {
	// Test WIA service / COM class not registered (0x800401F3)
	errClassNotReg := MapWIAError("0x800401F3:Invalid class string", 1)
	if errClassNotReg == nil {
		t.Fatal("expected non-nil error for 0x800401F3")
	}
	if !strings.Contains(errClassNotReg.Error(), "WIA") {
		t.Errorf("expected error message to mention WIA, got: %s", errClassNotReg.Error())
	}

	// Test user cancel
	errCancel := MapWIAError("80210064", 1)
	if !errors.Is(errCancel, ErrScanCanceled) {
		t.Errorf("expected ErrScanCanceled, got: %v", errCancel)
	}

	// Test offline
	errOffline := MapWIAError("80210015", 1)
	if !strings.Contains(errOffline.Error(), "غير متصل") {
		t.Errorf("expected offline error message, got: %s", errOffline.Error())
	}

	// Test default formatting stripping PowerShell At line / + lines
	rawErr := "At line:25 char:32\n+ Some powershell command\nUnrecognized error message"
	errDefault := MapWIAError(rawErr, 1)
	if strings.Contains(errDefault.Error(), "At line:") || strings.Contains(errDefault.Error(), "+") {
		t.Errorf("expected cleaned up error trace, got: %s", errDefault.Error())
	}
	if !strings.Contains(errDefault.Error(), "Unrecognized error message") {
		t.Errorf("expected summary to contain unrecognized error message, got: %s", errDefault.Error())
	}
}
