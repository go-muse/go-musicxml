package musicxml

import (
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestLinkContextInitialProfileAndHintComposition(t *testing.T) {
	// Adapted from the genuine, unpublished batch2-final-probes
	// qa_composed_contract_test.go and qa_composition_test.go. The old probes
	// remain preserved; this current-API regression isolates hint discovery
	// from the unchanged facet-free list(anyURI) datatype contract.
	source := `<opus xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xlink="http://www.w3.org/1999/xlink" xsi:schemaLocation="urn:a a.xsd trailing"><score xlink:href="score.musicxml"/></opus>`
	old, err := InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceLocal})
	if err != nil || !old.Satisfied() {
		t.Fatalf("odd list datatype baseline: err=%v report=%+v", err, old)
	}
	got, err := InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: validation.ProfileID("musicxml-4.0/source-links")})
	if err != nil {
		t.Fatalf("new nonfetching local link profile unavailable: %v", err)
	}
	properties := map[validation.ContextProperty]bool{}
	for _, fact := range got.Context {
		properties[fact.Property] = true
	}
	for _, property := range []validation.ContextProperty{"resource.link", "resource.schema-hints"} {
		if !properties[property] {
			t.Errorf("missing finalized %s projection", property)
		}
	}
}

func TestLinkContextCatalogBuild(t *testing.T) {
	if _, err := newLinkAssessmentEngine(); err != nil {
		t.Fatal(err)
	}
}

