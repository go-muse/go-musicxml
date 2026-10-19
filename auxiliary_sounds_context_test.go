package musicxml

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func inspectSounds(t *testing.T, source string) validation.Report {
	t.Helper()
	r, err := InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: ProfileSourceSounds})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func soundMappings(t *testing.T, r validation.Report) []validation.ContextResult {
	t.Helper()
	var out []validation.ContextResult
	for _, v := range r.Context {
		if v.Property == validation.ContextPropertySoundMapping {
			if v.SoundMapping == nil {
				t.Fatal("missing typed sound mapping")
			}
			out = append(out, v)
		}
	}
	return out
}
func TestAuxiliarySoundsDirectionalMappings(t *testing.T) {
	source := `<sounds><sound id=" a " primary=" yes "><any primary="no"> A  B&#x9;C </any><any primary="yes">alternative</any><any/></sound><sound id="b"><solo primary="yes">solo</solo><solo primary="yes"/><ensemble primary="no" number=" many:players "> ensemble </ensemble><ensemble/></sound><sound id="c"/></sounds>`
	r := inspectSounds(t, source)
	if !r.Satisfied() {
		t.Fatalf("valid mappings rejected: %+v", r)
	}
	got := soundMappings(t, r)
	want := []struct {
		role                             validation.SoundMappingRole
		owner, id, text, primary, number string
	}{
		{validation.SoundMappingSound, "a", "a", "", "yes", ""},
		{validation.SoundMappingAny, "a", "a", " A  B\tC ", "no", ""},
		{validation.SoundMappingAny, "a", "a", "alternative", "yes", ""},
		{validation.SoundMappingAny, "a", "a", "", "", ""},
		{validation.SoundMappingSound, "b", "b", "", "", ""},
		{validation.SoundMappingSolo, "b", "b", "solo", "yes", ""},
		{validation.SoundMappingSolo, "b", "b", "", "yes", ""},
		{validation.SoundMappingEnsemble, "b", "b", " ensemble ", "no", "many:players"},
		{validation.SoundMappingEnsemble, "b", "b", "", "", ""},
		{validation.SoundMappingSound, "c", "c", "", "", ""},
	}
	if len(got) != len(want) {
		t.Fatalf("mapping count %d want %d", len(got), len(want))
	}
	owners := map[string]validation.NodeRef{}
	seen := map[validation.NodeRef]bool{}
	for i, w := range want {
		result := got[i]
		v := result.SoundMapping
		if seen[result.Subject] {
			t.Error("alternatives collapsed")
		}
		seen[result.Subject] = true
		if w.role == validation.SoundMappingSound {
			owners[w.owner] = result.Subject
		}
		direction := validation.SoundMappingFromMusicXML
		libraryState := validation.EvidenceKnown
		if w.role == validation.SoundMappingSound {
			direction = validation.SoundMappingToMusicXML
			libraryState = validation.EvidenceAbsent
		}
		primaryState, numberState := validation.EvidenceKnown, validation.EvidenceKnown
		if w.primary == "" {
			primaryState = validation.EvidenceAbsent
		}
		if w.number == "" {
			numberState = validation.EvidenceAbsent
		}
		if v.Role != w.role || v.Direction != direction || v.Sound != owners[w.owner] || v.SoundID.State != validation.EvidenceKnown || v.SoundID.Value.Text != w.id || v.LibraryID.State != libraryState || v.LibraryID.Value.Text != w.text || v.Primary.State != primaryState || v.Primary.Value.Text != w.primary || v.Number.State != numberState || v.Number.Value.Text != w.number || v.Content.State != validation.EvidenceKnown || !v.Content.Value {
			t.Errorf("mapping %d: %+v", i, v)
		}
		if len(result.Origins) != 1 || result.Origins[0].Position == nil || result.Origins[0].DocumentID != result.Subject.DocumentID {
			t.Errorf("mapping %d lost original location", i)
		}
	}
}

func TestAuxiliarySoundsCausalInvalidFields(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		index        int
		field        string
	}{
		{"missing id", `<sounds><sound><any/></sound></sounds>`, 0, "id"},
		{"invalid id", `<sounds><sound id="1bad"><any/></sound></sounds>`, 1, "id"},
		{"reverse primary", `<sounds><sound id="a" primary="maybe"/></sounds>`, 0, "primary"},
		{"any primary", `<sounds><sound id="a"><any primary="maybe"/></sound></sounds>`, 1, "primary"},
		{"solo primary", `<sounds><sound id="a"><solo primary="maybe"/></sound></sounds>`, 1, "primary"},
		{"ensemble primary", `<sounds><sound id="a"><ensemble primary="maybe"/></sound></sounds>`, 1, "primary"},
		{"ensemble number", `<sounds><sound id="a"><ensemble number="two players"/></sound></sounds>`, 1, "number"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := inspectSounds(t, tc.source)
			if r.Conformance != validation.ConformanceViolated {
				t.Fatal("invalid field did not fail XSD")
			}
			rows := soundMappings(t, r)
			if len(rows) <= tc.index {
				t.Fatalf("lost independent mappings: %+v", rows)
			}
			v := rows[tc.index].SoundMapping
			e := v.Primary
			if tc.field == "id" {
				e = v.SoundID
			}
			if tc.field == "number" {
				e = v.Number
			}
			if e.State != validation.EvidenceInvalid || e.Cause == nil {
				t.Errorf("invalid field lost causal state: %+v", e)
			}
		})
	}
}

