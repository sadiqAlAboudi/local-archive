package optimizer

import (
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/disintegration/imaging"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const (
	// MaxDimension defines the maximum width or height bounding box for downscaling.
	MaxDimension = 2048

	// ScannedJPEGQuality defines the JPEG compression quality for pages reassembled into PDFs.
	ScannedJPEGQuality = 75

	// StandaloneJPEGQuality defines the JPEG compression quality for standalone image files.
	StandaloneJPEGQuality = 80
)

// OptimizeFile inspects the input file, automatically routes it through the appropriate
// optimization pipeline (scanned PDF, vector/text PDF, JPEG, or PNG), and writes the
// result to dstPath.
//
// Safety guarantees:
//   - If the optimized file is larger than the original, the original file is preserved.
//   - All temporary intermediate files and directories are safely cleaned up.
//   - If an optimization step fails, safe fallbacks are applied.
func OptimizeFile(srcPath, dstPath string) error {
	if srcPath == "" {
		return errors.New("source path is required")
	}
	if dstPath == "" {
		return errors.New("destination path is required")
	}

	srcStat, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("failed to stat source file: %w", err)
	}
	if srcStat.IsDir() {
		return fmt.Errorf("source path is a directory, expected a file: %s", srcPath)
	}
	if srcStat.Size() == 0 {
		return copyOrKeepOriginal(srcPath, dstPath)
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	fileType, err := detectFileType(srcPath)
	if err != nil {
		return copyOrKeepOriginal(srcPath, dstPath)
	}

	switch fileType {
	case "pdf":
		return optimizePDF(srcPath, dstPath, srcStat.Size())
	case "image":
		return optimizeImage(srcPath, dstPath, srcStat.Size())
	default:
		return copyOrKeepOriginal(srcPath, dstPath)
	}
}

// detectFileType determines if the file is a PDF or an Image by extension and header sniffing.
func detectFileType(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".pdf":
		return "pdf", nil
	case ".jpg", ".jpeg", ".png", ".webp", ".tif", ".tiff", ".bmp":
		return "image", nil
	}

	// Sniff magic bytes if extension is ambiguous or missing
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}

	if n >= 4 && string(buf[:4]) == "%PDF" {
		return "pdf", nil
	}

	contentType := http.DetectContentType(buf[:n])
	if strings.HasPrefix(contentType, "image/") {
		return "image", nil
	}

	return "unknown", nil
}

// optimizePDF handles PDF documents:
// 1. Attempts to extract raster page images (typical of phone/hardware scanners).
// 2. Sequentially downscales extracted images > 2048px and compresses to JPEG at 75%.
// 3. Reassembles pages into a new PDF using pdfcpu.
// 4. Falls back to structural optimization (pdfcpu.OptimizeFile) if image extraction
//    fails or yields no images (e.g. digital vector / text PDFs).
// 5. Preserves original file if the result is larger.
func optimizePDF(srcPath, dstPath string, origSize int64) error {
	tmpDir, err := os.MkdirTemp("", "pdf_opt_*")
	if err != nil {
		return fallbackStructuralPDF(srcPath, dstPath, origSize)
	}
	defer os.RemoveAll(tmpDir)

	extractDir := filepath.Join(tmpDir, "extracted")
	if err := os.MkdirAll(extractDir, 0755); err != nil {
		return fallbackStructuralPDF(srcPath, dstPath, origSize)
	}

	// Step 1: Extract embedded images
	extractErr := api.ExtractImagesFile(srcPath, extractDir, nil, nil)

	var imageFiles []string
	if extractErr == nil {
		entries, err := os.ReadDir(extractDir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				name := e.Name()
				if isThumbnail(name) {
					continue
				}
				if isSupportedImageExtension(name) {
					imageFiles = append(imageFiles, filepath.Join(extractDir, name))
				}
			}
		}
	}

	// Fallback check: if extraction failed or no raster images were found (vector/text PDF)
	if extractErr != nil || len(imageFiles) == 0 {
		return fallbackStructuralPDF(srcPath, dstPath, origSize)
	}

	// Sort images in natural page order (e.g. page 2 before page 10)
	sortFilesNaturally(imageFiles)

	// Step 2: Process extracted images sequentially in a temp directory
	processedDir := filepath.Join(tmpDir, "processed")
	if err := os.MkdirAll(processedDir, 0755); err != nil {
		return fallbackStructuralPDF(srcPath, dstPath, origSize)
	}

	var processedImages []string
	for i, imgPath := range imageFiles {
		outJpg := filepath.Join(processedDir, fmt.Sprintf("page_%05d.jpg", i+1))
		if err := processScannedImage(imgPath, outJpg, MaxDimension, ScannedJPEGQuality); err != nil {
			// If image processing fails, fall back to structural optimization
			return fallbackStructuralPDF(srcPath, dstPath, origSize)
		}
		processedImages = append(processedImages, outJpg)
	}

	// Step 3: Reassemble compressed images into a new PDF
	reassembledPdf := filepath.Join(tmpDir, "reassembled.pdf")
	imp := pdfcpu.DefaultImportConfig()
	if err := api.ImportImagesFile(processedImages, reassembledPdf, imp, nil); err != nil {
		return fallbackStructuralPDF(srcPath, dstPath, origSize)
	}

	// Step 4: Size safety check
	reStat, err := os.Stat(reassembledPdf)
	if err != nil || reStat.Size() >= origSize {
		return copyOrKeepOriginal(srcPath, dstPath)
	}

	return replaceFile(reassembledPdf, dstPath)
}

