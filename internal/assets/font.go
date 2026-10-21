package assets

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf16"
)

// The game font is a set of bitmap sheets. buildFont turns those pixels into
// square outlines inside a TrueType file so the browser can render the real
// font crisply at any multiple of 8px.

const (
	fontUnit     = 128 // font units per font pixel
	unitsPerEm   = 8 * fontUnit
	spaceAdvance = 4
)

type fontProvider struct {
	Type    string         `json:"type"`
	ID      string         `json:"id"`
	File    string         `json:"file"`
	Height  *int           `json:"height"`
	Ascent  int            `json:"ascent"`
	Chars   []string       `json:"chars"`
	Advance map[string]int `json:"advances"`
}

type fontDef struct {
	Providers []fontProvider `json:"providers"`
}

type rect struct{ x0, y0, x1, y1 int } // font units, y up from the baseline

type glyph struct {
	r       rune
	advance int
	rects   []rect
}

func (p *pack) collectGlyphs() (map[rune]glyph, error) {
	glyphs := map[rune]glyph{}
	var walk func(id string, depth int) error
	walk = func(id string, depth int) error {
		if depth > 8 {
			return fmt.Errorf("font reference loop at %s", id)
		}
		var def fontDef
		if err := p.readJSON(resourcePath(id, "font", ".json"), &def); err != nil {
			return err
		}
		for _, prov := range def.Providers {
			switch strings.TrimPrefix(prov.Type, "minecraft:") {
			case "reference":
				if strings.Contains(prov.ID, "unifont") {
					continue
				}
				if err := walk(prov.ID, depth+1); err != nil {
					return err
				}
			case "space":
				for s, adv := range prov.Advance {
					for _, r := range s {
						if _, ok := glyphs[r]; !ok {
							glyphs[r] = glyph{r: r, advance: adv * fontUnit}
						}
					}
				}
			case "bitmap":
				if err := p.addBitmapGlyphs(prov, glyphs); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := walk("minecraft:default", 0); err != nil {
		return nil, err
	}
	if _, ok := glyphs[' ']; !ok {
		glyphs[' '] = glyph{r: ' ', advance: spaceAdvance * fontUnit}
	}
	return glyphs, nil
}

// addBitmapGlyphs keeps the first definition of a character, matching how the
// game picks the first provider that has a glyph.
func (p *pack) addBitmapGlyphs(prov fontProvider, glyphs map[rune]glyph) error {
	sheet, err := p.texture(strings.Replace(prov.File, ".png", "", 1))
	if err != nil {
		return err
	}
	rows := len(prov.Chars)
	if rows == 0 {
		return nil
	}
	cols := len([]rune(prov.Chars[0]))
	cellW := sheet.Bounds().Dx() / cols
	cellH := sheet.Bounds().Dy() / rows
	height := 8
	if prov.Height != nil {
		height = *prov.Height
	}
	scale := float64(height) / float64(cellH)

	for row, line := range prov.Chars {
		for col, r := range []rune(line) {
			if r == 0 || r == ' ' {
				continue
			}
			if _, ok := glyphs[r]; ok {
				continue
			}
			// Width is the rightmost opaque column, plus one pixel of spacing.
			width := 0
			for x := cellW - 1; x >= 0 && width == 0; x-- {
				for y := 0; y < cellH; y++ {
					if sheet.NRGBAAt(col*cellW+x, row*cellH+y).A > 0 {
						width = x + 1
						break
					}
				}
			}
			if width == 0 {
				continue
			}

			g := glyph{r: r, advance: (int(math.Floor(0.5+float64(width)*scale)) + 1) * fontUnit}
			unit := scale * fontUnit
			for y := 0; y < cellH; y++ {
				for x := 0; x < width; {
					if sheet.NRGBAAt(col*cellW+x, row*cellH+y).A == 0 {
						x++
						continue
					}
					start := x
					for x < width && sheet.NRGBAAt(col*cellW+x, row*cellH+y).A > 0 {
						x++
					}
					top := float64(prov.Ascent)*fontUnit - float64(y)*unit
					g.rects = append(g.rects, rect{
						x0: int(math.Round(float64(start) * unit)), x1: int(math.Round(float64(x) * unit)),
						y0: int(math.Round(top - unit)), y1: int(math.Round(top)),
					})
				}
			}
			g.rects = mergeRects(g.rects)
			glyphs[r] = g
		}
	}
	return nil
}

// mergeRects joins horizontal runs that stack exactly on top of each other,
// which keeps glyph outlines small.
func mergeRects(in []rect) []rect {
	sort.Slice(in, func(i, j int) bool {
		if in[i].x0 != in[j].x0 {
			return in[i].x0 < in[j].x0
		}
		if in[i].x1 != in[j].x1 {
			return in[i].x1 < in[j].x1
		}
		return in[i].y0 < in[j].y0
	})
	var out []rect
	for _, r := range in {
		if n := len(out); n > 0 && out[n-1].x0 == r.x0 && out[n-1].x1 == r.x1 && out[n-1].y1 == r.y0 {
			out[n-1].y1 = r.y1
			continue
		}
		out = append(out, r)
	}
	return out
}

// ==== TrueType writer ====

type fontTable struct {
	tag  string
	data []byte
}

func buildTrueType(glyphMap map[rune]glyph, family string) ([]byte, error) {
	runes := make([]rune, 0, len(glyphMap))
	for r := range glyphMap {
		if r <= 0xFFFF && (r < 0xD800 || r > 0xDFFF) {
			runes = append(runes, r)
		}
	}
	sort.Slice(runes, func(i, j int) bool { return runes[i] < runes[j] })

	glyphs := []glyph{{advance: spaceAdvance * fontUnit}} // .notdef
	for _, r := range runes {
		glyphs = append(glyphs, glyphMap[r])
	}

	var glyf bytes.Buffer
	loca := make([]uint32, 0, len(glyphs)+1)
	var hmtx bytes.Buffer
	xMin, yMin, xMax, yMax := math.MaxInt16, math.MaxInt16, math.MinInt16, math.MinInt16
	maxPoints, maxContours, advanceMax := 0, 0, 0
	minLSB, minRSB, maxExtent := math.MaxInt16, math.MaxInt16, 0

	for _, g := range glyphs {
		loca = append(loca, uint32(glyf.Len()))
		lsb := 0
		if len(g.rects) > 0 {
			gx0, gy0, gx1, gy1 := bounds(g.rects)
			xMin, yMin, xMax, yMax = min(xMin, gx0), min(yMin, gy0), max(xMax, gx1), max(yMax, gy1)
			lsb = gx0
			minLSB, minRSB, maxExtent = min(minLSB, gx0), min(minRSB, g.advance-gx1), max(maxExtent, gx1)
			maxPoints, maxContours = max(maxPoints, len(g.rects)*4), max(maxContours, len(g.rects))
			writeGlyph(&glyf, g.rects, gx0, gy0, gx1, gy1)
			for glyf.Len()%4 != 0 {
				glyf.WriteByte(0)
			}
		}
		advanceMax = max(advanceMax, g.advance)
		be(&hmtx, uint16(g.advance), int16(lsb))
	}
	loca = append(loca, uint32(glyf.Len()))
	if minLSB == math.MaxInt16 {
		minLSB, minRSB = 0, 0
	}

	var locaBuf bytes.Buffer
	for _, off := range loca {
		be(&locaBuf, off)
	}

	ascender, descender := int16(8*fontUnit), int16(-1*fontUnit)
	winAscent, winDescent := uint16(max(yMax, int(ascender))), uint16(max(-yMin, int(-descender)))

	var head bytes.Buffer
	be(&head, uint32(0x00010000), uint32(0x00010000), uint32(0), uint32(0x5F0F3CF5),
		uint16(0x000B), uint16(unitsPerEm), int64(0), int64(0),
		int16(xMin), int16(yMin), int16(xMax), int16(yMax),
		uint16(0), uint16(8), int16(2), int16(1), int16(0))

	var hhea bytes.Buffer
	be(&hhea, uint32(0x00010000), ascender, descender, int16(0), uint16(advanceMax),
		int16(minLSB), int16(minRSB), int16(maxExtent), int16(1), int16(0), int16(0),
		int16(0), int16(0), int16(0), int16(0), int16(0), uint16(len(glyphs)))

	var maxp bytes.Buffer
	be(&maxp, uint32(0x00010000), uint16(len(glyphs)), uint16(maxPoints), uint16(maxContours),
		uint16(0), uint16(0), uint16(2), uint16(0), uint16(0), uint16(0), uint16(0), uint16(0),
		uint16(0), uint16(0), uint16(0))

	firstChar, lastChar := uint16(0x20), uint16(0x20)
	if len(runes) > 0 {
		firstChar, lastChar = uint16(runes[0]), uint16(runes[len(runes)-1])
	}
	var os2 bytes.Buffer
	be(&os2, uint16(4), int16(6*fontUnit), uint16(400), uint16(5), uint16(0),
		int16(5*fontUnit), int16(5*fontUnit), int16(0), int16(fontUnit), // subscript
		int16(5*fontUnit), int16(5*fontUnit), int16(0), int16(4*fontUnit), // superscript
		int16(fontUnit), int16(4*fontUnit), // strikeout
		int16(0))
	os2.Write(make([]byte, 10)) // panose
	be(&os2, uint32(0x00000007), uint32(0), uint32(0), uint32(0))
	os2.WriteString("HMZN")
	// Regular, and use the typo metrics so the line box matches the game's 9px lines.
	be(&os2, uint16(0x0040|0x0080), firstChar, lastChar, ascender, descender, int16(0),
		winAscent, winDescent, uint32(0x00000003), uint32(0), int16(5*fontUnit), int16(7*fontUnit),
		uint16(0), uint16(0x20), uint16(1))

	tables := []fontTable{
		{"OS/2", os2.Bytes()},
		{"cmap", buildCmap(runes)},
		{"glyf", glyf.Bytes()},
		{"head", head.Bytes()},
		{"hhea", hhea.Bytes()},
		{"hmtx", hmtx.Bytes()},
		{"loca", locaBuf.Bytes()},
		{"maxp", maxp.Bytes()},
		{"name", buildName(family)},
		{"post", buildPost()},
	}
	return assembleFont(tables), nil
}

func bounds(rs []rect) (int, int, int, int) {
	x0, y0, x1, y1 := rs[0].x0, rs[0].y0, rs[0].x1, rs[0].y1
	for _, r := range rs[1:] {
		x0, y0, x1, y1 = min(x0, r.x0), min(y0, r.y0), max(x1, r.x1), max(y1, r.y1)
	}
	return x0, y0, x1, y1
}

// writeGlyph emits each rect as a clockwise contour of four on-curve points.
func writeGlyph(w *bytes.Buffer, rs []rect, x0, y0, x1, y1 int) {
	be(w, int16(len(rs)), int16(x0), int16(y0), int16(x1), int16(y1))
	for i := range rs {
		be(w, uint16(i*4+3))
	}
	be(w, uint16(0)) // no instructions
	points := make([][2]int, 0, len(rs)*4)
	for _, r := range rs {
		points = append(points, [2]int{r.x0, r.y0}, [2]int{r.x0, r.y1}, [2]int{r.x1, r.y1}, [2]int{r.x1, r.y0})
	}
	for range points {
		w.WriteByte(0x01)
	}
	px := 0
	for _, pt := range points {
		be(w, int16(pt[0]-px))
		px = pt[0]
	}
	py := 0
	for _, pt := range points {
		be(w, int16(pt[1]-py))
		py = pt[1]
	}
}

func buildCmap(runes []rune) []byte {
	type segment struct{ start, end uint16 }
	var segs []segment
	for _, r := range runes {
		c := uint16(r)
		if n := len(segs); n > 0 && segs[n-1].end+1 == c {
			segs[n-1].end = c
			continue
		}
		segs = append(segs, segment{c, c})
	}
	segs = append(segs, segment{0xFFFF, 0xFFFF})

	var sub bytes.Buffer
	segCount := len(segs)
	searchRange := 2 * (1 << uint(math.Floor(math.Log2(float64(segCount)))))
	entrySelector := int(math.Log2(float64(searchRange / 2)))
	length := 16 + segCount*8
	be(&sub, uint16(4), uint16(length), uint16(0), uint16(segCount*2), uint16(searchRange),
		uint16(entrySelector), uint16(segCount*2-searchRange))
	for _, s := range segs {
		be(&sub, s.end)
	}
	be(&sub, uint16(0))
	for _, s := range segs {
		be(&sub, s.start)
	}
	glyphIndex := 1
	for _, s := range segs {
		if s.start == 0xFFFF {
			be(&sub, int16(1))
			continue
		}
		be(&sub, uint16(glyphIndex-int(s.start)))
		glyphIndex += int(s.end-s.start) + 1
	}
	for range segs {
		be(&sub, uint16(0))
	}

	var out bytes.Buffer
	be(&out, uint16(0), uint16(1), uint16(3), uint16(1), uint32(12))
	out.Write(sub.Bytes())
	return out.Bytes()
}

func buildName(family string) []byte {
	records := []struct {
		id    uint16
		value string
	}{
		{1, family}, {2, "Regular"}, {3, family + " Regular"}, {4, family}, {5, "Version 1.0"}, {6, strings.ReplaceAll(family, " ", "") + "-Regular"},
	}
	var strs bytes.Buffer
	var out bytes.Buffer
	be(&out, uint16(0), uint16(len(records)), uint16(6+12*len(records)))
	for _, rec := range records {
		encoded := utf16.Encode([]rune(rec.value))
		start := strs.Len()
		for _, u := range encoded {
			be(&strs, u)
		}
		be(&out, uint16(3), uint16(1), uint16(0x0409), rec.id, uint16(len(encoded)*2), uint16(start))
	}
	out.Write(strs.Bytes())
	return out.Bytes()
}

func buildPost() []byte {
	var out bytes.Buffer
	be(&out, uint32(0x00030000), int32(0), int16(-fontUnit), int16(fontUnit), uint32(0),
		uint32(0), uint32(0), uint32(0), uint32(0))
	return out.Bytes()
}

func assembleFont(tables []fontTable) []byte {
	sort.Slice(tables, func(i, j int) bool { return tables[i].tag < tables[j].tag })
	n := len(tables)
	searchRange := 16 * (1 << uint(math.Floor(math.Log2(float64(n)))))
	entrySelector := int(math.Log2(float64(searchRange / 16)))

	var out bytes.Buffer
	be(&out, uint32(0x00010000), uint16(n), uint16(searchRange), uint16(entrySelector), uint16(n*16-searchRange))

	offset := 12 + 16*n
	headOffset := 0
	var body bytes.Buffer
	for _, t := range tables {
		padded := append([]byte{}, t.data...)
		for len(padded)%4 != 0 {
			padded = append(padded, 0)
		}
		if t.tag == "head" {
			headOffset = offset
		}
		out.WriteString(t.tag)
		be(&out, checksum(padded), uint32(offset), uint32(len(t.data)))
		body.Write(padded)
		offset += len(padded)
	}
	out.Write(body.Bytes())

	font := out.Bytes()
	adjust := 0xB1B0AFBA - checksum(font)
	binary.BigEndian.PutUint32(font[headOffset+8:], adjust)
	return font
}

func checksum(b []byte) uint32 {
	var sum uint32
	for i := 0; i+3 < len(b); i += 4 {
		sum += binary.BigEndian.Uint32(b[i:])
	}
	return sum
}

func be(w *bytes.Buffer, values ...any) {
	for _, v := range values {
		_ = binary.Write(w, binary.BigEndian, v)
	}
}