func TestAuxiliarySoundsBrokenGrammarRetainsKnownSound(t *testing.T) {
	r := inspectSounds(t, `<sounds><sound id="a" primary="yes"><any/><solo/></sound></sounds>`)
	rows := soundMappings(t, r)
	if r.Conformance != validation.ConformanceViolated || len(rows) == 0 {
		t.Fatalf("grammar state: %+v", r)
	}
	v := rows[0].SoundMapping
	if v.Role != validation.SoundMappingSound || v.Primary.State != validation.EvidenceKnown || v.Primary.Value.Text != "yes" || v.Content.State != validation.EvidenceInvalid || v.Content.Cause == nil {
		t.Fatalf("independent field/grammar causes: %+v", v)
	}
}

func TestAuxiliarySoundsAssemblyAndSourceOnly(t *testing.T) {
	sounds, err := compileValidationSchema(validation.AssemblySounds, soundsValidationSchema)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := validation.NewSoundsMappingContextPlan(sounds)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range []struct {
		assembly validation.Assembly
		data     validationSchemaSet
	}{{validation.AssemblyMusicXML, scoreValidationSchema}, {validation.AssemblyOpus, opusValidationSchema}, {validation.AssemblyContainer, containerValidationSchema}} {
		schema, err := compileValidationSchema(entry.assembly, entry.data)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = validation.NewSoundsMappingContextPlan(schema); !errors.Is(err, validation.ErrInvalidConfiguration) {
			t.Fatalf("wrong assembly accepted %s: %v", entry.assembly, err)
		}
	}
	if _, err = validation.NewLocalContextPlan(sounds); !errors.Is(err, validation.ErrInvalidConfiguration) {
		t.Fatal("score plan accepted sounds")
	}
	for _, bad := range []string{"target", "assembly", "role"} {
		a := validation.AssemblySounds
		rule := validation.RuleDefinition{Key: validation.RuleKey{ID: "test-sounds", Revision: 1}, Class: validation.ClassMandatory, Role: validation.RoleContextProvider, Category: validation.CategoryNormative, Targets: []validation.Target{validation.TargetSource}, Scope: validation.ScopeDocument, Selector: validation.Selector{Assembly: &a}, Predicate: "test-sounds", Text: validation.RuleText{Title: "test", Explanation: "test", Message: "test"}}
		switch bad {
		case "target":
			rule.Targets = []validation.Target{validation.TargetModel}
		case "assembly":
			a = validation.AssemblyMusicXML
		case "role":
			rule.Role = validation.RolePredicate
		}
		if _, err = validation.NewCatalog([]validation.RuleDefinition{rule}, nil, []validation.PlanBinding{{ID: rule.Predicate, Plan: plan}}); !errors.Is(err, validation.ErrInvalidConfiguration) {
			t.Fatalf("bad %s accepted: %v", bad, err)
		}
	}
	for _, profile := range []validation.ProfileID{ProfileSourceContainer, ProfileSourceSounds} {
		if _, err = AssessModel(context.Background(), NewOpusDocument(), AssessmentOptions{Profile: profile}); !errors.Is(err, validation.ErrInvalidConfiguration) {
			t.Fatalf("source-only profile model accepted: %v", err)
		}
	}
	for _, source := range []string{`<sounds><sound id="a"/></sounds>`, `<container><rootfiles><rootfile full-path="a"/></rootfiles></container>`} {
		if _, err = Decode(strings.NewReader(source)); !errors.Is(err, ErrUnsupportedRoot) {
			t.Fatalf("new Document root introduced: %v", err)
		}
	}
}

