package musicxml

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"unicode/utf16"
)

const (
	maxFuzzXMLSize  = 1 << 20
	maxFuzzMXLSize  = 2 << 20
	maxFuzzLinkSize = 8 << 10
)

func FuzzDocumentRoundTrip(f *testing.F) {
	addDocumentFuzzSeeds(f)

	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > maxFuzzXMLSize {
			return
		}

		document, err := Decode(bytes.NewReader(input))
		if err != nil {
			return
		}

		var encoded bytes.Buffer
		if err := Encode(&encoded, document); err != nil {
			t.Fatalf("encode successfully decoded document: %v", err)
		}

		decodedAgain, err := Decode(bytes.NewReader(encoded.Bytes()))
		if err != nil {
			t.Fatalf("decode encoded document: %v", err)
		}
		if !reflect.DeepEqual(document, decodedAgain) {
			t.Fatal("document model changed after round-trip")
		}

		_ = Validate(document)
	})
}

func FuzzMXLPackageRoundTrip(f *testing.F) {
	addFileFuzzSeed(
		f,
		filepath.Join(
			musicXMLTestSuiteDirectory,
			"90a-Compressed-MusicXML.mxl",
		),
	)
	f.Add(mustEncodeFuzzOpusPackage(f))

	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > maxFuzzMXLSize {
			return
		}

		value, err := DecodeMXLPackage(bytes.NewReader(input))
		if err != nil {
			return
		}

		if _, ok := value.Document.(*OpusDocument); ok {
			resolved, resolveErr := value.ResolveOpus()
			if resolveErr == nil {
				if err := value.SyncResolvedOpus(resolved); err != nil {
					t.Fatalf("sync successfully resolved opus: %v", err)
				}
			}
		}

		var encoded bytes.Buffer
		if err := EncodeMXLPackage(&encoded, value); err != nil {
			t.Fatalf("encode successfully decoded MXL package: %v", err)
		}

		decodedAgain, err := DecodeMXLPackage(
			bytes.NewReader(encoded.Bytes()),
		)
		if err != nil {
			t.Fatalf("decode encoded MXL package: %v", err)
		}
		if !reflect.DeepEqual(value, decodedAgain) {
			t.Fatal("MXL package model changed after round-trip")
		}
	})
}

func FuzzMXLLinkResolution(f *testing.F) {
	f.Add("collections/main.musicxml", "../scores/first.musicxml")
	f.Add("main.musicxml", "score%20one.musicxml#movement")
	f.Add("collections/main.musicxml", "/scores/first.musicxml")
	f.Add("main.musicxml", "https://example.com/score.musicxml")
	f.Add("main.musicxml", "../../../score.musicxml")

	f.Fuzz(func(t *testing.T, sourcePath string, href string) {
		if len(sourcePath) > maxFuzzLinkSize ||
			len(href) > maxFuzzLinkSize {
			return
		}

		targetPath, _, err := resolveMXLLinkPath(sourcePath, href)
		if err != nil {
			return
		}
		if !validMXLContentPath(targetPath) {
			t.Fatalf("resolved invalid MXL content path %q", targetPath)
		}
	})
}

func addDocumentFuzzSeeds(f *testing.F) {
	f.Helper()

	f.Add([]byte(
		`<score-timewise version="4.0">` +
			`<part-list/>` +
			`</score-timewise>`,
	))
	f.Add(utf16FuzzSeed(
		`<?xml version="1.0" encoding="UTF-16"?>` +
			`<score-partwise version="4.0">` +
			`<part-list/>` +
			`</score-partwise>`,
	))

	for _, file := range []string{
		"01d-Pitches-Microtones.xml",
		"23d-Tuplets-Nested.xml",
		"32ad-Notations5.musicxml",
		"41d-StaffGroups-Nested.xml",
		"61j-Lyrics-Elisions.xml",
		"71e-TabStaves.xml",
		"73a-Percussion.xml",
	} {
		addFileFuzzSeed(
			f,
			filepath.Join(musicXMLTestSuiteDirectory, file),
		)
	}

	addFileFuzzSeed(
		f,
		filepath.Join("testdata", "official", "OpusLink.musicxml"),
	)
}

func addFileFuzzSeed(
	f *testing.F,
	path string,
) {
	f.Helper()

	input, err := os.ReadFile(path)
	if err != nil {
		f.Fatalf("read fuzz seed %q: %v", path, err)
	}

	f.Add(input)
}

func mustEncodeFuzzOpusPackage(f *testing.F) []byte {
	f.Helper()

	score := NewScorePartwise(PartDefinition{
		ID:   "P1",
		Name: "Piano",
	})
	var scoreXML bytes.Buffer
	if err := Encode(&scoreXML, score); err != nil {
		f.Fatalf("encode score fuzz seed: %v", err)
	}

	opus := NewOpusDocument()
	opus.AddScore(&OpusScore{
		Href: "../scores/score.musicxml",
	})
	value := &MXLPackage{
		Document: opus,
		RootFiles: []MXLRootFile{
			{
				FullPath:  "collections/main.musicxml",
				MediaType: musicXMLMIMEType,
			},
		},
		Resources: []MXLResource{
			{
				Path: "scores/score.musicxml",
				Data: scoreXML.Bytes(),
			},
			{
				Path: "images/cover.png",
				Data: []byte{0x89, 0x50, 0x4e, 0x47},
			},
		},
	}

	var result bytes.Buffer
	if err := EncodeMXLPackage(&result, value); err != nil {
		f.Fatalf("encode MXL fuzz seed: %v", err)
	}

	return result.Bytes()
}

func utf16FuzzSeed(value string) []byte {
	units := utf16.Encode([]rune(value))
	result := make([]byte, 2+len(units)*2)
	result[0] = 0xff
	result[1] = 0xfe

	for index, unit := range units {
		offset := 2 + index*2
		result[offset] = byte(unit)
		result[offset+1] = byte(unit >> 8)
	}

	return result
}
