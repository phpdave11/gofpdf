package gofpdf

import (
	"os"
	"testing"
)

func TestCOLRCPALParsing(t *testing.T) {
	// Test with a font that doesn't have COLR/CPAL tables
	fontPath := "font/calligra.ttf"
	fontData, err := os.ReadFile(fontPath)
	if err != nil {
		t.Skipf("Font file not found: %s", fontPath)
		return
	}

	reader := fileReader{readerPosition: 0, array: fontData}
	utf8File := newUTF8Font(&reader)
	err = utf8File.parseFile()
	if err != nil {
		t.Fatalf("Failed to parse font: %v", err)
	}

	// Calligra should not have color glyphs
	if utf8File.HasColorGlyphs() {
		t.Error("Expected calligra font to not have color glyphs")
	}
}

func TestGlyphOutlineParsing(t *testing.T) {
	// Test glyph outline parsing with a regular font
	fontPath := "font/calligra.ttf"
	fontData, err := os.ReadFile(fontPath)
	if err != nil {
		t.Skipf("Font file not found: %s", fontPath)
		return
	}

	reader := fileReader{readerPosition: 0, array: fontData}
	utf8File := newUTF8Font(&reader)
	err = utf8File.parseFile()
	if err != nil {
		t.Fatalf("Failed to parse font: %v", err)
	}

	// We need to initialize symbolPosition by calling GenerateCutFont or similar
	// For now, just check that the method doesn't panic
	utf8File.fileReader.readerPosition = 0
	utf8File.symbolPosition = make([]int, 0)
	utf8File.charSymbolDictionary = make(map[int]int)
	utf8File.tableDescriptions = make(map[string]*tableDescription)
	utf8File.outTablesData = make(map[string][]byte)
	utf8File.skip(4)
	utf8File.generateTableDescriptions()

	// Parse loca table to get symbol positions
	utf8File.SeekTable("head")
	utf8File.skip(50)
	locaFormat := utf8File.readUint16()

	utf8File.SeekTable("maxp")
	utf8File.skip(4)
	numSymbols := utf8File.readUint16()

	utf8File.parseLOCATable(locaFormat, numSymbols)

	// Try to parse a glyph outline
	outline := utf8File.ParseGlyphOutline(0)
	if outline == nil {
		t.Log("Glyph 0 has no outline (may be .notdef)")
	}

	// Test with a higher glyph ID that likely has an outline
	outline = utf8File.ParseGlyphOutline(10)
	if outline != nil && len(outline.Contours) > 0 {
		t.Logf("Glyph 10 has %d contours", len(outline.Contours))
	}
}

func TestColorEmojiAPI(t *testing.T) {
	pdf := New("P", "mm", "A4", "font")
	pdf.AddPage()
	pdf.AddUTF8Font("DejaVu", "", "DejaVuSansCondensed.ttf")
	pdf.SetFont("DejaVu", "", 14)

	// Test the API methods exist and work
	pdf.SetColorEmojiEnabled(true)
	if pdf.colorEmojiEnabled != true {
		t.Error("Expected colorEmojiEnabled to be true")
	}

	// DejaVu doesn't have color glyphs, so HasColorEmoji should return false
	if pdf.HasColorEmoji() {
		t.Error("Expected HasColorEmoji to return false for DejaVu font")
	}

	pdf.SetColorEmojiEnabled(false)
	if pdf.colorEmojiEnabled != false {
		t.Error("Expected colorEmojiEnabled to be false")
	}
}

func TestGlyphOutlineTransforms(t *testing.T) {
	// Create a simple test outline
	outline := &GlyphOutline{
		Bounds: [4]int16{0, 0, 100, 100},
		Contours: []GlyphContour{
			{
				{X: 0, Y: 0, OnCurve: true},
				{X: 100, Y: 0, OnCurve: true},
				{X: 100, Y: 100, OnCurve: true},
				{X: 0, Y: 100, OnCurve: true},
			},
		},
	}

	// Test scale
	scaled := ScaleOutline(outline, 2.0)
	if scaled.Contours[0][1].X != 200 {
		t.Errorf("Expected scaled X to be 200, got %f", scaled.Contours[0][1].X)
	}

	// Test translate
	translated := TranslateOutline(outline, 50, 50)
	if translated.Contours[0][0].X != 50 || translated.Contours[0][0].Y != 50 {
		t.Errorf("Expected translated point to be (50, 50), got (%f, %f)",
			translated.Contours[0][0].X, translated.Contours[0][0].Y)
	}

	// Test Y flip
	flipped := FlipY(outline)
	if flipped.Contours[0][2].Y != -100 {
		t.Errorf("Expected flipped Y to be -100, got %f", flipped.Contours[0][2].Y)
	}
}

func TestColorRecordParsing(t *testing.T) {
	// Test that GetPaletteColor returns a default color when no CPAL table exists
	reader := fileReader{readerPosition: 0, array: []byte{}}
	utf8File := newUTF8Font(&reader)

	color := utf8File.GetPaletteColor(0)
	if color.R != 0 || color.G != 0 || color.B != 0 || color.A != 255 {
		t.Errorf("Expected default color (0,0,0,255), got (%d,%d,%d,%d)",
			color.R, color.G, color.B, color.A)
	}
}
