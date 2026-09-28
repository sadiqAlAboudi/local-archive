package optimizer

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
)

// helper to create a solid color JPEG image
func createTestJPEG(t *testing.T, path string, w, h int, q int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 120, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create image file: %v", err)
	}
	defer f.Close()

	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: q}); err != nil {
		t.Fatalf("failed to encode jpeg: %v", err)
	}
}

// helper to create a PNG image (opaque or transparent)
func createTestPNG(t *testing.T, path string, w, h int, transparent bool) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			alpha := uint8(255)
			if transparent && (x < w/4 && y < h/4) {
				alpha = 0 // transparent quarter
			}
			// Smooth photographic gradient with content blocks
			r := uint8((x * 255) / w)
			g := uint8((y * 255) / h)
			b := uint8(((x + y) * 128) / (w + h))
			if (y/40)%2 == 0 && x > w/6 && x < 5*w/6 {
				r, g, b = 20, 20, 20
			}
			img.SetNRGBA(x, y, color.NRGBA{R: r, G: g, B: b, A: alpha})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create image file: %v", err)
	}
	defer f.Close()

	encoder := png.Encoder{CompressionLevel: png.NoCompression}
	if err := encoder.Encode(f, img); err != nil {
		t.Fatalf("failed to encode png: %v", err)
	}
}

// helper to create a scanned-like PDF containing raster images
func createScannedPDF(t *testing.T, path string, numPages int, w, h int) {
	t.Helper()
	tmpDir := t.TempDir()
	var imgPaths []string

	for i := 0; i < numPages; i++ {
		imgFile := filepath.Join(tmpDir, filepath.Base(path)+string(rune('1'+i))+".jpg")
		createTestJPEG(t, imgFile, w, h, 95)
		imgPaths = append(imgPaths, imgFile)
	}

	imp := pdfcpu.DefaultImportConfig()
	if err := api.ImportImagesFile(imgPaths, path, imp, nil); err != nil {
		t.Fatalf("failed to create scanned pdf: %v", err)
	}
}

func TestOptimizeFile_ScannedPDF(t *testing.T) {
	tmpDir := t.TempDir()
	srcPdf := filepath.Join(tmpDir, "scanned_original.pdf")
	dstPdf := filepath.Join(tmpDir, "scanned_optimized.pdf")

	// Create 2 pages with 2400x3200 images
	createScannedPDF(t, srcPdf, 2, 2400, 3200)

	srcStat, err := os.Stat(srcPdf)
	if err != nil {
		t.Fatalf("failed to stat src: %v", err)
	}

	if err := OptimizeFile(srcPdf, dstPdf); err != nil {
		t.Fatalf("OptimizeFile failed on scanned PDF: %v", err)
	}

	dstStat, err := os.Stat(dstPdf)
	if err != nil {
		t.Fatalf("failed to stat dst: %v", err)
	}

	t.Logf("Scanned PDF: original=%d bytes, optimized=%d bytes (saved %.1f%%)",
		srcStat.Size(), dstStat.Size(),
		100.0*(1.0-float64(dstStat.Size())/float64(srcStat.Size())))

	if dstStat.Size() >= srcStat.Size() {
		t.Errorf("expected optimized size (%d) < original size (%d)", dstStat.Size(), srcStat.Size())
	}

	// Verify the optimized PDF is valid and has 2 pages
	pageCount, err := api.PageCountFile(dstPdf)
	if err != nil {
		t.Fatalf("failed to read page count of optimized PDF: %v", err)
	}
	if pageCount != 2 {
		t.Errorf("expected 2 pages, got %d", pageCount)
	}
}

func TestOptimizeFile_StandaloneLargeJPEG(t *testing.T) {
	tmpDir := t.TempDir()
	srcJpg := filepath.Join(tmpDir, "large_photo.jpg")
	dstJpg := filepath.Join(tmpDir, "large_photo_opt.jpg")

	// Create a large 2500x2500 high-quality JPEG
	createTestJPEG(t, srcJpg, 2500, 2500, 95)

	srcStat, err := os.Stat(srcJpg)
	if err != nil {
		t.Fatalf("failed to stat src: %v", err)
	}

	if err := OptimizeFile(srcJpg, dstJpg); err != nil {
		t.Fatalf("OptimizeFile failed on large JPEG: %v", err)
	}

	dstStat, err := os.Stat(dstJpg)
	if err != nil {
		t.Fatalf("failed to stat dst: %v", err)
	}

	t.Logf("Large JPEG: original=%d bytes, optimized=%d bytes (saved %.1f%%)",
		srcStat.Size(), dstStat.Size(),
		100.0*(1.0-float64(dstStat.Size())/float64(srcStat.Size())))

	if dstStat.Size() >= srcStat.Size() {
		t.Errorf("expected optimized size (%d) < original size (%d)", dstStat.Size(), srcStat.Size())
	}

	// Check dimensions of optimized output
	cfg, _, err := readImageConfig(dstJpg)
	if err != nil {
		t.Fatalf("failed to read optimized image config: %v", err)
	}
	if cfg.Width > MaxDimension || cfg.Height > MaxDimension {
		t.Errorf("expected dimensions <= %d, got %dx%d", MaxDimension, cfg.Width, cfg.Height)
	}
}

