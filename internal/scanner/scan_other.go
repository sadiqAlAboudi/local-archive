//go:build !windows

package scanner

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"
)

// ScanPage provides a simulated scanned page on non-Windows platforms (Linux/macOS)
// for automated testing, development, and CI environments.
func ScanPage() (*ScannedPage, error) {
	width := 800
	height := 1131 // Standard A4 ratio (~1:1.414)

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill background with warm paper-white color
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{250, 250, 250, 255}}, image.Point{}, draw.Src)

	// Draw decorative document margins/header lines to simulate a physical scanned paper
	lineColor := color.RGBA{210, 215, 220, 255}
	for y := 60; y < 70; y++ {
		for x := 60; x < width-60; x++ {
			img.Set(x, y, lineColor)
		}
	}
	for y := 100; y < 105; y++ {
		for x := 60; x < width-200; x++ {
			img.Set(x, y, lineColor)
		}
	}
	for y := 130; y < 135; y++ {
		for x := 60; x < width-300; x++ {
			img.Set(x, y, lineColor)
		}
	}

	tempPath := GenerateTempScanPath(".jpg")
	f, err := os.Create(tempPath)
	if err != nil {
		return nil, err
	}

	var b64Buf bytes.Buffer
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 85}); err != nil {
		_ = f.Close()
		_ = os.Remove(tempPath)
		return nil, err
	}
	_ = f.Close()

	_ = jpeg.Encode(&b64Buf, img, &jpeg.Options{Quality: 80})
	b64Data := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(b64Buf.Bytes())

	return &ScannedPage{
		ID:       filepath.Base(tempPath),
		TempPath: tempPath,
		Base64:   b64Data,
		Width:    width,
		Height:   height,
		Rotation: 0,
	}, nil
}
