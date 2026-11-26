package gofpdf

import (
	"strings"
	"testing"
)

// TestSplitTextCoreFontWideRune covers text that contains code points outside
// the width table of a core font, which used to panic with an index out of
// range in SplitText.
func TestSplitTextCoreFontWideRune(t *testing.T) {
	pdf := New("P", "mm", "A4", "")
	pdf.SetFont("Arial", "", 12)
	pdf.AddPage()

	const txt = "Der Vertrag von 1969\u20132019 ist g\u00fcltig."
	lines := pdf.SplitText(txt, 200)
	if len(lines) != 1 || lines[0] != txt {
		t.Fatalf("SplitText(%q) = %q, want a single line holding the input", txt, lines)
	}

	// A narrow field must wrap without panicking and without dropping text.
	wrapped := pdf.SplitText(txt, 20)
	if len(wrapped) < 2 {
		t.Fatalf("SplitText(%q, 20) = %q, want several lines", txt, wrapped)
	}
	for _, line := range wrapped {
		if line == "" || strings.TrimSpace(line) == "" {
			t.Fatalf("SplitText(%q, 20) returned an empty line: %q", txt, wrapped)
		}
	}
}

// TestSplitTextUTF8Font checks the same characters with a font that does have
// widths for them.
func TestSplitTextUTF8Font(t *testing.T) {
	pdf := New("P", "mm", "A4", "")
	pdf.AddUTF8Font("dejavu", "", "font/DejaVuSansCondensed.ttf")
	pdf.SetFont("dejavu", "", 12)
	pdf.AddPage()

	const txt = "f\u00fcr 1969\u20132019"
	lines := pdf.SplitText(txt, 200)
	if len(lines) != 1 || lines[0] != txt {
		t.Fatalf("SplitText(%q) = %q, want a single line holding the input", txt, lines)
	}
}
