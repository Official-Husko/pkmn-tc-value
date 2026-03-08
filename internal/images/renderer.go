package images

import (
	"fmt"
	"os"

	termimg "github.com/blacktop/go-termimg"
)

type Renderer interface {
	Supported() bool
	Protocol() termimg.Protocol
	ClearAllString() string
	Render(path string, maxWidth int, maxHeight int) (string, error)
}

type TermimgRenderer struct {
	protocol termimg.Protocol
}

func NewRenderer() Renderer {
	protocol := termimg.Unsupported
	for _, candidate := range termimg.DetermineProtocols() {
		if candidate == termimg.Halfblocks || candidate == termimg.Unsupported {
			continue
		}
		protocol = candidate
		break
	}
	return TermimgRenderer{
		protocol: protocol,
	}
}

func (r TermimgRenderer) Supported() bool {
	return r.protocol != termimg.Unsupported
}

func (r TermimgRenderer) Protocol() termimg.Protocol {
	return r.protocol
}

func (r TermimgRenderer) ClearAllString() string {
	return termimg.ClearAllString()
}

func (r TermimgRenderer) Render(path string, maxWidth int, maxHeight int) (string, error) {
	if path == "" {
		return "[image unavailable]", nil
	}
	if _, err := os.Stat(path); err != nil {
		return "[image unavailable]", nil
	}
	if !r.Supported() {
		return "[image unavailable]", fmt.Errorf("no supported graphics protocol (halfblocks disabled)")
	}

	img, err := termimg.Open(path)
	if err != nil {
		return "[image unavailable]", fmt.Errorf("open image for terminal rendering: %w", err)
	}

	if maxWidth <= 0 {
		maxWidth = 42
	}
	if maxHeight <= 0 {
		maxHeight = 28
	}

	renderBuilder := img.
		Protocol(r.protocol).
		Width(maxWidth).
		Height(maxHeight).
		Scale(termimg.ScaleFit).
		Compression(false).
		PNG(true)

	if r.protocol == termimg.Sixel {
		renderBuilder = renderBuilder.
			Dither(true).
			DitherMode(termimg.DitherFloydSteinberg)
	} else {
		renderBuilder = renderBuilder.Dither(false)
	}

	rendered, err := renderBuilder.Render()
	if err != nil {
		return "[image unavailable]", fmt.Errorf("render image for terminal: %w", err)
	}
	if rendered == "" {
		return "[image unavailable]", nil
	}
	return rendered, nil
}
