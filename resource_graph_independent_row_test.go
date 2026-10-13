package musicxml

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func finalQAFinding(b validation.AssessmentBundle, reason validation.Code, outcome validation.Outcome) bool {
	for _, f := range b.PackageReport().Findings {
		if f.Reason == reason && f.Outcome == outcome {
			return true
		}
	}
	return false
}
func TestFinalQAImageSemanticTargets(t *testing.T) {
	for _, mode := range []string{"xml", "model", "raw-mxl", "current-mxl"} {
		for _, kind := range []string{"preserved", "changed", "one-dimension", "missing", "unselected", "missing-source", "missing-type"} {
			// Current Image fields have no presence bit: original omission is not representable.
			if (mode == "model" || mode == "current-mxl") && (kind == "missing-source" || kind == "missing-type") {
				continue
			}
			t.Run(mode+"/"+kind, func(t *testing.T) {
				attrs := `source="odd/allowed-folder/plot.dat" type="image/png" width="20" height="10"`
				if kind == "changed" {
					attrs = `source="odd/allowed-folder/plot.dat" type="image/png" width="21" height="10"`
				}
				if kind == "one-dimension" {
					attrs = `source="odd/allowed-folder/plot.dat" type="image/png" width="20"`
				}
				if kind == "missing-source" {
					attrs = `type="image/png" width="20" height="10"`
				}
				if kind == "missing-type" {
					attrs = `source="odd/allowed-folder/plot.dat" width="20" height="10"`
				}
				source := resourceScore(`<direction><direction-type><image ` + attrs + `/></direction-type></direction>`)
				calls := 0
				r := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
					calls++
					if q.Image == nil || q.Document != nil || q.Image.Href != "odd/allowed-folder/plot.dat" || q.Image.MediaType != "image/png" {
						t.Fatalf("unexpected image request %+v", q)
					}
					if q.Image.BaseURI != "https://final-qa.test/root.opus" {
						t.Fatalf("base lost %+v", q.Image)
					}
					wantEntry := ""
					if mode == "raw-mxl" || mode == "current-mxl" {
						wantEntry = "root.opus"
					}
					if q.Image.ArchiveEntry != wantEntry {
						t.Fatalf("entry %q want %q", q.Image.ArchiveEntry, wantEntry)
					}
					if kind == "missing" {
						return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceUnknown, Reason: "fixture-image-unavailable"}, nil
					}
					w, _ := validation.NewDecimal(big.NewRat(400, 1))
					h, _ := validation.NewDecimal(big.NewRat(200, 1))
					one, _ := validation.NewDecimal(big.NewRat(1, 1))
					m := validation.ImageMetadataResult{Request: *q.Image, Resource: validation.ResourcePin{CanonicalID: "image", Representation: validation.ResourceBinary, Digest: strings.Repeat("a", 64)}, Width: packageKnown(w), Height: packageKnown(h), Unit: validation.ImageUnitPixel, PixelAspectRatio: packageKnown(one), Authority: validation.EvidenceAuthority{ID: "independent-image-table", ExactRelationComplete: true, Provenance: []validation.SourceRef{{IDs: []string{"image-fixture"}, URL: "https://final-qa.test/image-table", Locator: "exact ratio"}}}}
					return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Image: &m}, nil
				})
				opts := finalQAOptions(r)
				opts.Dependencies = []ResourceDependency{ResourceImages}
				if kind == "unselected" {
					opts.Dependencies = nil
				}
				b, e := finalQARun(t, mode, source, context.Background(), opts)
				if e != nil {
					t.Fatal(e)
				}
				wantCalls := 1
				if kind == "unselected" || kind == "missing-source" || kind == "missing-type" {
					wantCalls = 0
				}
				if calls != wantCalls {
					t.Fatalf("calls%d want%d", calls, wantCalls)
				}
				if kind == "missing-source" || kind == "missing-type" {
					bad := false
					for i := 0; i < b.ResourceCount(); i++ {
						_, r, _ := b.ResourceReport(i)
						bad = bad || r.Conformance == validation.ConformanceViolated
					}
					if !bad || b.Satisfied() {
						t.Fatal("missing XSD required image field certified")
					}
					return
				}
				if b.ImageAdviceCount() != 1 || b.PackageReport().Counts.Fail != 0 {
					t.Fatalf("advice became mandatory failure: %+v", b.PackageReport())
				}
				a, _ := b.ImageAdviceAt(0)
				switch kind {
				case "preserved", "changed":
					if a.PreservesAspectRatio.State != validation.EvidenceKnown || a.PreservesAspectRatio.Value != (kind == "preserved") {
						t.Fatalf("ratio proof %+v", a)
					}
					if (kind == "changed") != finalQAFinding(b, "image-aspect-ratio-changed", validation.OutcomeFail) {
						t.Fatalf("ratio outcome %+v", b.PackageReport().Findings)
					}
				case "one-dimension":
					if a.PreservesAspectRatio.State != validation.EvidenceAbsent {
						t.Fatal("one dimension invented distortion", a)
					}
				default:
					if a.Availability.State != validation.EvidenceUnknown || a.PreservesAspectRatio.State != validation.EvidenceUnknown {
						t.Fatal("missing metadata invented ratio", a)
					}
				}
			})
		}
	}
}

