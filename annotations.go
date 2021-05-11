package gofpdf

import (
	"fmt"
)

type Highlight struct {
	Contents   string
	Rect       []float32
	QuadPoints []float32
	Color      []float32
	Opacity    float32
	Author     string
}

func (h *Highlight) String() string {
	if h.Rect == nil || len(h.Rect) != 4 {
		return ""
	}

	hlstr := "<</Type /Annot /Subtype /Highlight "

	if h.Color != nil && len(h.Color) == 3 {
		hlstr += fmt.Sprintf("/C %.4f ", h.Color)
	}

	hlstr += fmt.Sprintf("/CA %.4f", h.Opacity)
	hlstr += "/F 4" // No-Zoom flag

	if h.QuadPoints != nil && len(h.QuadPoints) >= 8 {
		hlstr += fmt.Sprintf(" /QuadPoints %.4f ", h.QuadPoints)
	}

	if h.Contents != "" {
		hlstr += fmt.Sprintf("/Contents (%s)", h.Contents)
	}

	if h.Author != "" {
		hlstr += fmt.Sprintf("/T (%s)", h.Author)
	}

	hlstr += fmt.Sprintf("/Rect %.4f", h.Rect)
	hlstr += "]>>"
	return hlstr
}

func (f *Fpdf) AddHighlightAnnotation(highlight Highlight) {
	if f.page >= len(f.highlights) {
		f.highlights = append(f.highlights, []Highlight{highlight})
	} else {
		f.highlights[f.page] = append(f.highlights[f.page], highlight)
	}
}
