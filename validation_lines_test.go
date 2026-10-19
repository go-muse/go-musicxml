package musicxml

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// publicProfiles parses every exported profile constant of the root package
// sources, so that a new public profile cannot escape the registry checks,
// and returns each profile with the rules it requires.
func publicProfiles(t *testing.T) map[validation.ProfileID]validation.Profile {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	packages := []func() (*validation.CompiledPackageCatalog, error){packageAssessmentCatalog, dictionaryPackageAssessmentCatalog, dtdDictionaryPackageAssessmentCatalog, semanticPackageAssessmentCatalog}
	out := make(map[validation.ProfileID]validation.Profile)
	declared := 0
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value := spec.(*ast.ValueSpec)
				selector, typed := value.Type.(*ast.SelectorExpr)
				typed = typed && selector.Sel.Name == "ProfileID"
				for i, ident := range value.Names {
					if !typed && !strings.HasPrefix(ident.Name, "Profile") {
						continue
					}
					// Every profile constant has the one form this test reads.
					var literal *ast.BasicLit
					if i < len(value.Values) {
						literal, _ = value.Values[i].(*ast.BasicLit)
					}
					if !typed || !ident.IsExported() || literal == nil || literal.Kind != token.STRING {
						t.Fatalf("%s: %s must be an exported validation.ProfileID string constant", name, ident.Name)
					}
					text, err := strconv.Unquote(literal.Value)
					if err != nil {
						t.Fatal(err)
					}
					id := validation.ProfileID(text)
					declared++
					var profile validation.Profile
					found := false
					if e, err := sourceAssessmentEngineForProfile(id); err == nil {
						profile, found = e.Profile(id)
					} else if e, err := modelAssessmentEngineForProfile(id); err == nil {
						profile, found = e.Profile(id)
					} else {
						for _, build := range packages {
							c, err := build()
							if err != nil {
								t.Fatal(err)
							}
							if profile, found = c.Profile(id); found {
								break
							}
						}
					}
					if !found {
						t.Fatalf("%s (%s) resolves to no engine or package catalog", ident.Name, id)
					}
					out[id] = profile
				}
			}
		}
	}
	if declared != len(out) || len(out) < 37 {
		t.Fatalf("found %d public profiles in %d declarations", len(out), declared)
	}
	return out
}

func requiredIDs(profiles ...validation.Profile) map[validation.RuleID]bool {
	out := make(map[validation.RuleID]bool)
	for _, p := range profiles {
		for _, key := range p.Required {
			out[key.ID] = true
		}
	}
	return out
}

// Every rule revision of every public profile has exactly one line, and every
// registry row names a rule that some public profile requires.
func TestEveryRuleHasAValidationLine(t *testing.T) {
	required := make(map[validation.RuleKey]bool)
	for id, p := range publicProfiles(t) {
		for _, key := range p.Required {
			required[key] = true
			if _, ok := RuleLine(key); !ok {
				t.Errorf("%s requires %s@%d, which has no validation line", id, key.ID, key.Revision)
			}
		}
	}
	seen := make(map[validation.RuleKey]bool)
	for _, g := range ruleLineGroups {
		if g.line != LineSource && g.line != LineModel || g.reason == "" || len(g.rules) == 0 {
			t.Errorf("malformed registry group %v %q", g.line, g.reason)
		}
		for _, key := range g.rules {
			if seen[key] {
				t.Errorf("%s@%d has two registry rows", key.ID, key.Revision)
			}
			seen[key] = true
			if !required[key] {
				t.Errorf("registry row %s@%d names no rule of a public profile", key.ID, key.Revision)
			}
		}
	}
	if len(seen) != len(required) {
		t.Errorf("registry has %d rows for %d rules", len(seen), len(required))
	}
	if _, ok := RuleLine(validation.RuleKey{ID: "MX40-NOT-A-RULE", Revision: 1}); ok {
		t.Error("an unknown rule has a validation line")
	}
}

// lineIntrinsic returns the profiles that a semantic package profile binds
// for its resources, split into model-target rows and source-target rows.
func lineIntrinsic(t *testing.T, id validation.ProfileID) (model, source []validation.ProfileID) {
	t.Helper()
	_, packages, err := semanticPackageAssessmentDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range packages {
		if binding.Profile.ID != id {
			continue
		}
		for _, row := range binding.Intrinsic {
			if row.Target == validation.TargetModel {
				model = append(model, row.Profile)
			} else {
				source = append(source, row.Profile)
			}
		}
		return model, source
	}
	t.Fatalf("%s is not a semantic package profile", id)
	return nil, nil
}