func TestFinalQACompressedLinkFamilyBases(t *testing.T) {
	score := resourceScore("")
	tests := []struct{ name, source, path, correct, decoy string }{
		{"measure-link", resourceScore(`<link xlink:href="arbitrary-target.musicxml"/>`), "sets/root.musicxml", "arbitrary-target.musicxml", "sets/arbitrary-target.musicxml"},
		{"part-link", `<score-partwise xmlns:xlink="http://www.w3.org/1999/xlink"><part-list><score-part id="P1"><part-link xlink:href="arbitrary-target.musicxml"/><part-name>P</part-name></score-part></part-list><part id="P1"><measure number="1"/></part></score-partwise>`, "sets/root.musicxml", "arbitrary-target.musicxml", "sets/arbitrary-target.musicxml"},
		{"opus-score", `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="arbitrary-target.musicxml"/></opus>`, "sets/root.opus", "sets/arbitrary-target.musicxml", "arbitrary-target.musicxml"},
	}
	for _, mode := range []string{"raw-mxl", "current-mxl"} {
		for _, tc := range tests {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				var b validation.AssessmentBundle
				var e error
				if mode == "raw-mxl" {
					wire := packageGraphBytes(tc.source, tc.path, "", assessmentZIPFixture{name: tc.correct, body: score}, assessmentZIPFixture{name: tc.decoy, body: `<unexpected/>`})
					b, e = InspectMXL(context.Background(), bytes.NewReader(wire), packageGraphOptions())
				} else {
					d, x := Decode(strings.NewReader(tc.source))
					if x != nil {
						t.Fatal(x)
					}
					p := &MXLPackage{Document: d, RootFiles: []MXLRootFile{{FullPath: tc.path}}, Resources: []MXLResource{{Path: tc.correct, Data: []byte(score)}, {Path: tc.decoy, Data: []byte(`<unexpected/>`)}}}
					b, e = AssessMXLPackage(context.Background(), p, MXLAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileMXLModel}})
				}
				want := 2
				if mode == "raw-mxl" {
					want = 3
				}
				if e != nil || b.ResourceCount() != want || !b.EnumerationComplete() {
					t.Fatalf("wrong base err=%v resources=%d findings=%+v", e, b.ResourceCount(), b.PackageReport().Findings)
				}
				for i := 0; i < b.ResourceCount(); i++ {
					_, r, _ := b.ResourceReport(i)
					if r.Conformance == validation.ConformanceViolated {
						t.Fatalf("decoy/invalid selected %+v", r.Findings)
					}
				}
				found := false
				for i := 0; i < b.AdoptionCount(); i++ {
					a, _ := b.AdoptionContext(i)
					found = found || a.ArchiveEntry == tc.correct
					if a.ArchiveEntry == tc.decoy {
						t.Fatalf("wrong base selected %q", a.ArchiveEntry)
					}
				}
				if !found {
					t.Fatal("correct literal archive entry not adopted")
				}
			})
		}
	}
}