func resourceScore(body string) string {
	return `<score-partwise xmlns:xlink="http://www.w3.org/1999/xlink"><part-list><score-part id="P1"><part-name>P</part-name></score-part></part-list><part id="P1"><measure number="1">` + body + `</measure></part></score-partwise>`
}
func linkResources(t *testing.T, source string) (validation.Report, []validation.ContextResult) {
	t.Helper()
	report, err := InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceLinks})
	if err != nil {
		t.Fatal(err)
	}
	var facts []validation.ContextResult
	for _, fact := range report.Context {
		if fact.Resource != nil {
			facts = append(facts, fact)
		}
	}
	return report, facts
}
func TestLinkContextSourceAttachmentAndDefaults(t *testing.T) {
	cases := []struct {
		name, attrs, following string
		state                  validation.EvidenceState
		target                 string
	}{
		{"next-marker", `position="9999999999999999999999999999"`, `<bookmark id="target"/><note><rest/><duration>1</duration></note>`, validation.EvidenceKnown, "bookmark"},
		{"explicit-skip", `element="beam" position="2"`, `<bookmark id="target"/><note><rest/><duration>1</duration><beam number="1">begin</beam><beam number="2">begin</beam></note>`, validation.EvidenceKnown, "beam[2]"},
		{"missing-position", `element="beam"`, `<note><rest/><duration>1</duration><beam>begin</beam></note>`, validation.EvidenceUnknown, ""},
		{"last-marker", "", "", validation.EvidenceAbsent, ""},
		{"huge", `element="beam" position="99999999999999999999999999999"`, `<note><rest/><duration>1</duration><beam>begin</beam></note>`, validation.EvidenceAbsent, ""},
		{"colon", `element="q:beam" position="1"`, `<note><rest/><duration>1</duration><beam>begin</beam></note>`, validation.EvidenceUnsupported, ""},
		{"two-parents", `element="words" position="1"`, `<direction><direction-type><words>A</words></direction-type><direction-type><words>B</words></direction-type></direction>`, validation.EvidenceUnknown, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			report, facts := linkResources(t, resourceScore(`<link xlink:href="  other.musicxml#  " `+tc.attrs+`/>`+tc.following))
			if !report.Satisfied() {
				t.Fatalf("optional link context became mandatory failure: %+v", report.Findings)
			}
			var link *validation.ResourceLinkContext
			var fact validation.ContextResult
			for _, v := range facts {
				if v.Resource.Link != nil && v.Resource.Link.Kind == validation.ResourceScoreLink {
					link = v.Resource.Link
					fact = v
					break
				}
			}
			if link == nil {
				t.Fatal("missing link context")
			}
			if link.Attachment.State != tc.state {
				t.Fatalf("attachment=%+v; report=%+v", link.Attachment, report)
			}
			if link.Href.Value.State != validation.EvidenceKnown || link.Href.Value.Value.Text != "other.musicxml#" || link.Href.Raw.Value != "  other.musicxml#  " {
				t.Fatalf("href raw/normalized lost: %+v", link.Href)
			}
			if link.Show.Presence.State != validation.EvidenceAbsent || link.Show.Value.Value.Text != "replace" || link.Actuate.Value.Value.Text != "onRequest" {
				t.Fatalf("XLink defaults: %+v", link)
			}
			if link.Fragment.State != validation.EvidenceKnown || link.Fragment.Value != "" {
				t.Fatalf("fragment delimiter lost: %+v", link.Fragment)
			}
			if fact.Resource.Part.State != validation.EvidenceKnown {
				t.Fatalf("part identity unavailable: %+v", fact.Resource.Part)
			}
			if tc.target != "" && link.Attachment.Value.Node == 0 {
				t.Fatal("missing target ref")
			}
		})
	}
}
func TestLinkContextInvalidAndEmptyRemainDistinct(t *testing.T) {
	for _, attrs := range []string{`xlink:href="" xlink:show=""`, `xlink:href="%GG"`, `xlink:show="replace"`} {
		report, facts := linkResources(t, resourceScore(`<link `+attrs+`/><note><rest/><duration>1</duration></note>`))
		if report.Satisfied() {
			t.Fatalf("invalid required/enum/URI value accepted: %s", attrs)
		}
		for _, fact := range facts {
			if link := fact.Resource.Link; link != nil {
				if strings.Contains(attrs, `show=""`) {
					if link.Show.Value.State != validation.EvidenceInvalid || link.Show.Value.Cause == nil || link.Show.Raw.State != validation.EvidenceKnown || link.Show.Raw.Value != "" {
						t.Fatalf("empty replaced by default: %+v", link.Show)
					}
					if link.Href.Value.State != validation.EvidenceKnown || link.Href.Value.Value.Text != "" {
						t.Fatalf("present empty URI not known: %+v", link.Href)
					}
				} else if link.Href.Value.State != validation.EvidenceInvalid || link.Href.Value.Cause == nil {
					t.Fatalf("invalid/missing href lacks XSD cause: %+v", link.Href)
				}
			}
		}
	}
	_, facts := linkResources(t, resourceScore(`<link xlink:href="" xlink:actuate="onLoad"/><note><rest/><duration>1</duration></note>`))
	for _, fact := range facts {
		if link := fact.Resource.Link; link != nil {
			if link.Href.Presence.State != validation.EvidenceKnown || link.Href.Value.Value.Text != "" || link.Fragment.State != validation.EvidenceAbsent || link.Actuate.Value.Value.Text != "onLoad" {
				t.Fatalf("empty URI/onLoad metadata changed: %+v", link)
			}
		}
	}
}
func TestLinkContextImagesGroupsAndScopedInstrument(t *testing.T) {
	source := `<score-partwise xmlns:xlink="http://www.w3.org/1999/xlink"><credit><credit-image source="photo.jpg" type="image/png" width="12"/></credit><part-list><score-part id="P1"><part-link xlink:href="parts.musicxml"><instrument-link id="I1"/><group-link> exact group </group-link></part-link><part-name>P</part-name><group> exact group </group><score-instrument id="I1"><instrument-name>P</instrument-name></score-instrument></score-part></part-list><part id="P1"><measure number="1"><direction><direction-type><image source="plot.dat" type="application/postscript" width="5" height="7"/></direction-type></direction></measure></part></score-partwise>`
	report, facts := linkResources(t, source)
	if !report.Satisfied() {
		t.Fatalf("valid linked metadata fixture: %+v", report.Findings)
	}
	images, groups, instruments := 0, 0, 0
	for _, fact := range facts {
		r := fact.Resource
		if image := r.Image; image != nil {
			images++
			want := "independent-x-y"
			mime := "application/postscript"
			if image.Credit {
				want = "proportional"
				mime = "image/png"
			}
			if image.Scaling.Value != want || image.MediaType.Value.Value.Text != mime {
				t.Fatalf("image advice/type: %+v", image)
			}
		}
		if group := r.Group; group != nil {
			groups++
			if group.Value.Value != " exact group " {
				t.Fatalf("group string normalized: %+v", group)
			}
		}
		if instrument := r.Instrument; instrument != nil {
			instruments++
			if instrument.Target.State != validation.EvidenceKnown || instrument.ID.Value.Value.Text != "I1" || r.Part.State != validation.EvidenceKnown {
				t.Fatalf("typed same-part reference: %+v", r)
			}
		}
	}
	if images != 2 || groups != 2 || instruments != 1 {
		t.Fatalf("images=%d groups=%d instruments=%d", images, groups, instruments)
	}
}
func TestLinkContextSchemaHintsDiscoveryIsIndependent(t *testing.T) {
	cases := []struct {
		name, hint string
		pairs      int
		complete   validation.EvidenceState
		trailing   string
	}{
		{"empty", "", 0, validation.EvidenceKnown, ""},
		{"pair", "urn:a a.xsd", 1, validation.EvidenceKnown, ""},
		{"odd", "urn:a a.xsd extra", 1, validation.EvidenceKnown, "extra"},
		{"nbsp", "urn:a\u00a0a.xsd", 0, validation.EvidenceKnown, "urn:a\u00a0a.xsd"},
		{"invalid", "urn:a %GG", 1, validation.EvidenceInvalid, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := `<opus xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="` + tc.hint + `"/>`
			report, facts := linkResources(t, source)
			var hints *validation.ResourceSchemaHintsContext
			for _, fact := range facts {
				if fact.Resource.SchemaHints != nil {
					hints = fact.Resource.SchemaHints
					break
				}
			}
			if hints == nil || len(hints.Pairs) != tc.pairs || hints.Complete.State != tc.complete || hints.Trailing.Value != tc.trailing {
				t.Fatalf("hints=%+v findings=%+v", hints, report.Findings)
			}
			if tc.complete == validation.EvidenceInvalid {
				if report.Satisfied() || hints.Complete.Cause == nil {
					t.Fatal("independent datatype failure lost")
				}
			} else if !report.Satisfied() {
				t.Fatalf("discovery context became datatype failure: %+v", report)
			}
		})
	}
}
func TestLinkContextModelCurrentFactsDoNotRecoverSourceHints(t *testing.T) {
	source := resourceScore(`<link xlink:href="old.musicxml"/><note><rest/><duration>1</duration></note>`)
	doc, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	report, err := AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelLinks})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, fact := range report.Context {
		if fact.Resource == nil {
			continue
		}
		if h := fact.Resource.SchemaHints; h != nil {
			found = true
			if h.Complete.State != validation.EvidenceUnsupported || h.Raw.State == validation.EvidenceAbsent {
				t.Fatalf("discarded source hints fabricated: %+v", h)
			}
		}
	}
	if !found {
		t.Fatal("missing explicit model source-history limit")
	}
}