func TestOptimizeFile_StandaloneOpaquePNG(t *testing.T) {
	tmpDir := t.TempDir()
	srcPng := filepath.Join(tmpDir, "opaque_scan.png")
	dstJpg := filepath.Join(tmpDir, "opaque_scan_opt.jpg")

	// Create an opaque 2400x2400 PNG
	createTestPNG(t, srcPng, 2400, 2400, false)

	srcStat, err := os.Stat(srcPng)
	if err != nil {
		t.Fatalf("failed to stat src: %v", err)
	}

	if err := OptimizeFile(srcPng, dstJpg); err != nil {
		t.Fatalf("OptimizeFile failed on opaque PNG: %v", err)
	}

	dstStat, err := os.Stat(dstJpg)
	if err != nil {
		t.Fatalf("failed to stat dst: %v", err)
	}

	t.Logf("Opaque PNG: original=%d bytes, optimized=%d bytes (saved %.1f%%)",
		srcStat.Size(), dstStat.Size(),
		100.0*(1.0-float64(dstStat.Size())/float64(srcStat.Size())))

	if dstStat.Size() >= srcStat.Size() {
		t.Errorf("expected optimized size (%d) < original size (%d)", dstStat.Size(), srcStat.Size())
	}

	// Check that the format was converted to JPEG
	f, err := os.Open(dstJpg)
	if err != nil {
		t.Fatalf("failed to open dst: %v", err)
	}
	defer f.Close()

	header := make([]byte, 2)
	_, _ = f.Read(header)
	if !bytes.Equal(header, []byte{0xFF, 0xD8}) {
		t.Errorf("expected JPEG magic bytes FF D8, got %X", header)
	}
}

func TestOptimizeFile_StandaloneTransparentPNG(t *testing.T) {
	tmpDir := t.TempDir()
	srcPng := filepath.Join(tmpDir, "transparent.png")
	dstPng := filepath.Join(tmpDir, "transparent_opt.png")

	// Create a transparent 2400x2400 PNG
	createTestPNG(t, srcPng, 2400, 2400, true)

	if err := OptimizeFile(srcPng, dstPng); err != nil {
		t.Fatalf("OptimizeFile failed on transparent PNG: %v", err)
	}

	// Verify that the output is still PNG and preserves transparency
	f, err := os.Open(dstPng)
	if err != nil {
		t.Fatalf("failed to open dst: %v", err)
	}
	defer f.Close()

	header := make([]byte, 8)
	_, _ = f.Read(header)
	// PNG magic bytes: \x89PNG\r\n\x1a\n
	if !bytes.Equal(header[:4], []byte("\x89PNG")) {
		t.Errorf("expected PNG magic bytes, got %X", header[:4])
	}

	// Decode to check that transparency is preserved
	_, _ = f.Seek(0, 0)
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("failed to decode output PNG: %v", err)
	}
	if isImageOpaque(img) {
		t.Errorf("expected image to retain transparent alpha, but isImageOpaque returned true")
	}
}

func TestNaturalSorting(t *testing.T) {
	files := []string{
		"doc_10_Im0.jpg",
		"doc_1_Im0.jpg",
		"doc_2_Im0.jpg",
		"doc_20_Im0.jpg",
		"doc_03_Im0.jpg",
	}

	sortFilesNaturally(files)

	expected := []string{
		"doc_1_Im0.jpg",
		"doc_2_Im0.jpg",
		"doc_03_Im0.jpg",
		"doc_10_Im0.jpg",
		"doc_20_Im0.jpg",
	}

	for i, f := range files {
		if f != expected[i] {
			t.Errorf("at index %d: expected %s, got %s", i, expected[i], f)
		}
	}
}

func TestOptimizeFile_VectorPDFFallback(t *testing.T) {
	tmpDir := t.TempDir()
	srcPdf := filepath.Join(tmpDir, "vector.pdf")
	dstPdf := filepath.Join(tmpDir, "vector_opt.pdf")

	// Minimal valid single-page vector PDF (has no embedded images)
	content := `%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources <<>> >>
endobj
xref
0 4
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
trailer
<< /Size 4 /Root 1 0 R >>
startxref
197
%%EOF`

	if err := os.WriteFile(srcPdf, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write vector pdf: %v", err)
	}

	if err := OptimizeFile(srcPdf, dstPdf); err != nil {
		t.Fatalf("OptimizeFile failed on vector PDF: %v", err)
	}

	if _, err := os.Stat(dstPdf); err != nil {
		t.Fatalf("expected dstPdf to exist after fallback optimization: %v", err)
	}

	// Validate the output PDF is valid
	if err := api.ValidateFile(dstPdf, nil); err != nil {
		t.Fatalf("optimized vector PDF failed validation: %v", err)
	}
}

func TestOptimizeFile_SizeSafetyCheck(t *testing.T) {
	tmpDir := t.TempDir()
	srcJpg := filepath.Join(tmpDir, "small_already_compressed.jpg")
	dstJpg := filepath.Join(tmpDir, "small_already_compressed_opt.jpg")

	// Create a small 100x100 JPEG with low quality (e.g. 50%)
	createTestJPEG(t, srcJpg, 100, 100, 50)

	srcStat, err := os.Stat(srcJpg)
	if err != nil {
		t.Fatalf("failed to stat src: %v", err)
	}

	if err := OptimizeFile(srcJpg, dstJpg); err != nil {
		t.Fatalf("OptimizeFile failed on small JPEG: %v", err)
	}

	dstStat, err := os.Stat(dstJpg)
	if err != nil {
		t.Fatalf("failed to stat dst: %v", err)
	}

	// Destination must NEVER be larger than original
	if dstStat.Size() > srcStat.Size() {
		t.Errorf("size safety violated: dst size (%d) > src size (%d)", dstStat.Size(), srcStat.Size())
	}
}
