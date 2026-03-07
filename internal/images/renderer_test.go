package images

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	termimg "github.com/blacktop/go-termimg"
)

func TestTermimgRendererRender(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sample.png")

	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 40), G: uint8(y * 40), B: 180, A: 255})
		}
	}

	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create temp image: %v", err)
	}
	defer file.Close()
	if err := png.Encode(file, img); err != nil {
		t.Fatalf("encode temp image: %v", err)
	}

	renderer := TermimgRenderer{protocol: termimg.Halfblocks}
	out, err := renderer.Render(path, 8, 4)
	if err != nil {
		t.Fatalf("render image: %v", err)
	}
	if strings.TrimSpace(out) == "" {
		t.Fatal("expected rendered image output")
	}
}
