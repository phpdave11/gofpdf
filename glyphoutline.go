package gofpdf

import (
	"encoding/binary"
	"math"
)

// GlyphPoint represents a point in a glyph outline
type GlyphPoint struct {
	X, Y    float64
	OnCurve bool
}

// GlyphContour is a closed contour made up of points
type GlyphContour []GlyphPoint

// GlyphOutline contains all contours of a glyph
type GlyphOutline struct {
	Contours []GlyphContour
	Bounds   [4]int16 // xMin, yMin, xMax, yMax
}

// TrueType simple glyph flags
const (
	glyphOnCurve         = 1 << 0
	glyphXShortVector    = 1 << 1
	glyphYShortVector    = 1 << 2
	glyphRepeat          = 1 << 3
	glyphXSameOrPosShort = 1 << 4
	glyphYSameOrPosShort = 1 << 5
)

// Composite glyph flags (reusing from utf8fontfile.go constants)
const (
	compositeArg1And2AreWords = 1 << 0
	compositeArgsAreXYValues  = 1 << 1
	compositeRoundXYToGrid    = 1 << 2
	compositeWeHaveAScale     = 1 << 3
	compositeMoreComponents   = 1 << 5
	compositeWeHaveAnXYScale  = 1 << 6
	compositeWeHaveATwoByTwo  = 1 << 7
	compositeWeHaveInstr      = 1 << 8
	compositeUseMyMetrics     = 1 << 9
	compositeOverlapCompound  = 1 << 10
)

// ParseGlyphOutline extracts the outline of a glyph from the glyf table
func (utf *utf8FontFile) ParseGlyphOutline(glyphID uint16) *GlyphOutline {
	if len(utf.symbolPosition) == 0 {
		return nil
	}

	if int(glyphID) >= len(utf.symbolPosition)-1 {
		return nil
	}

	glyfData := utf.getTableData("glyf")
	if glyfData == nil {
		return nil
	}

	symbolPos := utf.symbolPosition[glyphID]
	symbolLen := utf.symbolPosition[glyphID+1] - symbolPos

	if symbolLen == 0 {
		// Empty glyph (like space)
		return &GlyphOutline{}
	}

	data := glyfData[symbolPos : symbolPos+symbolLen]
	return utf.parseGlyphData(data, glyfData)
}

func (utf *utf8FontFile) parseGlyphData(data []byte, glyfData []byte) *GlyphOutline {
	if len(data) < 10 {
		return nil
	}

	numContours := int16(binary.BigEndian.Uint16(data[0:2]))
	xMin := int16(binary.BigEndian.Uint16(data[2:4]))
	yMin := int16(binary.BigEndian.Uint16(data[4:6]))
	xMax := int16(binary.BigEndian.Uint16(data[6:8]))
	yMax := int16(binary.BigEndian.Uint16(data[8:10]))

	outline := &GlyphOutline{
		Bounds: [4]int16{xMin, yMin, xMax, yMax},
	}

	if numContours >= 0 {
		// Simple glyph
		utf.parseSimpleGlyph(data[10:], int(numContours), outline)
	} else {
		// Composite glyph
		utf.parseCompositeGlyph(data[10:], glyfData, outline)
	}

	return outline
}