func finalQAMimeWire(root string, mime *assessmentZIPFixture, order []int, extra ...assessmentZIPFixture) []byte {
	entries := []assessmentZIPFixture{}
	if mime != nil {
		entries = append(entries, *mime)
	}
	entries = append(entries, assessmentZIPFixture{name: mxlContainerPath, body: `<container><rootfiles><rootfile full-path="main.musicxml"/></rootfiles></container>`}, assessmentZIPFixture{name: "main.musicxml", body: root})
	entries = append(entries, extra...)
	return assessmentZIPBytes(entries, order, "")
}
func TestFinalQAMimetypePrefixSemanticClassifiers(t *testing.T) {
	cases := []struct {
		name, body, reason string
		fail, unknown      bool
	}{
		{"utf8", "\xef\xbb\xbf" + mxlMIMEType, "mimetype-leading-bom", true, false},
		{"utf16be", "\xfe\xff" + mxlMIMEType, "mimetype-leading-bom", true, false},
		{"utf16le", "\xff\xfe" + mxlMIMEType, "mimetype-leading-bom", true, false},
		{"utf32be", "\x00\x00\xfe\xff" + mxlMIMEType, "mimetype-leading-bom", true, false},
		{"utf32le", "\xff\xfe\x00\x00" + mxlMIMEType, "mimetype-leading-bom", true, false},
		{"exact", mxlMIMEType, "", false, false}, {"empty", "", "", false, false}, {"trailing", mxlMIMEType + " \n", "", false, false},
		{"unresolved", "x" + mxlMIMEType, "mimetype-prefix-family-unresolved", false, true},
	}
	for _, p := range []byte{0, ' ', '\t', '\n', '\v', '\f', '\r'} {
		cases = append(cases, struct {
			name, body, reason string
			fail, unknown      bool
		}{"padding-" + string([]byte{p}), string([]byte{p}) + mxlMIMEType, "mimetype-leading-padding", true, false})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := assessmentZIPFixture{name: mxlMIMETypePath, body: tc.body}
			b, e := InspectMXL(context.Background(), bytes.NewReader(finalQAMimeWire(`<opus/>`, &m, nil)), packageGraphOptions())
			if e != nil {
				t.Fatal(e)
			}
			if tc.fail {
				if !finalQAFinding(b, validation.Code(tc.reason), validation.OutcomeFail) || b.PackageReport().Counts.Fail != 1 || b.Satisfied() {
					t.Fatalf("mandatory classifier lost: %+v", b.PackageReport())
				}
			} else if tc.unknown {
				if !finalQAFinding(b, validation.Code(tc.reason), validation.OutcomeUnknown) || b.Satisfied() {
					t.Fatalf("unreviewed prefix certified: %+v", b.PackageReport())
				}
			} else if !b.Satisfied() || b.PackageReport().Counts.Fail != 0 {
				t.Fatalf("allowed prefix rejected: %+v", b.PackageReport())
			}
		})
	}
}
func TestFinalQAMimetypeAdvisoryAndLegacyOmission(t *testing.T) {
	for _, kind := range []string{"exact", "missing-new", "missing-old", "wrong-content", "not-first", "central-order-only", "encrypted", "compressed"} {
		t.Run(kind, func(t *testing.T) {
			m := &assessmentZIPFixture{name: mxlMIMETypePath, body: mxlMIMEType}
			root := `<opus/>`
			var order []int
			reason := ""
			mandatory := false
			wantErr := false
			if strings.HasPrefix(kind, "missing-") {
				m = nil
				root = strings.Replace(resourceScore(""), "<score-partwise ", `<score-partwise version="4.0" `, 1)
				reason = "mimetype-recommended-presence"
				if kind == "missing-old" {
					root = strings.Replace(root, `version="4.0"`, `version="2.0"`, 1)
				}
			}
			if kind == "wrong-content" {
				m.body = mxlMIMEType + "\n"
				reason = "mimetype-recommended-content"
			}
			if kind == "central-order-only" {
				order = []int{2, 1, 0}
			}
			if kind == "compressed" {
				m.method = 8
				reason = "mimetype-local-compressed"
				mandatory = true
			}
			if kind == "encrypted" {
				m.flags = 1
				reason = "mimetype-local-encrypted"
				mandatory = true
				wantErr = true
			}
			wire := finalQAMimeWire(root, m, order)
			if kind == "not-first" {
				wire = assessmentZIPBytes([]assessmentZIPFixture{{name: mxlContainerPath, body: `<container><rootfiles><rootfile full-path="main.musicxml"/></rootfiles></container>`}, {name: mxlMIMETypePath, body: mxlMIMEType}, {name: "main.musicxml", body: root}}, []int{1, 0, 2}, "")
				reason = "mimetype-recommended-first"
			}
			b, e := InspectMXL(context.Background(), bytes.NewReader(wire), packageGraphOptions())
			if (e != nil) != wantErr {
				t.Fatalf("err=%v", e)
			}
			if reason != "" && !finalQAFinding(b, validation.Code(reason), validation.OutcomeFail) {
				t.Fatalf("missing %s %+v", reason, b.PackageReport().Findings)
			}
			if mandatory {
				if b.PackageReport().Conformance != validation.ConformanceViolated || b.Satisfied() {
					t.Fatal("mandatory transport failure lost")
				}
			} else if !b.Satisfied() || b.PackageReport().Counts.Fail != 0 {
				t.Fatalf("advice became rejection: %+v", b.PackageReport())
			}
		})
	}
}
func TestFinalQAUTF8EntryAdvicePublic(t *testing.T) {
	for _, tc := range []struct {
		name   string
		flags  uint16
		legacy bool
	}{{"ascii.dat", 0, false}, {"répertoire.dat", 0, false}, {"répertoire.dat", 2048, false}, {"legacy-\x82.dat", 0, true}} {
		t.Run(tc.name, func(t *testing.T) {
			m := assessmentZIPFixture{name: mxlMIMETypePath, body: mxlMIMEType}
			wire := finalQAMimeWire(`<opus/>`, &m, nil, assessmentZIPFixture{name: tc.name, body: "opaque", flags: tc.flags})
			b, e := InspectMXL(context.Background(), bytes.NewReader(wire), packageGraphOptions())
			if (e != nil && !tc.legacy) || b.PackageReport().Counts.Fail != 0 {
				t.Fatalf("encoding advice became mandatory err=%v %+v", e, b.PackageReport())
			}
			if tc.legacy && (e == nil || errors.Is(e, validation.ErrInvalidConfiguration)) {
				t.Fatalf("legacy path policy must remain distinct from core configuration: %v", e)
			}
			if finalQAFinding(b, "legacy-entry-name-encoding", validation.OutcomeFail) != tc.legacy {
				t.Fatalf("encoding advice mismatch %+v", b.PackageReport().Findings)
			}
		})
	}
}