// lineClosures returns, per line, the line profiles with every profile they
// bind for package resources. retained lists the source profiles that line 3
// binds for the files a converted MXLPackage keeps as bytes.
func lineClosures(t *testing.T, profiles map[validation.ProfileID]validation.Profile) (source, model, retained []validation.Profile) {
	t.Helper()
	source = []validation.Profile{profiles[ProfileSourceLine], profiles[ProfileMXLSourceLine]}
	sourceModelRows, sourceRows := lineIntrinsic(t, ProfileMXLSourceLine)
	if len(sourceModelRows) != 0 {
		t.Fatalf("line 2 binds model profiles %v", sourceModelRows)
	}
	for _, id := range sourceRows {
		source = append(source, profiles[id])
	}
	model = []validation.Profile{profiles[ProfileModelSemantics], profiles[ProfileMXLModelSemantics]}
	modelRows, retainedRows := lineIntrinsic(t, ProfileMXLModelSemantics)
	for _, id := range modelRows {
		model = append(model, profiles[id])
	}
	for _, id := range retainedRows {
		retained = append(retained, profiles[id])
	}
	return source, model, retained
}

// The line profiles, with the profiles they bind for package resources, hold
// only their own line's rules. Line 3 also assesses the files a converted
// package keeps as bytes with source profiles; those rules may belong to
// line 2.
func TestLineProfilesOwnTheirRules(t *testing.T) {
	source, model, _ := lineClosures(t, publicProfiles(t))
	for _, tc := range []struct {
		line     ValidationLine
		profiles []validation.Profile
	}{{LineSource, source}, {LineModel, model}} {
		for _, p := range tc.profiles {
			if len(p.Required) == 0 {
				t.Fatalf("%s requires no rules", p.ID)
			}
			for _, key := range p.Required {
				line, _ := RuleLine(key)
				if line == tc.line {
					continue
				}
				// Retained-file overlap is accepted only through the rows
				// that assess retained files, which this test leaves out.
				if reason, allowed := allowedLineOverlap[key.ID]; tc.line == LineModel && line == LineSource && allowed && reason != overlapRetained {
					continue
				}
				t.Errorf("%s (line %d) requires %s@%d of line %d", p.ID, tc.line, key.ID, key.Revision, line)
			}
		}
	}
}

// Line 3 binds exactly these source profiles for the files a converted
// package keeps as bytes.
func TestRetainedFilesUseSourceProfiles(t *testing.T) {
	_, retained := lineIntrinsic(t, ProfileMXLModelSemantics)
	sort.Slice(retained, func(i, j int) bool { return retained[i] < retained[j] })
	want := []validation.ProfileID{ProfileSourceInternalDTDContainer, ProfileSourceSemantics, ProfileSourceSemantics, ProfileSourceInternalDTDSoundsDictionaries}
	sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
	if fmt.Sprint(retained) != fmt.Sprint(want) {
		t.Errorf("retained-file profiles %v, want %v", retained, want)
	}
}

// Lines 2 and 3 check the same rule only where AllowedLineOverlap says so,
// and every accepted overlap is real.
func TestLinesOverlapOnlyWhereAllowed(t *testing.T) {
	sourceProfiles, modelProfiles, retained := lineClosures(t, publicProfiles(t))
	source := requiredIDs(sourceProfiles...)
	model := requiredIDs(append(modelProfiles, retained...)...)
	var overlap, missing []string
	for id := range source {
		if model[id] {
			if _, ok := allowedLineOverlap[id]; !ok {
				overlap = append(overlap, string(id))
			}
		}
	}
	for id, reason := range AllowedLineOverlap() {
		if !source[id] || !model[id] || reason == "" {
			missing = append(missing, string(id))
		}
	}
	sort.Strings(overlap)
	sort.Strings(missing)
	if len(overlap) != 0 {
		t.Errorf("lines 2 and 3 both check %v", overlap)
	}
	if len(missing) != 0 {
		t.Errorf("accepted overlap that is not an overlap: %v", missing)
	}
}

func TestValidationLineString(t *testing.T) {
	for line, want := range map[ValidationLine]string{LineDecoder: "decoder", LineSource: "source", LineModel: "model", 0: "unknown", 4: "unknown"} {
		if got := line.String(); got != want {
			t.Errorf("%d: %q, want %q", line, got, want)
		}
	}
}

// The line profiles are new identities; a change to their rules, plans or
// closure must change these pins deliberately.
func TestLineProfilesExactIdentity(t *testing.T) {
	profiles := publicProfiles(t)
	for id, want := range map[validation.ProfileID][2]string{
		ProfileSourceLine:    {"7be7564fac3908064f43eeda11d0b93023311db358110234baf8adb8467f1408", "04fb11d56a41d043451031b6133aff7afce75e187612a7330ae365bbeb484b23"},
		ProfileMXLSourceLine: {"0bd27f2c2e49941f1311d624bd0ca4d0e65f5891c72513f854c523f811d89893", "8c9b33ac1c9118dbe18dc97990858d8c9077eb9cfc7818f193fdf21754db595c"},
	} {
		p := profiles[id]
		raw, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(raw)); p.CatalogDigest != want[0] || got != want[1] {
			t.Errorf("%s identity catalog=%s profile=%s, want %s %s", id, p.CatalogDigest, got, want[0], want[1])
		}
	}
}