func (utf *utf8FontFile) parseSimpleGlyph(data []byte, numContours int, outline *GlyphOutline) {
	if numContours == 0 || len(data) < numContours*2 {
		return
	}

	// Read end points of contours
	endPtsOfContours := make([]uint16, numContours)
	for i := 0; i < numContours; i++ {
		endPtsOfContours[i] = binary.BigEndian.Uint16(data[i*2 : i*2+2])
	}

	numPoints := int(endPtsOfContours[numContours-1]) + 1
	offset := numContours * 2

	// Skip instruction length and instructions
	if offset+2 > len(data) {
		return
	}
	instructionLength := int(binary.BigEndian.Uint16(data[offset : offset+2]))
	offset += 2 + instructionLength

	if offset > len(data) {
		return
	}

	// Read flags
	flags := make([]byte, numPoints)
	for i := 0; i < numPoints; {
		if offset >= len(data) {
			return
		}
		flag := data[offset]
		offset++
		flags[i] = flag
		i++

		if (flag & glyphRepeat) != 0 {
			if offset >= len(data) {
				return
			}
			repeatCount := int(data[offset])
			offset++
			for j := 0; j < repeatCount && i < numPoints; j++ {
				flags[i] = flag
				i++
			}
		}
	}

	// Read X coordinates
	xCoords := make([]int, numPoints)
	var x int
	for i := 0; i < numPoints; i++ {
		flag := flags[i]
		if (flag & glyphXShortVector) != 0 {
			if offset >= len(data) {
				return
			}
			dx := int(data[offset])
			offset++
			if (flag & glyphXSameOrPosShort) != 0 {
				x += dx
			} else {
				x -= dx
			}
		} else if (flag & glyphXSameOrPosShort) == 0 {
			if offset+2 > len(data) {
				return
			}
			dx := int(int16(binary.BigEndian.Uint16(data[offset : offset+2])))
			offset += 2
			x += dx
		}
		// else: x is same as previous
		xCoords[i] = x
	}

	// Read Y coordinates
	yCoords := make([]int, numPoints)
	var y int
	for i := 0; i < numPoints; i++ {
		flag := flags[i]
		if (flag & glyphYShortVector) != 0 {
			if offset >= len(data) {
				return
			}
			dy := int(data[offset])
			offset++
			if (flag & glyphYSameOrPosShort) != 0 {
				y += dy
			} else {
				y -= dy
			}
		} else if (flag & glyphYSameOrPosShort) == 0 {
			if offset+2 > len(data) {
				return
			}
			dy := int(int16(binary.BigEndian.Uint16(data[offset : offset+2])))
			offset += 2
			y += dy
		}
		// else: y is same as previous
		yCoords[i] = y
	}

	// Build contours
	outline.Contours = make([]GlyphContour, numContours)
	pointIdx := 0
	for c := 0; c < numContours; c++ {
		endPt := int(endPtsOfContours[c])
		contourLen := endPt - pointIdx + 1
		contour := make(GlyphContour, contourLen)

		for i := 0; i < contourLen; i++ {
			contour[i] = GlyphPoint{
				X:       float64(xCoords[pointIdx]),
				Y:       float64(yCoords[pointIdx]),
				OnCurve: (flags[pointIdx] & glyphOnCurve) != 0,
			}
			pointIdx++
		}
		outline.Contours[c] = contour
	}
}