func TestFinalQARootKindsAndFirstMediaPublic(t *testing.T) {
	timewise := `<score-timewise><part-list><score-part id="P1"><part-name>P</part-name></score-part></part-list><measure number="1"><part id="P1"/></measure></score-timewise>`
	for _, root := range []string{resourceScore(""), timewise, `<opus/>`} {
		name := "opus"
		if strings.Contains(root, "<score-partwise") {
			name = "partwise"
		}
		if strings.Contains(root, "<score-timewise") {
			name = "timewise"
		}
		for _, mode := range []string{"raw-mxl", "current-mxl"} {
			t.Run(mode+"/"+name, func(t *testing.T) {
				b, e := finalQARun(t, mode, root, context.Background(), finalQAOptions(nil))
				if e != nil || b.PackageReport().Counts.Fail != 0 || !b.EnumerationComplete() {
					t.Fatalf("supported kind failed: %v %+v", e, b.PackageReport())
				}
				for _, f := range b.PackageReport().Findings {
					if f.Rule.ID == "MX40-PACKAGE-first-root-musicxml" || f.Rule.ID == "MX40-PACKAGE-rootfile-root" {
						t.Fatalf("kind unavailable %+v", f)
					}
				}
			})
		}
	}
	for _, tc := range []struct {
		name, attr, reason string
		fail, unknown      bool
	}{
		{"absent", "", "", false, false}, {"empty", ` media-type=""`, "media-type-syntax-unresolved", false, true},
		{"case", ` media-type="APPLICATION/VND.RECORDARE.MUSICXML+XML"`, "", false, false},
		{"charset", ` media-type="application/vnd.recordare.musicxml+xml; charset=UTF-8"`, "", false, false},
		{"unknown-parameter", ` media-type="application/vnd.recordare.musicxml+xml; extension=1"`, "musicxml-media-parameter-unresolved", false, true},
		{"pdf", ` media-type="application/pdf"`, "established-nonmusicxml-media-type", true, false},
		{"unknown", ` media-type="application/x-unknown"`, "musicxml-media-type-set-unresolved", false, true},
	} {
		t.Run("media/"+tc.name, func(t *testing.T) {
			c := `<container><rootfiles><rootfile full-path="main.xml"` + tc.attr + `/></rootfiles></container>`
			wire := assessmentZIPBytes([]assessmentZIPFixture{{name: mxlMIMETypePath, body: mxlMIMEType}, {name: mxlContainerPath, body: c}, {name: "main.xml", body: `<opus/>`}}, nil, "")
			b, e := InspectMXL(context.Background(), bytes.NewReader(wire), packageGraphOptions())
			if e != nil {
				t.Fatal(e)
			}
			if tc.fail {
				if !finalQAFinding(b, validation.Code(tc.reason), validation.OutcomeFail) || b.Satisfied() {
					t.Fatalf("established negative lost %+v", b.PackageReport())
				}
			} else if tc.unknown {
				if !finalQAFinding(b, validation.Code(tc.reason), validation.OutcomeUnknown) || b.Satisfied() {
					t.Fatalf("uncertain set falsely proved %+v", b.PackageReport())
				}
			} else if !b.Satisfied() {
				t.Fatalf("supported media failed %+v", b.PackageReport())
			}
		})
	}
	for _, root := range []string{`<score-partwise xmlns="urn:foreign"/>`, `<score-timewise xmlns="urn:foreign"/>`, `<opus xmlns="urn:foreign"/>`} {
		t.Run("foreign/"+root, func(t *testing.T) {
			b, e := InspectMXL(context.Background(), bytes.NewReader(packageGraphBytes(root, "first.xml", "")), packageGraphOptions())
			if !errors.Is(e, ErrUnsupportedRoot) || !errors.Is(e, validation.ErrInvalidConfiguration) || b.Satisfied() || b.EnumerationComplete() || b.ResourceCount() != 1 || b.AdoptionCount() != 1 {
				t.Fatalf("foreign root accepted: %v %+v", e, b.PackageReport())
			}
			finalQAOriginalSourceFailure(t, b, root)
			if finalQAFinding(b, "wrong-rootfile-document-kind", validation.OutcomeFail) {
				t.Fatal("unadmitted assembly manufactured a package kind predicate")
			}
		})
	}
}

