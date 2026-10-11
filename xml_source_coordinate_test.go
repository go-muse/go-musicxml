package musicxml

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

func TestSourceCursorCoordinateWidth(t *testing.T) {
	maximum := int(^uint(0) >> 1)
	for _, tc := range []struct {
		name, text   string
		line, column int
		want         [][2]int
	}{
		{"column-ascii", "ab\nZ", 7, maximum - 1, [][2]int{{7, maximum}, {0, 0}, {0, 0}, {0, 0}}},
		{"column-unicode", "é𝄞\r\nZ", 7, maximum - 1, [][2]int{{7, maximum}, {0, 0}, {0, 0}, {0, 0}}},
		{"line", "\n\r\nZ", maximum - 1, 7, [][2]int{{maximum, 1}, {0, 0}, {0, 0}}},
		{"newline-at-max-column", "\rZ", 7, maximum, [][2]int{{8, 1}, {8, 2}}},
		{"already-omitted", "a\né", 0, 0, [][2]int{{0, 0}, {0, 0}, {0, 0}}},
	} {
		for encoding, input := range map[string][]byte{
			"utf8":    []byte("----" + tc.text),
			"utf16be": encodeUTF16("----"+tc.text, binary.BigEndian),
			"utf16le": encodeUTF16("----"+tc.text, binary.LittleEndian),
		} {
			for _, fragmented := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/fragmented=%t", tc.name, encoding, fragmented), func(t *testing.T) {
					var inputReader io.Reader = bytes.NewReader(input)
					if fragmented {
						inputReader = iotest.OneByteReader(inputReader)
					}
					c, err := newSourceXMLCursor(inputReader, context.Background(), nil)
					if err != nil {
						t.Fatal(err)
					}
					defer c.close()
					for range 4 {
						if _, err := c.take(); err != nil {
							t.Fatal(err)
						}
					}
					consumed := c.position.ByteOffset
					const base int64 = 1 << 33
					c.position = sourceXMLPosition{ByteOffset: base, Line: tc.line, Column: tc.column}
					c.rawOffset = base
					previous := c.position
					for i, want := range tc.want {
						peeked, err := c.peek()
						if err != nil {
							t.Fatal(err)
						}
						if c.position != previous {
							t.Fatal("peek advanced committed position")
						}
						u, err := c.take()
						if err != nil {
							t.Fatal(err)
						}
						if u != peeked || u.Start != previous || u.End != c.position {
							t.Fatalf("unit %d cache/position mismatch: %+v", i, u)
						}
						if u.End.Line != want[0] || u.End.Column != want[1] {
							t.Errorf("unit %d coordinate=(%d,%d), want %v", i, u.End.Line, u.End.Column, want)
						}
						if u.End.ByteOffset <= u.Start.ByteOffset {
							t.Errorf("unit %d lost byte position: %+v", i, u)
						}
						previous = u.End
					}
					if _, err := c.take(); err != io.EOF {
						t.Fatalf("end: %v", err)
					}
					if want := base + int64(len(input)) - consumed; c.position.ByteOffset != want {
						t.Fatalf("byte offset=%d, want %d", c.position.ByteOffset, want)
					}
				})
			}
		}
	}
}

func TestSourceASCIISpanCoordinateWidth(t *testing.T) {
	maximum := int(^uint(0) >> 1)
	for _, tc := range []struct {
		name, text   string
		line, column int
		want         [2]int
	}{
		{"exact-column", "a", 7, maximum - 1, [2]int{7, maximum}},
		{"column-overflow", "ab\nZ", 7, maximum - 1, [2]int{0, 0}},
		{"exact-line", "\nZ", maximum - 1, 7, [2]int{maximum, 2}},
		{"line-overflow", "\n\nZ", maximum - 1, 7, [2]int{0, 0}},
		{"newline-at-max-column", "\nZ", 7, maximum, [2]int{8, 2}},
		{"already-omitted", "a\nZ", 0, 0, [2]int{0, 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := newSourceXMLCursor(strings.NewReader("----"+tc.text+"<"), context.Background(), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer c.close()
			for range 4 {
				if _, err := c.take(); err != nil {
					t.Fatal(err)
				}
			}
			const base int64 = 1 << 33
			c.position = sourceXMLPosition{ByteOffset: base, Line: tc.line, Column: tc.column}
			c.rawOffset = base
			span := c.asciiSpan(sourceASCIIText, 0, 100)
			if string(span) != tc.text {
				t.Fatalf("span=%q", span)
			}
			c.consumeASCII(span)
			if c.position.Line != tc.want[0] || c.position.Column != tc.want[1] {
				t.Fatalf("position=%+v, want coordinates=%v", c.position, tc.want)
			}
			if c.position.ByteOffset != base+int64(len(tc.text)) {
				t.Fatalf("byte offset=%d", c.position.ByteOffset)
			}
		})
	}
}

func TestSourceTokenCoordinateWidthPreservesXML(t *testing.T) {
	const input = "<!--a--><root a=\"v\">é\n𝄞</root>"
	r, err := newSourceXMLTokenReader(strings.NewReader(input), sourceXMLConfig{Context: context.Background()})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.cursor.position.Column = int(^uint(0)>>1) - 5
	starts := 0
	for {
		token, err := r.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if start, ok := token.(xml.StartElement); ok {
			starts++
			meta := r.Metadata()
			if start.Name.Local != "root" || meta.Span.Start.ByteOffset != 8 {
				t.Fatalf("root=%+v metadata=%+v", start, meta)
			}
			positions := []sourceXMLPosition{meta.Span.Start, meta.Span.End}
			for _, attr := range meta.Attributes {
				positions = append(positions, attr.NameSpan.Start, attr.NameSpan.End, attr.ValueSpan.Start, attr.ValueSpan.End)
			}
			for _, p := range positions {
				if p.Line != 0 || p.Column != 0 {
					t.Errorf("overflow coordinate was not omitted: %+v", p)
				}
			}
		}
	}
	if summary := r.Summary(); starts != 1 || !summary.SyntaxComplete || !summary.InputComplete || summary.BytesConsumed != int64(len(input)) {
		t.Fatalf("starts=%d summary=%+v", starts, summary)
	}
}