func (utf *utf8FontFile) parseCompositeGlyph(data []byte, glyfData []byte, outline *GlyphOutline) {
	offset := 0
	flags := uint16(compositeMoreComponents)

	for (flags & compositeMoreComponents) != 0 {
		if offset+4 > len(data) {
			return
		}

		flags = binary.BigEndian.Uint16(data[offset : offset+2])
		glyphIndex := binary.BigEndian.Uint16(data[offset+2 : offset+4])
		offset += 4

		// Read transform arguments
		var arg1, arg2 int
		if (flags & compositeArg1And2AreWords) != 0 {
			if offset+4 > len(data) {
				return
			}
			arg1 = int(int16(binary.BigEndian.Uint16(data[offset : offset+2])))
			arg2 = int(int16(binary.BigEndian.Uint16(data[offset+2 : offset+4])))
			offset += 4
		} else {
			if offset+2 > len(data) {
				return
			}
			arg1 = int(int8(data[offset]))
			arg2 = int(int8(data[offset+1]))
			offset += 2
		}

		// Initialize transform matrix
		var a, b, c, d float64 = 1, 0, 0, 1
		var e, f float64 = 0, 0

		if (flags & compositeArgsAreXYValues) != 0 {
			e = float64(arg1)
			f = float64(arg2)
		}

		// Read scale/transform
		if (flags & compositeWeHaveAScale) != 0 {
			if offset+2 > len(data) {
				return
			}
			scale := read2Dot14(data[offset : offset+2])
			offset += 2
			a = scale
			d = scale
		} else if (flags & compositeWeHaveAnXYScale) != 0 {
			if offset+4 > len(data) {
				return
			}
			a = read2Dot14(data[offset : offset+2])
			d = read2Dot14(data[offset+2 : offset+4])
			offset += 4
		} else if (flags & compositeWeHaveATwoByTwo) != 0 {
			if offset+8 > len(data) {
				return
			}
			a = read2Dot14(data[offset : offset+2])
			b = read2Dot14(data[offset+2 : offset+4])
			c = read2Dot14(data[offset+4 : offset+6])
			d = read2Dot14(data[offset+6 : offset+8])
			offset += 8
		}

		// Get component glyph outline
		if int(glyphIndex) < len(utf.symbolPosition)-1 {
			compPos := utf.symbolPosition[glyphIndex]
			compLen := utf.symbolPosition[glyphIndex+1] - compPos

			if compLen > 0 && compPos+compLen <= len(glyfData) {
				compData := glyfData[compPos : compPos+compLen]
				compOutline := utf.parseGlyphData(compData, glyfData)

				if compOutline != nil {
					// Transform and add component contours
					for _, contour := range compOutline.Contours {
						transformedContour := make(GlyphContour, len(contour))
						for i, pt := range contour {
							// Apply 2D affine transform
							newX := a*pt.X + c*pt.Y + e
							newY := b*pt.X + d*pt.Y + f
							transformedContour[i] = GlyphPoint{
								X:       newX,
								Y:       newY,
								OnCurve: pt.OnCurve,
							}
						}
						outline.Contours = append(outline.Contours, transformedContour)
					}
				}
			}
		}
	}
}

// read2Dot14 reads a 2.14 fixed-point number
func read2Dot14(data []byte) float64 {
	val := int16(binary.BigEndian.Uint16(data))
	return float64(val) / 16384.0
}

// TransformOutline applies a 2D affine transform to a glyph outline
func TransformOutline(outline *GlyphOutline, a, b, c, d, e, f float64) *GlyphOutline {
	if outline == nil {
		return nil
	}

	result := &GlyphOutline{
		Bounds:   outline.Bounds, // Bounds would need recalculating for accuracy
		Contours: make([]GlyphContour, len(outline.Contours)),
	}

	for i, contour := range outline.Contours {
		result.Contours[i] = make(GlyphContour, len(contour))
		for j, pt := range contour {
			result.Contours[i][j] = GlyphPoint{
				X:       a*pt.X + c*pt.Y + e,
				Y:       b*pt.X + d*pt.Y + f,
				OnCurve: pt.OnCurve,
			}
		}
	}

	return result
}

// ScaleOutline scales a glyph outline by a factor
func ScaleOutline(outline *GlyphOutline, scale float64) *GlyphOutline {
	return TransformOutline(outline, scale, 0, 0, scale, 0, 0)
}

// TranslateOutline translates a glyph outline by (dx, dy)
func TranslateOutline(outline *GlyphOutline, dx, dy float64) *GlyphOutline {
	return TransformOutline(outline, 1, 0, 0, 1, dx, dy)
}

// FlipY flips the Y coordinates (for PDF coordinate system)
func FlipY(outline *GlyphOutline) *GlyphOutline {
	return TransformOutline(outline, 1, 0, 0, -1, 0, 0)
}

// RotateOutline rotates a glyph outline by angle (in radians)
func RotateOutline(outline *GlyphOutline, angle float64) *GlyphOutline {
	cos := math.Cos(angle)
	sin := math.Sin(angle)
	return TransformOutline(outline, cos, sin, -sin, cos, 0, 0)
}