// processScannedImage opens a single extracted page image, downscales it if exceeding
// maxDim (preserving aspect ratio), and encodes to JPEG at the specified quality.
func processScannedImage(srcPath, dstPath string, maxDim, quality int) error {
	srcImg, err := imaging.Open(srcPath, imaging.AutoOrientation(true))
	if err != nil {
		return err
	}

	// Downscale if dimensions exceed maxDim (imaging.Fit preserves aspect ratio)
	img := imaging.Fit(srcImg, maxDim, maxDim, imaging.Lanczos)

	// Ensure background is solid white if the image contains any transparency
	var finalImg image.Image = img
	if !isImageOpaque(finalImg) {
		finalImg = compositeOnWhite(finalImg)
	}

	return imaging.Save(finalImg, dstPath, imaging.JPEGQuality(quality))
}

// fallbackStructuralPDF applies pdfcpu.OptimizeFile for text/vector PDFs or when
// raster extraction is not applicable.
func fallbackStructuralPDF(srcPath, dstPath string, origSize int64) error {
	tmpDir, err := os.MkdirTemp("", "pdf_struct_*")
	if err != nil {
		return copyOrKeepOriginal(srcPath, dstPath)
	}
	defer os.RemoveAll(tmpDir)

	tmpPdf := filepath.Join(tmpDir, "optimized.pdf")
	if err := api.OptimizeFile(srcPath, tmpPdf, nil); err != nil {
		// If structural optimization fails, safely preserve original
		return copyOrKeepOriginal(srcPath, dstPath)
	}

	st, err := os.Stat(tmpPdf)
	if err != nil || st.Size() >= origSize {
		return copyOrKeepOriginal(srcPath, dstPath)
	}

	return replaceFile(tmpPdf, dstPath)
}

// optimizeImage handles standalone images (JPEG, PNG, etc.):
// 1. Reads configuration headers first to avoid loading huge pixel buffers if unneeded.
// 2. Downscales if exceeding a 2048px bounding box.
// 3. For PNG: converts completely opaque images to JPEG at 80% quality; keeps transparent
//    images as PNG with maximum compression.
// 4. For JPEG: re-encodes at 80% quality.
// 5. Preserves original file if optimized output is larger.
func optimizeImage(srcPath, dstPath string, origSize int64) error {
	// Step 1: Read image configuration headers first to check dimensions
	cfg, format, err := readImageConfig(srcPath)
	if err != nil {
		return copyOrKeepOriginal(srcPath, dstPath)
	}

	tmpDir, err := os.MkdirTemp("", "img_opt_*")
	if err != nil {
		return copyOrKeepOriginal(srcPath, dstPath)
	}
	defer os.RemoveAll(tmpDir)

	tempOut := filepath.Join(tmpDir, "optimized_image")

	needsResize := cfg.Width > MaxDimension || cfg.Height > MaxDimension

	// Decode image with EXIF orientation correction
	img, err := imaging.Open(srcPath, imaging.AutoOrientation(true))
	if err != nil {
		return copyOrKeepOriginal(srcPath, dstPath)
	}

	// Step 2: Downscale if exceeding 2048px bounding box
	if needsResize {
		img = imaging.Fit(img, MaxDimension, MaxDimension, imaging.Lanczos)
	}

	outFile, err := os.Create(tempOut)
	if err != nil {
		return copyOrKeepOriginal(srcPath, dstPath)
	}

	var encodeErr error
	lowerFormat := strings.ToLower(format)

	switch lowerFormat {
	case "png":
		// Step 3: Check alpha transparency
		if isImageOpaque(img) {
			// Completely opaque: convert to JPEG at 80% quality for maximum size reduction
			encodeErr = imaging.Encode(outFile, img, imaging.JPEG, imaging.JPEGQuality(StandaloneJPEGQuality))
		} else {
			// Transparent: keep PNG with maximum compression
			encodeErr = imaging.Encode(outFile, img, imaging.PNG, imaging.PNGCompressionLevel(png.BestCompression))
		}
	case "jpeg", "jpg":
		// Step 4: Re-encode JPEG at 80% quality
		var finalImg image.Image = img
		if !isImageOpaque(finalImg) {
			finalImg = compositeOnWhite(finalImg)
		}
		encodeErr = imaging.Encode(outFile, finalImg, imaging.JPEG, imaging.JPEGQuality(StandaloneJPEGQuality))
	default:
		// Other formats (e.g. WebP, TIFF, BMP)
		if isImageOpaque(img) {
			encodeErr = imaging.Encode(outFile, img, imaging.JPEG, imaging.JPEGQuality(StandaloneJPEGQuality))
		} else {
			encodeErr = imaging.Encode(outFile, img, imaging.PNG, imaging.PNGCompressionLevel(png.BestCompression))
		}
	}

	closeErr := outFile.Close()
	if encodeErr != nil || closeErr != nil {
		return copyOrKeepOriginal(srcPath, dstPath)
	}

	// Step 5: Size safety check
	optStat, err := os.Stat(tempOut)
	if err != nil || optStat.Size() >= origSize {
		return copyOrKeepOriginal(srcPath, dstPath)
	}

	return replaceFile(tempOut, dstPath)
}