// Splitting the lines brings strict loading of ordinary test-suite scores
// within the default budgets; both files exceeded the default rule steps when
// strict loading ran the full source semantics.
func TestStrictLoadingFitsDefaultBudgets(t *testing.T) {
	for _, name := range []string{"01a-Pitches-Pitches.xml", "41c-StaffGroups.xml"} {
		data, err := os.ReadFile(filepath.Join("testdata", "corpus", "musicxml-test-suite", name))
		if err != nil {
			t.Fatal(err)
		}
		result, err := DecodeAssessed(context.Background(), bytes.NewReader(data), DecodeAssessmentOptions{})
		if err != nil || !result.Accepted || result.SourceLine.Profile.ID != ProfileSourceLine || result.ModelLine.Profile.ID != ProfileModelSemantics {
			t.Errorf("%s: accepted=%v error=%v", name, result.Accepted, err)
		}
	}
}

// Line 2 of strict MXL loading assesses only what conversion consumes, the
// container and the first rootfile; a later rootfile or a linked file that
// the converted package keeps as bytes is line 3's, and the representability
// limit of the converted model does not apply to it.
func TestStrictMXLLoadingLeavesRetainedFilesToLineThree(t *testing.T) {
	unrepresentable := representabilityScore("", representabilityNote("1.00000000000000000001", ""))
	for name, entries := range map[string][][2]string{
		"linked": {
			{"META-INF/container.xml", `<container><rootfiles><rootfile full-path="root.opus" media-type="application/vnd.recordare.musicxml+xml"/></rootfiles></container>`},
			{"root.opus", `<opus xmlns:xlink="http://www.w3.org/1999/xlink" version="4.0"><title>T</title><score xlink:href="score.musicxml"/></opus>`},
			{"score.musicxml", unrepresentable},
		},
		"later rootfile": {
			{"META-INF/container.xml", `<container><rootfiles><rootfile full-path="score.musicxml" media-type="application/vnd.recordare.musicxml+xml"/><rootfile full-path="later.musicxml" media-type="application/vnd.recordare.musicxml+xml"/></rootfiles></container>`},
			{"score.musicxml", representabilityScore("", representabilityNote("1", ""))},
			{"later.musicxml", unrepresentable},
		},
	} {
		t.Run(name, func(t *testing.T) {
			var archive bytes.Buffer
			writer := zip.NewWriter(&archive)
			for _, entry := range entries {
				w, err := writer.Create(entry[0])
				if err != nil {
					t.Fatal(err)
				}
				if _, err = io.WriteString(w, entry[1]); err != nil {
					t.Fatal(err)
				}
			}
			if err := writer.Close(); err != nil {
				t.Fatal(err)
			}
			result, err := DecodeMXLAssessed(context.Background(), bytes.NewReader(archive.Bytes()), DecodeMXLAssessmentOptions{Lenient: true})
			if err != nil {
				t.Fatal(err)
			}
			// Each line's bundle lists the documents it assessed.
			documents := func(b validation.AssessmentBundle) (out []string) {
				for i := 0; i < b.ResourceCount(); i++ {
					if _, report, ok := b.ResourceReport(i); ok {
						out = append(out, string(report.Profile.ID))
						if hasRepresentabilityFailure(report) {
							t.Errorf("%s rejects the retained value as unrepresentable", report.Profile.ID)
						}
					}
				}
				sort.Strings(out)
				return out
			}
			source, model := documents(result.SourceLine), documents(result.ModelLine)
			// Line 3 may stay incomplete for unrelated package roles; it finds
			// no violation in the retained file.
			if !result.SourceLine.Satisfied() || result.ModelLine.Failures().Violated || strings.Join(source, ",") != string(ProfileSourceInternalDTDContainer)+","+string(ProfileSourceLine) || strings.Join(model, ",") != string(ProfileModelSemantics)+","+string(ProfileSourceSemantics) {
				t.Fatalf("source line documents %v, model line documents %v; source=%+v model=%+v", source, model, result.SourceLine.Failures(), result.ModelLine.Failures())
			}
		})
	}
}

