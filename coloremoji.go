package gofpdf

import (
	"fmt"
	"strings"
)

// ColorEmojiRenderer handles rendering of color emoji glyphs
type ColorEmojiRenderer struct {
	utf8File   *utf8FontFile
	unitsPerEm int
}

// NewColorEmojiRenderer creates a new color emoji renderer
func NewColorEmojiRenderer(utf8File *utf8FontFile) *ColorEmojiRenderer {
	return &ColorEmojiRenderer{
		utf8File:   utf8File,
		unitsPerEm: utf8File.GetUnitsPerEm(),
	}
}

// RenderColorGlyph renders a color glyph to PDF path operators
// x, y are the position in PDF user units
// fontSize is the font size in points
// k is the scale factor (points per user unit)
func (r *ColorEmojiRenderer) RenderColorGlyph(glyphID uint16, x, y, fontSize, k float64) string {
	if r.utf8File == nil || !r.utf8File.HasColorGlyphs() {
		return ""
	}

	layers := r.utf8File.GetColorGlyphLayers(glyphID)
	if layers == nil {
		return ""
	}

	var result strings.Builder
	scale := fontSize / float64(r.unitsPerEm)

	// Render layers back-to-front (first layer is bottom)
	for _, layer := range layers {
		color := r.utf8File.GetPaletteColor(layer.PaletteIndex)
		outline := r.utf8File.ParseGlyphOutline(layer.GlyphID)

		if outline == nil || len(outline.Contours) == 0 {
			continue
		}

		// Save graphics state
		result.WriteString("q ")

		// Set fill color (RGB normalized to 0-1)
		if color.A < 255 {
			// Handle transparency
			alpha := float64(color.A) / 255.0
			result.WriteString(fmt.Sprintf("%.3f %.3f %.3f rg ",
				float64(color.R)/255.0,
				float64(color.G)/255.0,
				float64(color.B)/255.0))
			// Note: Full alpha support requires ExtGState which is more complex
			// For now we just use the RGB values
			_ = alpha
		} else {
			result.WriteString(fmt.Sprintf("%.3f %.3f %.3f rg ",
				float64(color.R)/255.0,
				float64(color.G)/255.0,
				float64(color.B)/255.0))
		}

		// Convert outline to PDF path
		pathStr := glyphOutlineToPDFPath(outline, x, y, scale, k)
		result.WriteString(pathStr)

		// Fill the path
		result.WriteString("f ")

		// Restore graphics state
		result.WriteString("Q ")
	}

	return result.String()
}

// glyphOutlineToPDFPath converts a glyph outline to PDF path operators
// x, y: position in user units
// scale: fontSize / unitsPerEm
// k: points per user unit
func glyphOutlineToPDFPath(outline *GlyphOutline, x, y, scale, k float64) string {
	if outline == nil || len(outline.Contours) == 0 {
		return ""
	}

	var result strings.Builder

	for _, contour := range outline.Contours {
		if len(contour) < 2 {
			continue
		}

		pathOps := contourToPDFOps(contour, x, y, scale, k)
		result.WriteString(pathOps)
	}

	return result.String()
}

// contourToPDFOps converts a single contour to PDF path operators
func contourToPDFOps(contour GlyphContour, baseX, baseY, scale, k float64) string {
	if len(contour) < 2 {
		return ""
	}

	var result strings.Builder

	// Helper to transform coordinates
	transform := func(pt GlyphPoint) (float64, float64) {
		// Apply scale and flip Y for PDF coordinate system
		px := baseX + pt.X*scale
		py := baseY + pt.Y*scale // Y is bottom-up in PDF
		// Convert to points
		return px * k, py * k
	}

	// Find the first on-curve point or create one
	startIdx := -1
	for i, pt := range contour {
		if pt.OnCurve {
			startIdx = i
			break
		}
	}

	// If no on-curve point, start with midpoint between first two off-curve points
	if startIdx == -1 {
		// All points are off-curve, create implicit on-curve point
		p0 := contour[0]
		p1 := contour[1]
		midX := (p0.X + p1.X) / 2
		midY := (p0.Y + p1.Y) / 2
		px, py := transform(GlyphPoint{X: midX, Y: midY, OnCurve: true})
		result.WriteString(fmt.Sprintf("%.2f %.2f m ", px, py))
		startIdx = 0
	} else {
		// Move to first on-curve point
		px, py := transform(contour[startIdx])
		result.WriteString(fmt.Sprintf("%.2f %.2f m ", px, py))
	}

	// Process remaining points
	n := len(contour)
	i := (startIdx + 1) % n
	for count := 0; count < n; count++ {
		curr := contour[i]
		next := contour[(i+1)%n]

		if curr.OnCurve {
			// Line to on-curve point
			px, py := transform(curr)
			result.WriteString(fmt.Sprintf("%.2f %.2f l ", px, py))
		} else {
			// Quadratic bezier - we have an off-curve control point
			// TrueType uses quadratic beziers, PDF uses cubic
			// Convert quadratic to cubic:
			// C1 = P0 + 2/3 * (P1 - P0)
			// C2 = P2 + 2/3 * (P1 - P2)

			// Get the previous point (start of curve)
			prevIdx := (i - 1 + n) % n
			prev := contour[prevIdx]

			// If previous point is also off-curve, use implicit midpoint
			var p0X, p0Y float64
			if !prev.OnCurve {
				p0X = (prev.X + curr.X) / 2
				p0Y = (prev.Y + curr.Y) / 2
			} else {
				p0X = prev.X
				p0Y = prev.Y
			}

			// Get the end point
			var p2X, p2Y float64
			if next.OnCurve {
				p2X = next.X
				p2Y = next.Y
			} else {
				// Implicit midpoint between two off-curve points
				p2X = (curr.X + next.X) / 2
				p2Y = (curr.Y + next.Y) / 2
			}

			// Control point (off-curve)
			p1X := curr.X
			p1Y := curr.Y

			// Convert quadratic to cubic bezier control points
			c1X := p0X + 2.0/3.0*(p1X-p0X)
			c1Y := p0Y + 2.0/3.0*(p1Y-p0Y)
			c2X := p2X + 2.0/3.0*(p1X-p2X)
			c2Y := p2Y + 2.0/3.0*(p1Y-p2Y)

			// Transform all points
			c1px, c1py := transform(GlyphPoint{X: c1X, Y: c1Y})
			c2px, c2py := transform(GlyphPoint{X: c2X, Y: c2Y})
			epx, epy := transform(GlyphPoint{X: p2X, Y: p2Y})

			result.WriteString(fmt.Sprintf("%.2f %.2f %.2f %.2f %.2f %.2f c ",
				c1px, c1py, c2px, c2py, epx, epy))

			// If next point is on-curve and not the implicit midpoint, skip it
			// as we've already used it as the endpoint
			if next.OnCurve {
				i = (i + 1) % n
				count++
			}
		}

		i = (i + 1) % n
	}

	// Close the path
	result.WriteString("h ")

	return result.String()
}

// IsColorGlyph checks if a glyph ID has color layers
func (r *ColorEmojiRenderer) IsColorGlyph(glyphID uint16) bool {
	if r.utf8File == nil || !r.utf8File.HasColorGlyphs() {
		return false
	}
	return r.utf8File.GetColorGlyphLayers(glyphID) != nil
}

// GetGlyphLayers returns the color layers for a glyph
func (r *ColorEmojiRenderer) GetGlyphLayers(glyphID uint16) []LayerRecord {
	if r.utf8File == nil {
		return nil
	}
	return r.utf8File.GetColorGlyphLayers(glyphID)
}
