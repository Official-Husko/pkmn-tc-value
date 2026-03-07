package images

import (
	"fmt"
	"os"

	termimg "github.com/blacktop/go-termimg"
)

type Renderer interface {
	Supported() bool
	Render(path string, maxWidth int, maxHeight int) (string, error)
}

type TermimgRenderer struct {
	protocol termimg.Protocol
}

func NewRenderer() Renderer {
	return TermimgRenderer{
		protocol: termimg.DetectProtocol(),
	}
}

func (r TermimgRenderer) Supported() bool {
	return true
}

func (r TermimgRenderer) Render(path string, maxWidth int, maxHeight int) (string, error) {
	if path == "" {
		return "[image unavailable]", nil
	}
	if _, err := os.Stat(path); err != nil {
		return "[image unavailable]", nil
	}

	img, err := termimg.Open(path)
	if err != nil {
		return "[image unavailable]", fmt.Errorf("open image for terminal rendering: %w", err)
	}

	if maxWidth <= 0 {
		maxWidth = 32
	}
	if maxHeight <= 0 {
		maxHeight = 20
	}

	rendered, err := img.
		Protocol(r.protocol).
		Width(maxWidth).
		Height(maxHeight).
		Scale(termimg.ScaleFit).
		Render()
	if err != nil {
		return "[image unavailable]", fmt.Errorf("render image for terminal: %w", err)
	}
	if rendered == "" {
		return "[image unavailable]", nil
	}
	return rendered, nil
}
