package gofpdf

import (
	"math"
	//	"strings"
	"unicode"
)

// splitTextRuneWidth returns the width of c in thousandths of an em for the
// current font. Characters outside the font's width table fall back to the
// font's missing width, as GetStringSymbolWidth does, instead of indexing past
// the end of the table. A core font such as Arial only covers the first 256
// code points, so text like "1969-2019" with a typographic dash used to panic.
func (f *Fpdf) splitTextRuneWidth(c rune) int {
	cw := f.currentFont.Cw
	if int(c) < len(cw) && cw[c] > 0 {
		if cw[c] != 65535 {
			return cw[c]
		}
		return 0
	}
	if f.currentFont.Desc.MissingWidth != 0 {
		return f.currentFont.Desc.MissingWidth
	}
	return 500
}

// SplitText splits UTF-8 encoded text into several lines using the current
// font. Each line has its length limited to a maximum width given by w. This
// function can be used to determine the total height of wrapped text for
// vertical placement purposes.
func (f *Fpdf) SplitText(txt string, w float64) (lines []string) {
	wmax := int(math.Ceil((w - 2*f.cMargin) * 1000 / f.fontSize))
	s := []rune(txt) // Return slice of UTF-8 runes
	nb := len(s)
	for nb > 0 && s[nb-1] == '\n' {
		nb--
	}
	s = s[0:nb]
	sep := -1
	i := 0
	j := 0
	l := 0
	for i < nb {
		c := s[i]
		l += f.splitTextRuneWidth(c)
		if unicode.IsSpace(c) || isChinese(c) {
			sep = i
		}
		if c == '\n' || l > wmax {
			if sep == -1 {
				if i == j {
					i++
				}
				sep = i
			} else {
				i = sep + 1
			}
			lines = append(lines, string(s[j:sep]))
			sep = -1
			j = i
			l = 0
		} else {
			i++
		}
	}
	if i != j {
		lines = append(lines, string(s[j:i]))
	}
	return lines
}