// readImageConfig reads only the image header metadata without decoding pixel data.
func readImageConfig(path string) (image.Config, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return image.Config{}, "", err
	}
	defer f.Close()

	return image.DecodeConfig(f)
}

type opaqueChecker interface {
	Opaque() bool
}

// isImageOpaque checks if an image is completely opaque (no transparent alpha pixels).
func isImageOpaque(img image.Image) bool {
	if oc, ok := img.(opaqueChecker); ok {
		return oc.Opaque()
	}

	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			if a < 0xffff {
				return false
			}
		}
	}
	return true
}

// compositeOnWhite composites an image with transparency over a solid white background.
func compositeOnWhite(img image.Image) image.Image {
	bounds := img.Bounds()
	bg := image.NewRGBA(bounds)
	draw.Draw(bg, bounds, image.White, image.Point{}, draw.Src)
	draw.Draw(bg, bounds, img, bounds.Min, draw.Over)
	return bg
}

// isThumbnail checks if an extracted filename represents an embedded thumbnail rather than a page.
func isThumbnail(filename string) bool {
	lower := strings.ToLower(filename)
	return strings.Contains(lower, "_thumb.") || strings.HasSuffix(lower, "_thumb")
}

// isSupportedImageExtension checks if the file has an image extension produced by pdfcpu extraction.
func isSupportedImageExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".tif", ".tiff", ".webp", ".bmp":
		return true
	default:
		return false
	}
}

// replaceFile atomically replaces dst with src, falling back to copy+remove if cross-device.
func replaceFile(src, dst string) error {
	if src == dst {
		return nil
	}
	_ = os.MkdirAll(filepath.Dir(dst), 0755)

	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	if err := copyFile(src, dst); err != nil {
		return err
	}
	_ = os.Remove(src)
	return nil
}

// copyOrKeepOriginal ensures dstPath has the original content if srcPath != dstPath.
func copyOrKeepOriginal(src, dst string) error {
	if src == dst {
		return nil
	}
	_ = os.MkdirAll(filepath.Dir(dst), 0755)
	return copyFile(src, dst)
}

// copyFile copies data from src to dst.
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

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// sortFilesNaturally sorts filenames naturally (e.g. "doc_2" before "doc_10").
func sortFilesNaturally(files []string) {
	sort.Slice(files, func(i, j int) bool {
		return naturalLess(files[i], files[j])
	})
}

func naturalLess(a, b string) bool {
	baseA := filepath.Base(a)
	baseB := filepath.Base(b)

	chunksA := splitIntoChunks(baseA)
	chunksB := splitIntoChunks(baseB)

	minLen := len(chunksA)
	if len(chunksB) < minLen {
		minLen = len(chunksB)
	}

	for i := 0; i < minLen; i++ {
		ca, cb := chunksA[i], chunksB[i]
		na, errA := strconv.ParseInt(ca, 10, 64)
		nb, errB := strconv.ParseInt(cb, 10, 64)

		if errA == nil && errB == nil {
			if na != nb {
				return na < nb
			}
			if len(ca) != len(cb) {
				return len(ca) < len(cb)
			}
		} else {
			if ca != cb {
				return ca < cb
			}
		}
	}
	return len(chunksA) < len(chunksB)
}

func splitIntoChunks(s string) []string {
	var chunks []string
	var cur strings.Builder
	var isDigit bool

	for i, r := range s {
		digit := unicode.IsDigit(r)
		if i == 0 {
			isDigit = digit
			cur.WriteRune(r)
		} else if digit == isDigit {
			cur.WriteRune(r)
		} else {
			chunks = append(chunks, cur.String())
			cur.Reset()
			cur.WriteRune(r)
			isDigit = digit
		}
	}
	if cur.Len() > 0 {
		chunks = append(chunks, cur.String())
	}
	return chunks
}