func TestAuxiliarySourceUnavailableEvidence(t *testing.T) {
	for _, tc := range []struct {
		profile validation.ProfileID
		source  string
	}{{ProfileSourceContainer, `<!DOCTYPE container SYSTEM "https://invalid.example/no-fetch"><container><rootfiles><rootfile full-path="a"/></rootfiles></container>`}, {ProfileSourceSounds, `<!DOCTYPE sounds SYSTEM "https://invalid.example/no-fetch"><sounds><sound id="a"/></sounds>`}} {
		r, err := InspectXML(context.Background(), strings.NewReader(tc.source), AssessmentOptions{Profile: tc.profile})
		if err != nil {
			t.Fatal(err)
		}
		if r.Satisfied() || r.Conformance == validation.ConformanceViolated {
			t.Fatalf("unread DTD fabricated invalidity or acceptance: %+v", r)
		}
	}
}

type auxiliaryOwnedReader struct {
	io.Reader
	closed bool
}

func (r *auxiliaryOwnedReader) Close() error { r.closed = true; return nil }
func TestAuxiliarySourceOwnershipAndBudgets(t *testing.T) {
	source := `<sounds><sound id="a" primary="maybe"><any primary="maybe">value</any></sound></sounds>`
	first, second := inspectSounds(t, source), inspectSounds(t, source)
	rows := soundMappings(t, first)
	rows[0].SoundMapping.SoundID.Value.Text = "caller"
	rows[0].SoundMapping.Sound.DocumentID = "caller"
	rows[0].SoundMapping.Primary.Cause.Rule.ID = "caller"
	rows[0].Origins[0].Path = "caller"
	rows[0].Origins[0].Position.Line = 999
	if !reflect.DeepEqual(second.Context, inspectSounds(t, source).Context) {
		t.Fatal("caller mutated later report")
	}
	if rows[1].SoundMapping.Sound.DocumentID == "caller" || rows[1].SoundMapping.Primary.Cause.Rule.ID == "caller" {
		t.Fatal("mapping payloads alias")
	}
	for _, profile := range []validation.ProfileID{ProfileSourceContainer, ProfileSourceSounds} {
		text := `<container><rootfiles><rootfile full-path="a"/></rootfiles></container>`
		if profile == ProfileSourceSounds {
			text = `<sounds><sound id="a"><any/><any/></sound></sounds>`
		}
		r := &auxiliaryOwnedReader{Reader: strings.NewReader(text)}
		report, err := InspectXML(context.Background(), r, AssessmentOptions{Profile: profile, Budgets: validation.Budgets{MaxInputBytes: int64(len(text))}})
		if err != nil || !report.Satisfied() || r.closed {
			t.Fatalf("exact bytes/reader ownership: %v", err)
		}
		report, err = InspectXML(context.Background(), strings.NewReader(text), AssessmentOptions{Profile: profile, Budgets: validation.Budgets{MaxInputBytes: int64(len(text) - 1)}})
		if !errors.Is(err, validation.ErrResourceLimit) || report.Satisfied() {
			t.Fatalf("one-under input accepted: %v", err)
		}
		for _, b := range []validation.Budgets{{MaxRuleSteps: 1}, {MaxBufferedBytes: 1}, {MaxInputBytes: -1}} {
			report, err = InspectXML(context.Background(), strings.NewReader(text), AssessmentOptions{Profile: profile, Budgets: b})
			if err == nil || report.Satisfied() {
				t.Fatalf("invalid/limited budgets accepted: %+v", b)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err = InspectXML(ctx, strings.NewReader(text), AssessmentOptions{Profile: profile}); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation lost: %v", err)
		}
	}
	text := `<sounds><sound id="a"><any/><any/></sound></sounds>`
	for _, limit := range []int64{3, 4} {
		r, err := InspectXML(context.Background(), strings.NewReader(text), AssessmentOptions{Profile: ProfileSourceSounds, Budgets: validation.Budgets{MaxContextFacts: limit}})
		if limit == 4 {
			if err != nil || !r.Satisfied() || len(r.Context) != 4 {
				t.Fatalf("exact context budget: %v %+v", err, r)
			}
		} else if !errors.Is(err, validation.ErrResourceLimit) || r.Satisfied() || len(r.Context) > int(limit) {
			t.Fatalf("one-under context budget: %v %+v", err, r)
		}
	}
}

func TestAuxiliaryFrozenEightProfiles(t *testing.T) {
	data, err := os.ReadFile("testdata/validation/frozen-eight-profiles-batch8.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Profile   validation.Profile
		Report    validation.Report
		Error     string
		Satisfied bool
	}
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 8 {
		t.Fatalf("frozen profile count=%d", len(rows))
	}
	var rawRows []map[string]json.RawMessage
	if err = json.Unmarshal(data, &rawRows); err != nil {
		t.Fatal(err)
	}
	doc, err := Decode(strings.NewReader(`<opus version="4.0"/>`))
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range rows {
		var got validation.Report
		if row.Profile.Target == validation.TargetSource {
			got, err = InspectXML(context.Background(), strings.NewReader(`<opus version="4.0"/>`), AssessmentOptions{Profile: row.Profile.ID})
		} else {
			got, err = AssessModel(context.Background(), doc, AssessmentOptions{Profile: row.Profile.ID})
		}
		if err != nil {
			t.Fatal(err)
		}
		// Compare the original captured JSON shape as well as values. New optional
		// payloads must not add null fields to any old profile report.
		actual, _ := json.Marshal(got)
		var actualValue, expectedValue any
		if err = json.Unmarshal(actual, &actualValue); err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(rawRows[i]["report"], &expectedValue); err != nil {
			t.Fatal(err)
		}
		// Keep the original snapshot unchanged. Only the documented withdrawal
		// of the figure-number overconstraint may migrate context reports.
		migrateFrozenFigureNumberReport(t, row.Profile.ID, expectedValue)
		if !reflect.DeepEqual(actualValue, expectedValue) {
			t.Errorf("profile report changed beyond its documented migration: %s\n got %s\nhistorical %s", row.Profile.ID, actual, rawRows[i]["report"])
		}
	}
}

func TestAuxiliaryDiagnosticsCannotEraseViolation(t *testing.T) {
	for _, tc := range []struct {
		profile validation.ProfileID
		source  string
	}{{ProfileSourceContainer, `<container extra="x"><rootfiles><rootfile/></rootfiles></container>`}, {ProfileSourceSounds, `<sounds><sound id="a" primary="maybe"/><sound id="a" primary="maybe"/></sounds>`}} {
		r, err := InspectXML(context.Background(), strings.NewReader(tc.source), AssessmentOptions{Profile: tc.profile, Budgets: validation.Budgets{MaxDiagnosticBytes: 1}})
		if err != nil || !r.DiagnosticsTruncated || r.Counts.Fail == 0 || r.Satisfied() || r.Conformance != validation.ConformanceViolated {
			t.Fatalf("truncated details erased normative failure: %v %+v", err, r)
		}
	}
}

type auxiliaryCancelReader struct {
	reader *strings.Reader
	cancel context.CancelFunc
	read   int
}

func (r *auxiliaryCancelReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	n, err := r.reader.Read(p)
	r.read += n
	if r.read == 40 {
		r.cancel()
	}
	return n, err
}
func TestAuxiliaryCancellationDuringParseReleasesOwnership(t *testing.T) {
	for _, tc := range []struct {
		profile validation.ProfileID
		source  string
	}{{ProfileSourceContainer, `<container><rootfiles><rootfile full-path="a"/></rootfiles></container>`}, {ProfileSourceSounds, `<sounds><sound id="a"><ensemble number="many">value</ensemble></sound></sounds>`}} {
		ctx, cancel := context.WithCancel(context.Background())
		reader := &auxiliaryCancelReader{reader: strings.NewReader(tc.source), cancel: cancel}
		run, err := newSourceAssessmentRun(ctx, reader, DecodeOptions{}, &AssessmentOptions{Profile: tc.profile}, false)
		if err != nil {
			cancel()
			t.Fatal(err)
		}
		parseErr := run.drain()
		r, finishErr := run.finish(parseErr)
		cancel()
		if !errors.Is(errors.Join(parseErr, finishErr), context.Canceled) || r.Satisfied() {
			t.Fatalf("mid-parse cancellation: %v %v %+v", parseErr, finishErr, r)
		}
		for _, kind := range []validation.BudgetKind{validation.BudgetBufferedBytes, validation.BudgetDepth} {
			used, err := run.account.Usage(kind)
			if err != nil || used != 0 {
				t.Errorf("%s cleanup retained %d: %v", kind, used, err)
			}
		}
	}
}

func FuzzAuxiliarySourceAssessment(f *testing.F) {
	for _, tc := range auxiliarySourceCases() {
		f.Add(tc.source, tc.assembly == "sounds")
	}
	f.Fuzz(func(t *testing.T, source string, sounds bool) {
		if len(source) > 65536 {
			t.Skip()
		}
		profile := ProfileSourceContainer
		if sounds {
			profile = ProfileSourceSounds
		}
		r, err := InspectXML(context.Background(), strings.NewReader(source), AssessmentOptions{Profile: profile, Budgets: validation.Budgets{MaxInputBytes: 65536, MaxBufferedBytes: 4 << 20, MaxRuleSteps: 1_000_000, MaxNodes: 2048, MaxContextFacts: 2048, MaxDiagnostics: 16}})
		if err != nil && r.Satisfied() {
			t.Fatal("operation error accepted")
		}
		for _, row := range r.Context {
			if row.Property == validation.ContextPropertySoundMapping && (row.SoundMapping == nil || !sounds) {
				t.Fatal("invalid auxiliary projection")
			}
		}
	})
}