func TestLinkContextIndependentInvalidHintAndUnavailableValue(t *testing.T) {
	// Current-API adaptation of batch2-final-probes/qa_composition_test.go:
	// preserve the independent bad hint alongside unsupported ENTITY context.
	source := `<score-partwise xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xs="http://www.w3.org/2001/XMLSchema"><movement-title xsi:type="xs:ENTITY" xsi:schemaLocation="%GG">image</movement-title><part-list><score-part id="P1"><part-name>P</part-name></score-part></part-list><part id="P1"><measure number="1"/></part></score-partwise>`
	report, facts := linkResources(t, source)
	if report.Conformance != validation.ConformanceViolated || report.AssessmentComplete {
		t.Fatalf("independent failure/capability composition lost: conformance=%s complete=%v findings=%+v", report.Conformance, report.AssessmentComplete, report.Findings)
	}
	found := false
	for _, fact := range facts {
		if h := fact.Resource.SchemaHints; h != nil && h.Raw.Value == "%GG" {
			found = true
			if h.Complete.State != validation.EvidenceInvalid || h.Complete.Cause == nil {
				t.Fatalf("hint invalidity lost: %+v", h)
			}
		}
	}
	if !found {
		t.Fatal("missing independent hint")
	}
}

func TestLinkContextTimewiseCreditAndForeignHomonyms(t *testing.T) {
	source := `<score-timewise xmlns:xlink="http://www.w3.org/1999/xlink"><credit><link xlink:href="#mark"/><bookmark id="mark"/><credit-words>T</credit-words></credit><part-list><score-part id="P1"><part-name>P</part-name></score-part></part-list><measure number="1"><part id="P1"><link xlink:href="child.musicxml"/><note><rest/><duration>1</duration></note></part></measure></score-timewise>`
	for _, model := range []bool{false, true} {
		var report validation.Report
		var err error
		if model {
			var doc Document
			doc, err = Decode(strings.NewReader(source))
			if err == nil {
				report, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: ProfileModelLinks})
			}
		} else {
			report, err = InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceLinks})
		}
		if err != nil {
			t.Fatal(err)
		}
		links := 0
		for _, fact := range report.Context {
			if fact.Resource == nil || fact.Resource.Link == nil {
				continue
			}
			link := fact.Resource.Link
			links++
			if link.Attachment.State != validation.EvidenceKnown {
				t.Fatalf("model=%v local attachment=%+v", model, link.Attachment)
			}
			if link.Href.Value.Value.Text == "child.musicxml" && fact.Resource.Part.State != validation.EvidenceKnown {
				t.Fatalf("timewise part unavailable: %+v", fact.Resource.Part)
			}
		}
		if links != 3 {
			t.Fatalf("model=%v links=%d", model, links)
		}
	}
	// A foreign namesake never acquires local link semantics from spelling.
	_, facts := linkResources(t, resourceScore(`<q:link xmlns:q="urn:foreign" xlink:href="other"/>`))
	for _, fact := range facts {
		if fact.Resource.Link != nil {
			t.Fatalf("foreign homonym acquired link role: %+v", fact)
		}
	}
}