func TestFinalQALocalAttachmentSourceModelAxes(t *testing.T) {
	for _, model := range []bool{false, true} {
		for _, tc := range []struct {
			name, attrs, tail string
			state             validation.EvidenceState
			invalid           bool
		}{
			{"next-marker", `position="9"`, `<bookmark id="next"/>`, validation.EvidenceKnown, false},
			{"exact-descendant", `element="beam" position="2"`, `<note><rest/><duration>1</duration><beam number="1">begin</beam><beam number="2">begin</beam></note>`, validation.EvidenceKnown, false},
			{"absent-position", `element="beam"`, `<note><rest/><duration>1</duration><beam>begin</beam></note>`, validation.EvidenceUnknown, false},
			{"no-next", "", "", validation.EvidenceAbsent, false},
			{"invalid-position", `element="beam" position="0"`, `<note><rest/><duration>1</duration><beam>begin</beam></note>`, validation.EvidenceInvalid, true},
		} {
			label := "source"
			if model {
				label = "model"
			}
			t.Run(label+"/"+tc.name, func(t *testing.T) {
				s := resourceScore(`<link xlink:href="" ` + tc.attrs + `/>` + tc.tail)
				var r validation.Report
				var e error
				if model {
					d, x := Decode(strings.NewReader(s))
					if x != nil {
						t.Fatal(x)
					}
					r, e = AssessModel(context.Background(), d, AssessmentOptions{Profile: ProfileModelLinks})
				} else {
					r, e = InspectXML(context.Background(), strings.NewReader(s), AssessmentOptions{Profile: ProfileSourceLinks})
				}
				if e != nil || r.Satisfied() == tc.invalid {
					t.Fatalf("local result %v %+v", e, r)
				}
				found := false
				for _, c := range r.Context {
					if c.Resource != nil && c.Resource.Link != nil && c.Resource.Link.Kind == validation.ResourceScoreLink {
						found = true
						a := c.Resource.Link.Attachment
						if a.State != tc.state {
							t.Fatalf("attachment %+v expected %s", a, tc.state)
						}
						if tc.invalid && a.Cause == nil {
							t.Fatal("missing positiveInteger prerequisite cause")
						}
					}
				}
				if !found {
					t.Fatal("missing link")
				}
			})
		}
	}
}