// Splitting the lines loses no finding: every rule that the full source
// semantics fails on a document also fails on line 2 or line 3 of strict
// loading.
func TestLinesKeepEverySourceFinding(t *testing.T) {
	inputs := map[string]string{
		"xsd":       mandatoryScore("0", ""),
		"local":     mandatoryScore("1", `<direction><direction-type><accordion-registration/></direction-type></direction>`),
		"timeline":  mandatoryScore("1", `<harmony><numeral><numeral-root>1</numeral-root></numeral><kind>major</kind></harmony>`),
		"local+xsd": mandatoryScore("0", `<direction><direction-type><accordion-registration/></direction-type></direction>`),
	}
	for _, name := range []string{"41g-PartNoId.xml", "41h-TooManyParts.xml", "74a-FiguredBass.xml", "99d-AccordionInvalid.xml"} {
		data, err := os.ReadFile(filepath.Join("testdata", "corpus", "musicxml-test-suite", name))
		if err != nil {
			t.Fatal(err)
		}
		inputs[name] = string(data)
	}
	failed := func(into map[validation.RuleID]bool, r validation.Report) {
		for _, f := range r.Findings {
			if f.Outcome == validation.OutcomeFail {
				into[f.Rule.ID] = true
			}
		}
	}
	for name, input := range inputs {
		source, err := InspectXML(context.Background(), strings.NewReader(input), AssessmentOptions{})
		if err != nil {
			t.Fatal(err)
		}
		want := map[validation.RuleID]bool{}
		failed(want, source)
		if len(want) == 0 {
			t.Fatalf("%s: fixture has no source finding", name)
		}
		// A document the model cannot hold fails conversion, an operational
		// error even in the lenient mode; its findings must then be line 2's.
		result, err := DecodeAssessed(context.Background(), strings.NewReader(input), DecodeAssessmentOptions{Lenient: true})
		var conversion *ConversionError
		if err != nil && !errors.As(err, &conversion) {
			t.Fatalf("%s: %v", name, err)
		}
		got := map[validation.RuleID]bool{}
		failed(got, result.SourceLine)
		failed(got, result.ModelLine)
		for id := range want {
			if !got[id] {
				t.Errorf("%s: %s fails on the full source semantics but on neither line", name, id)
			}
		}
	}
}

// Strict loading gives line 2 no dictionaries and line 3 the imported set,
// on one account.
func TestAssessedLoadingLinesConfiguration(t *testing.T) {
	normalized, err := assessedLoadingOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	source, model, err := assessedLoadingLines(context.Background(), normalized)
	if err != nil {
		t.Fatal(err)
	}
	defer model.Dictionaries.Close()
	if source.Profile != ProfileSourceLine || source.Dictionaries != nil || source.DocumentID != sourceDocumentID {
		t.Errorf("line 2 configuration: profile=%s dictionaries=%v document=%s", source.Profile, source.Dictionaries != nil, source.DocumentID)
	}
	if model.Profile != ProfileModelSemantics || model.Dictionaries == nil || model.DocumentID != modelDocumentID || model.Account != source.Account {
		t.Errorf("line 3 configuration: profile=%s dictionaries=%v document=%s shared account=%v", model.Profile, model.Dictionaries != nil, model.DocumentID, model.Account == source.Account)
	}
}

// In strict MXL loading only line 3 imports dictionaries: a provider failure
// leaves line 2 satisfied, lets line 3 run and report the unavailable
// evidence, and still rejects the package in both modes.
func TestStrictMXLLoadingDictionaryFailureReachesLineThree(t *testing.T) {
	offline := errors.New("dictionary store offline")
	var calls int
	source := DictionarySourceFunc(func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
		calls++
		return DictionaryComponentResult{}, offline
	})
	archive := rawMXL(t, mandatoryDictionaryScore())
	for _, lenient := range []bool{false, true} {
		calls = 0
		required := requiredDictionarySelections()
		result, err := DecodeMXLAssessed(context.Background(), bytes.NewReader(archive), DecodeMXLAssessmentOptions{
			Assessment: MXLAssessmentOptions{Graph: AssessmentOptions{Dictionaries: &DictionaryOptions{Source: source, Selections: required}}},
			Lenient:    lenient,
		})
		if !errors.Is(err, offline) || result.Package != nil || result.Accepted {
			t.Fatalf("lenient=%v: error=%v package=%v accepted=%v", lenient, err, result.Package != nil, result.Accepted)
		}
		if !result.SourceLine.Satisfied() || !result.ModelLine.PackageReport().Requested || result.ModelLine.Satisfied() {
			t.Fatalf("lenient=%v: source satisfied=%v model requested=%v model satisfied=%v", lenient, result.SourceLine.Satisfied(), result.ModelLine.PackageReport().Requested, result.ModelLine.Satisfied())
		}
		if calls != len(required) {
			t.Errorf("lenient=%v: %d provider calls, want one per selected component in line 3", lenient, calls)
		}
	}
}