func TestLinkContextModelEditsAndOwnedReports(t *testing.T) {
	source := resourceScore(`<link xlink:href="old.musicxml#old"/><bookmark id="target"/><note><rest/><duration>1</duration></note>`)
	doc, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	score := doc.(*ScorePartwise)
	score.Part[0].Measure[0].Content[0].Link.Href = "current.musicxml#"
	report, err := AssessModel(context.Background(), score, AssessmentOptions{Profile: ProfileModelLinks})
	if err != nil {
		t.Fatal(err)
	}
	var old *validation.ResourceLinkContext
	for _, fact := range report.Context {
		if fact.Resource != nil && fact.Resource.Link != nil && fact.Resource.Link.Kind == validation.ResourceScoreLink {
			old = fact.Resource.Link
		}
	}
	if old == nil || old.Href.Value.Value.Text != "current.musicxml#" || old.Fragment.State != validation.EvidenceKnown || old.Fragment.Value != "" || old.Href.Raw.State == validation.EvidenceKnown {
		t.Fatalf("stale source or fabricated lexeme: %+v", old)
	}
	score.Part[0].Measure[0].Content[0].Link.Href = "latest.musicxml"
	old.Href.Value.Value.Text = "caller-edit"
	next, err := AssessModel(context.Background(), score, AssessmentOptions{Profile: ProfileModelLinks})
	if err != nil {
		t.Fatal(err)
	}
	for _, fact := range next.Context {
		if fact.Resource != nil && fact.Resource.Link != nil && fact.Resource.Link.Kind == validation.ResourceScoreLink {
			if got := fact.Resource.Link.Href.Value.Value.Text; got != "latest.musicxml" {
				t.Fatalf("borrowed report/model cache: %q", got)
			}
		}
	}
}

func TestLinkContextOpusFamiliesReuseLocalVersionAndPagination(t *testing.T) {
	source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus><score xlink:href="part.musicxml"/></opus><opus-link xlink:href="other.opus"/></opus>`
	report, facts := linkResources(t, source)
	kinds := map[validation.ResourceLinkKind]int{}
	for _, fact := range facts {
		if link := fact.Resource.Link; link != nil {
			kinds[link.Kind]++
			if link.Show.Value.Value.Text != "replace" || link.Actuate.Value.Value.Text != "onRequest" {
				t.Fatalf("Opus defaults: %+v", link)
			}
		}
	}
	if kinds[validation.ResourceOpusScore] != 1 || kinds[validation.ResourceOpusLink] != 1 {
		t.Fatalf("assembly families: %+v", kinds)
	}
	versions, pages := 0, 0
	for _, fact := range report.Context {
		switch fact.Property {
		case "layout.opus-version":
			versions++
			if fact.Value.Value.Text != "1.0" {
				t.Fatal("version default changed")
			}
		case "layout.opus-new-page":
			pages++
			if fact.Value.State != validation.EvidenceUnknown {
				t.Fatal("application page choice fabricated")
			}
		}
	}
	if versions != 2 || pages != 1 {
		t.Fatalf("reused local Opus facts: version=%d page=%d", versions, pages)
	}
	// The score assembly's same-spelled opus use is a link, not an inline Opus root.
	score := `<score-partwise xmlns:xlink="http://www.w3.org/1999/xlink"><work><opus xlink:href="catalog.opus"/></work><part-list><score-part id="P1"><part-name>P</part-name></score-part></part-list><part id="P1"><measure number="1"/></part></score-partwise>`
	_, facts = linkResources(t, score)
	found := false
	for _, fact := range facts {
		if link := fact.Resource.Link; link != nil {
			found = true
			if link.Kind != validation.ResourceScoreWorkOpus || fact.Resource.SubjectDeclaration.Value.Assembly != validation.AssemblyMusicXML {
				t.Fatalf("wrong opus assembly: %+v", fact)
			}
		}
	}
	if !found {
		t.Fatal("score-work opus template missing")
	}
}

func TestLinkContextNewTemporaryOwnersReleaseOnLimits(t *testing.T) {
	source := resourceScore(`<link xlink:href="score.musicxml"/><note><rest/><duration>1</duration></note>`)
	for _, budgets := range []validation.Budgets{{}, {MaxContextFacts: 1}, {MaxBufferedBytes: 64 * 1024}, {MaxRuleSteps: 1000}} {
		account := adapterAccount(t, budgets)
		config := adapterSourceConfig(t, account, "resource-test", "scores/main.musicxml", ProfileSourceLinks, sourceBytesPrimary)
		_, _ = adapterInspect(t, strings.NewReader(source), config)
		adapterClean(t, account)
	}
}
