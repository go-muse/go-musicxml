package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

const semanticQALocalKey = `<numeral-key><numeral-fifths>0</numeral-fifths><numeral-mode>major</numeral-mode></numeral-key>`
const semanticQADivisions = `<attributes><divisions>1</divisions></attributes>`

// These cases deliberately use ordinary integer inputs, for which source and
// current-model values coincide. They do not assert decimal/binary identity.
func semanticQAScore(form string, measures [][2]string) string {
	const parts = `<part-list><score-part id="P1"><part-name>First</part-name></score-part><score-part id="P2"><part-name>Second</part-name></score-part></part-list>`
	var b strings.Builder
	b.WriteString(`<score-` + form + ` version="4.0">` + parts)
	if form == "partwise" {
		for p, id := range []string{"P1", "P2"} {
			b.WriteString(`<part id="` + id + `">`)
			for _, m := range measures {
				b.WriteString(`<measure number="same-label">` + m[p] + `</measure>`)
			}
			b.WriteString(`</part>`)
		}
	} else {
		for _, m := range measures {
			b.WriteString(`<measure number="same-label">`)
			for p, id := range []string{"P1", "P2"} {
				b.WriteString(`<part id="` + id + `">` + m[p] + `</part>`)
			}
			b.WriteString(`</measure>`)
		}
	}
	b.WriteString(`</score-` + form + `>`)
	return b.String()
}

func semanticQAFindings(r validation.Report, id string) []validation.Finding {
	var out []validation.Finding
	for _, f := range r.Findings {
		if string(f.Rule.ID) == id {
			out = append(out, f)
		}
	}
	return out
}

func semanticQAExpect(t *testing.T, r validation.Report, id string, want validation.Outcome, count int) {
	t.Helper()
	found := semanticQAFindings(r, id)
	if len(found) != count {
		t.Fatalf("%s findings=%d want %d; %s", id, len(found), count, localFindings(r))
	}
	for _, f := range found {
		if f.Outcome != want {
			t.Errorf("%s got %+v want %s", id, f, want)
		}
		if want == validation.OutcomeFail && f.Cause != nil {
			t.Errorf("proven local consequence acquired an unrelated cause: %+v", f)
		}
	}
}

func TestTimelineSemanticQAHierarchyParityAndPartIsolation(t *testing.T) {
	key := `<attributes><divisions>1</divisions><key><fifths>0</fifths></key></attributes>`
	for _, form := range []string{"partwise", "timewise"} {
		t.Run(form, func(t *testing.T) {
			// Repeated measure labels are distinct occurrences. P1's key must carry
			// across repeated timewise IDREFs; P2 must never inherit that key.
			source := semanticQAScore(form, [][2]string{
				{key + timelineNote("1", "", ""), semanticQADivisions + timelineNote("1", "", "")},
				{timelineNumeral("", ""), timelineNumeral("", "")},
				{timelineNumeral("", ""), timelineNumeral(semanticQALocalKey, "")},
			})
			assessTimelineBoth(t, source, func(t *testing.T, r validation.Report) {
				semanticQAExpect(t, r, "MX40-TIMELINE-numeral-requires-key", validation.OutcomeFail, 1)
				if !r.AssessmentComplete || r.Conformance != validation.ConformanceViolated {
					t.Errorf("closed missing key should be a complete violation: %+v", r)
				}
				for _, f := range r.Findings {
					if f.Rule.ID != "MX40-TIMELINE-numeral-requires-key" {
						t.Errorf("unintended finding: %+v", f)
					}
				}
			})
		})
	}
}

func TestTimelineSemanticQANumeralMusicalOrderAndScopes(t *testing.T) {
	note := timelineNote("2", "", "")
	key := `<attributes><key><fifths>0</fifths></key></attributes>`
	for _, tc := range []struct {
		name, body string
		want       validation.Outcome
		count      int
	}{
		{"key-lexically-later-but-musically-earlier", semanticQADivisions + note + timelineNumeral("", "") + note + `<backup><duration>3</duration></backup>` + key, "", 0},
		{"key-lexically-earlier-but-musically-later", semanticQADivisions + note + key + note + `<backup><duration>3</duration></backup>` + timelineNumeral("", ""), validation.OutcomeFail, 1},
		{"same-position-later-key", timelineNumeral("", "") + key, "", 0},
		{"invisible-key-still-encoded", `<attributes><key print-object="no"><fifths>0</fifths></key></attributes>` + timelineNumeral("", ""), "", 0},
		{"empty-nontraditional-key-still-encoded", `<attributes><key/></attributes>` + timelineNumeral("", ""), "", 0},
		{"nontraditional-key-still-encoded", `<attributes><key><key-step>F</key-step><key-alter>1</key-alter></key></attributes>` + timelineNumeral("", ""), "", 0},
		{"staff-two-does-not-borrow-staff-one", `<attributes><key number="1"><fifths>0</fifths></key><staves>2</staves></attributes>` + timelineNumeral("", `<staff>2</staff>`), validation.OutcomeFail, 1},
		{"omitted-multistaff-assignment-remains-unknown", `<attributes><key number="1"><fifths>0</fifths></key><staves>2</staves></attributes>` + timelineNumeral("", ""), validation.OutcomeUnknown, 1},
		{"local-key-independent-of-omitted-multistaff-assignment", `<attributes><key number="1"><fifths>0</fifths></key><staves>2</staves></attributes>` + timelineNumeral(semanticQALocalKey, ""), "", 0},
		{"local-key-does-not-leak-to-next-numeral", timelineNumeral(semanticQALocalKey, "") + timelineNumeral("", ""), validation.OutcomeFail, 1},
		{"same-position-conflicting-content-proves-existence", `<attributes><key><fifths>0</fifths></key></attributes><attributes><key><fifths>3</fifths></key></attributes>` + timelineNumeral("", ""), "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assessTimelineBoth(t, (localFixture{body: tc.body}).xml(), func(t *testing.T, r validation.Report) {
				semanticQAExpect(t, r, "MX40-TIMELINE-numeral-requires-key", tc.want, tc.count)
				if tc.count == 0 && !r.Satisfied() {
					t.Fatalf("positive case rejected: %s", localFindings(r))
				}
				if tc.want == validation.OutcomeUnknown && r.AssessmentComplete {
					t.Fatalf("unknown staff was called complete")
				}
				for _, f := range r.Findings {
					if strings.HasPrefix(string(f.Rule.ID), "MX40-XSD-") && f.Outcome == validation.OutcomeFail {
						t.Errorf("fixture schema failure: %+v", f)
					}
				}
			})
		})
	}
}

func TestTimelineSemanticQAChordPrincipalRelations(t *testing.T) {
	grace := `<note><grace/><pitch><step>C</step><octave>4</octave></pitch></note>`
	graceChord := `<note><grace/><chord/><pitch><step>E</step><octave>4</octave></pitch></note>`
	for _, tc := range []struct {
		name, body string
		want       validation.Outcome
		count      int
	}{
		{"four-one-three", timelineNote("4", "", "") + timelineNote("1", `<chord/>`, "") + timelineNote("3", `<chord/>`, ""), "", 0},
		{"four-one-five", timelineNote("4", "", "") + timelineNote("1", `<chord/>`, "") + timelineNote("5", `<chord/>`, ""), validation.OutcomeFail, 1},
		{"cross-staff-mixed-voice", `<attributes><staves>2</staves></attributes>` + timelineNote("4", "", `<voice>upper</voice><staff>2</staff>`) + timelineNote("3", `<chord/>`, `<voice>lower</voice><staff>1</staff>`), "", 0},
		{"nearest-principal-not-last-same-voice", timelineNote("4", "", `<voice>A</voice>`) + timelineNote("1", "", `<voice>B</voice>`) + timelineNote("2", `<chord/>`, `<voice>A</voice>`), validation.OutcomeFail, 1},
		{"cue-principal-and-cue-chord", timelineNote("2", `<cue/>`, "") + timelineNote("1", `<cue/><chord/>`, ""), "", 0},
		{"cue-principal-and-normal-chord", timelineNote("2", `<cue/>`, "") + timelineNote("1", `<chord/>`, ""), "", 0},
		{"normal-principal-and-cue-chord", timelineNote("2", "", "") + timelineNote("1", `<cue/><chord/>`, ""), "", 0},
		{"grace-group-does-not-invent-zero", grace + graceChord + timelineNote("1", "", ""), "", 0},
		{"grace-principal-does-not-borrow-older-duration", timelineNote("4", "", "") + grace + timelineNote("1", `<chord/>`, ""), validation.OutcomeUnknown, 1},
		{"orphan-chord-is-unavailable-not-rejected", timelineNote("1", `<chord/>`, ""), validation.OutcomeUnknown, 1},
		{"backup-interrupts-principal", timelineNote("4", "", "") + `<backup><duration>4</duration></backup>` + timelineNote("1", `<chord/>`, ""), validation.OutcomeUnknown, 1},
		{"forward-interrupts-principal", timelineNote("4", "", "") + `<forward><duration>1</duration></forward>` + timelineNote("1", `<chord/>`, ""), validation.OutcomeUnknown, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assessTimelineBoth(t, (localFixture{body: semanticQADivisions + tc.body}).xml(), func(t *testing.T, r validation.Report) {
				semanticQAExpect(t, r, "MX40-TIMELINE-chord-duration-bound", tc.want, tc.count)
				if tc.count == 0 && !r.Satisfied() {
					t.Fatalf("positive case rejected: %s", localFindings(r))
				}
				if tc.want == validation.OutcomeUnknown && (r.AssessmentComplete || r.Conformance == validation.ConformanceViolated) {
					t.Errorf("unavailable principal fabricated a violation: %+v", r)
				}
				for _, f := range r.Findings {
					if strings.HasPrefix(string(f.Rule.ID), "MX40-XSD-") && f.Outcome == validation.OutcomeFail {
						t.Errorf("fixture schema failure: %+v", f)
					}
				}
			})
		})
	}
}

func TestTimelineSemanticQARegionLocalChangedDivisionsRewind(t *testing.T) {
	// 0 -> 4 at D1; D2 -> 6; backup 2/D2 -> 5, wholly inside
	// the established D2 region. No boundary-crossing interpretation is chosen.
	body := semanticQADivisions + timelineNote("4", "", "") + `<attributes><divisions>2</divisions></attributes>` + timelineNote("4", "", "") + `<backup><duration>2</duration></backup>` + timelineNote("2", "", "") + timelineNote("1", `<chord/>`, "")
	assessTimelineBoth(t, (localFixture{body: body}).xml(), func(t *testing.T, r validation.Report) {
		if !r.Satisfied() {
			t.Fatalf("region-local rewind overblocked: %s", localFindings(r))
		}
	})
}

// semanticQAObserve uses the actual source observer or current-model walker,
// with only the requested hierarchy-enumeration gap injected at its boundary.
func semanticQAAdmit(t *testing.T, source string, target validation.Target, engine *validation.Engine, profile validation.ProfileID, gap string) (*validation.Session, *validation.Account, validation.SyntaxState) {
	t.Helper()
	ctx := context.Background()
	account, err := validation.NewBudgetAccount(validation.Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	documentID := modelDocumentID
	if target == validation.TargetSource {
		documentID = sourceDocumentID
	}
	session, err := engine.Begin(ctx, validation.RunOptions{DocumentID: documentID, Profile: profile, Account: account})
	if err != nil {
		t.Fatal(err)
	}
	sink := &timelineGapSink{Session: session, headers: map[validation.NodeID]validation.NodeHeader{}, gapName: gap}
	syntax := validation.SyntaxNotAssessed
	if target == validation.TargetModel {
		doc, err := Decode(strings.NewReader(source))
		if err != nil {
			t.Fatal(err)
		}
		_, err = walkModel(ctx, doc, sink, account, modelDocumentID, modelWalkOptions{})
		if err != nil {
			t.Fatal(err)
		}
	} else {
		parser, err := newSourceXMLTokenReader(strings.NewReader(source), sourceXMLConfig{Context: ctx, Account: account, MaxDepth: 256})
		if err != nil {
			t.Fatal(err)
		}
		observer, err := newSourceFactObserver(ctx, sink, account)
		if err != nil {
			t.Fatal(err)
		}
		src := &observingXMLSource{ctx: ctx, lexical: parser, decoder: xml.NewTokenDecoder(parser), observer: observer}
		for {
			_, err = src.Token()
			if err != nil {
				break
			}
		}
		if !errors.Is(err, io.EOF) {
			t.Fatal(err)
		}
		if err = observer.close(); err != nil {
			t.Fatal(err)
		}
		if err = parser.Close(); err != nil {
			t.Fatal(err)
		}
		syntax = validation.SyntaxWellFormed
	}
	return session, account, syntax
}

func semanticQAObserve(t *testing.T, source string, target validation.Target, engine *validation.Engine, profile validation.ProfileID, gap string) validation.Report {
	t.Helper()
	session, _, syntax := semanticQAAdmit(t, source, target, engine, profile, gap)
	r, err := session.Finish(context.Background(), validation.Completion{InputComplete: true, Syntax: syntax})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestTimelineSemanticQAHierarchyGapsPreserveAbsenceUncertainty(t *testing.T) {
	engine, err := newTimelineAssessmentEngine()
	if err != nil {
		t.Fatal(err)
	}
	for _, form := range []string{"partwise", "timewise"} {
		for _, target := range []validation.Target{validation.TargetSource, validation.TargetModel} {
			profile := ProfileModelTimeline.ID()
			if target == validation.TargetSource {
				profile = ProfileSourceTimeline.ID()
			}
			for _, gap := range []string{"score-" + form, "part", "measure"} {
				t.Run(form+"/"+string(target)+"/"+gap, func(t *testing.T) {
					source := semanticQAScore(form, [][2]string{{timelineNumeral("", ""), timelineNumeral(semanticQALocalKey, "")}})
					r := semanticQAObserve(t, source, target, engine, profile, gap)
					got := semanticQAFindings(r, "MX40-TIMELINE-numeral-requires-key")
					if len(got) != 1 || got[0].Outcome == validation.OutcomeFail || r.AssessmentComplete {
						t.Fatalf("hierarchy gap fabricated absence or blocked independent local key: %s", localFindings(r))
					}
				})
			}
		}
	}
}

func TestTimelineSemanticQACanonicalOwnerSurvivesUnrelatedInvalidChild(t *testing.T) {
	for _, form := range []string{"partwise", "timewise"} {
		t.Run(form, func(t *testing.T) {
			source := semanticQAScore(form, [][2]string{{`<attributes><key><fifths>0</fifths></key></attributes>` + timelineNumeral("", ""), ""}})
			source = strings.Replace(source, `<part-name>First</part-name>`, `<part-name justify="bad-value">First</part-name>`, 1)
			assessTimelineBoth(t, source, func(t *testing.T, r validation.Report) {
				semanticQAExpect(t, r, "MX40-TIMELINE-numeral-requires-key", "", 0)
				for _, f := range r.Findings {
					if strings.HasPrefix(string(f.Rule.ID), "MX40-LOCAL-part-id-target/") {
						t.Errorf("unrelated part-name invalidity lost canonical owner: %+v", f)
					}
				}
				// This declared enum is preserved in current-model fields too.
				if r.Conformance != validation.ConformanceViolated {
					t.Error("missing original-source attribute violation")
				}
			})
		})
	}
}

func TestTimelineSemanticQANewAllStaffKeySupersedesOlderInvalidLocalKey(t *testing.T) {
	body := `<attributes><divisions>1</divisions><key number="2"><cancel location="bad-location">0</cancel><fifths>0</fifths></key><staves>2</staves></attributes>` + timelineNote("1", "", `<staff>2</staff>`) + `<attributes><key><fifths>3</fifths></key></attributes>` + timelineNumeral("", `<staff>2</staff>`)
	assessTimelineBoth(t, (localFixture{body: body}).xml(), func(t *testing.T, r validation.Report) {
		semanticQAExpect(t, r, "MX40-TIMELINE-numeral-requires-key", "", 0)
		if r.Conformance != validation.ConformanceViolated {
			t.Error("supersession erased historical scalar failure")
		}
	})
}

func TestTimelineSemanticQAScalarCauseIsPreservedWithoutDuplicateProseFailure(t *testing.T) {
	for _, tc := range []struct{ name, body, id string }{
		{"invalid-key", `<attributes><key><cancel location="bad-location">0</cancel><fifths>0</fifths></key></attributes>` + timelineNumeral("", ""), "MX40-TIMELINE-numeral-requires-key"},
		{"invalid-local-key", timelineNumeral(`<numeral-key><numeral-fifths>0</numeral-fifths><numeral-mode>bad-mode</numeral-mode></numeral-key>`, ""), "MX40-TIMELINE-numeral-requires-key"},
		{"invalid-chord-duration", semanticQADivisions + timelineNote("1", "", "") + timelineNote("-1", `<chord/>`, ""), "MX40-TIMELINE-chord-duration-bound"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assessTimelineBoth(t, (localFixture{body: tc.body}).xml(), func(t *testing.T, r validation.Report) {
				got := semanticQAFindings(r, tc.id)
				if len(got) != 1 || got[0].Outcome != validation.OutcomeBlocked || got[0].Cause == nil {
					t.Fatalf("lost causal blocker: %s", localFindings(r))
				}
				cause := got[0].Cause
				found := false
				for _, f := range r.Findings {
					if f.Rule == cause.Rule && f.Subject == cause.Subject && f.Outcome == validation.OutcomeFail {
						found = true
					}
				}
				if !found {
					t.Errorf("cause does not point to retained XSD failure: %+v; %s", cause, localFindings(r))
				}
				if r.Conformance != validation.ConformanceViolated || r.AssessmentComplete {
					t.Error("causal incompleteness erased known scalar violation")
				}
			})
		})
	}
}

func TestTimelineSemanticQALocalKeySurvivesUnsupportedProvider(t *testing.T) {
	schema := timelineContractSchema(t)
	provider, consumer := timelineContractPlans(t, schema, true)
	for _, target := range []validation.Target{validation.TargetSource, validation.TargetModel} {
		t.Run(string(target), func(t *testing.T) {
			rules, profiles, bindings := timelineContractCatalog(t, schema, provider, consumer, true)
			profiles[0].Target = target
			profiles[0].Support[1].State = validation.ImplementationUnsupported
			profiles[0].Support[1].Reason = "qa-provider-unavailable"
			catalog, err := validation.NewCatalog(rules, profiles, bindings)
			if err != nil {
				t.Fatal(err)
			}
			engine, err := validation.NewEngine(catalog)
			if err != nil {
				t.Fatal(err)
			}
			for _, local := range []bool{false, true} {
				key := ""
				if local {
					key = semanticQALocalKey
				}
				r := semanticQAObserve(t, (localFixture{body: timelineNumeral(key, "")}).xml(), target, engine, profiles[0].ID, "")
				semanticQAExpect(t, r, "z-timeline", validation.OutcomeUnsupported, 1)
				if local {
					semanticQAExpect(t, r, "a-consumer", "", 0)
				} else {
					got := semanticQAFindings(r, "a-consumer")
					if len(got) != 1 || got[0].Outcome != validation.OutcomeUnsupported || got[0].Reason != "qa-provider-unavailable" {
						t.Fatalf("fallback lost provider cause: %s", localFindings(r))
					}
				}
				if r.AssessmentComplete || r.Satisfied() {
					t.Error("unavailable required provider reported complete")
				}
			}
		})
	}
}

func TestTimelineSemanticQALocalKeyProofSurvivesFinalizationResourceStop(t *testing.T) {
	schema := timelineContractSchema(t)
	provider, consumer := timelineContractPlans(t, schema, true)
	for _, target := range []validation.Target{validation.TargetSource, validation.TargetModel} {
		for _, local := range []bool{false, true} {
			name := string(target) + "/fallback"
			if local {
				name = string(target) + "/local-key"
			}
			t.Run(name, func(t *testing.T) {
				var reports [2]validation.Report
				for variant := range reports {
					rules, profiles, bindings := timelineContractCatalog(t, schema, provider, consumer, true)
					profiles[0].Target = target
					// All XSD and owner obligations are identical. This control removes
					// only the consumer so Counts.Pass can certify its pre-stop proof.
					if variant == 0 {
						profiles[0].Required = profiles[0].Required[1:]
						profiles[0].Support = profiles[0].Support[1:]
					}
					catalog, err := validation.NewCatalog(rules, profiles, bindings)
					if err != nil {
						t.Fatal(err)
					}
					engine, err := validation.NewEngine(catalog)
					if err != nil {
						t.Fatal(err)
					}
					key := ""
					if local {
						key = semanticQALocalKey
					}
					session, account, syntax := semanticQAAdmit(t, (localFixture{body: timelineNumeral(key, "")}).xml(), target, engine, profiles[0].ID, "")
					used, err := account.Usage(validation.BudgetRuleSteps)
					if err != nil {
						t.Fatal(err)
					}
					if err = account.Charge(validation.BudgetRuleSteps, account.Limits().MaxRuleSteps-used); err != nil {
						t.Fatal(err)
					}
					reports[variant], err = session.Finish(context.Background(), validation.Completion{InputComplete: true, Syntax: syntax})
					if !errors.Is(err, validation.ErrResourceLimit) || reports[variant].AssessmentComplete {
						t.Fatalf("missing authentic finalization stop: %v %+v", err, reports[variant])
					}
					if reports[variant].Conformance == validation.ConformanceViolated {
						t.Errorf("resource stop fabricated violation: %s", localFindings(reports[variant]))
					}
				}
				want := reports[0].Counts.Pass
				if local {
					want++
				}
				if reports[1].Counts.Pass != want {
					t.Fatalf("independent local proof did not survive: control=%+v consumer=%+v", reports[0].Counts, reports[1].Counts)
				}
			})
		}
	}
}

func TestTimelineSemanticQAProviderCompletionDoesNotPromiseEveryFactKnown(t *testing.T) {
	// No divisions can normalize this principal, but no selected consumer
	// needs its duration. The own numeral-key is a complete local proof.
	source := (localFixture{body: timelineNote("1", "", "") + timelineNumeral(semanticQALocalKey, "")}).xml()
	assessTimelineBoth(t, source, func(t *testing.T, r validation.Report) {
		if !r.Satisfied() {
			t.Fatalf("finalized provider was confused with universally known state: %s", localFindings(r))
		}
		if !strings.Contains(r.Profile.NormativeClosure, "pass does not assert that every optional or ambiguous musical fact is known") {
			t.Error("profile conceals provider-pass limitation")
		}
	})
}

func TestTimelineSemanticQAProvenChordFailureSurvivesLaterUnitGap(t *testing.T) {
	body := semanticQADivisions + timelineNote("1", "", "") + timelineNote("2", `<chord/>`, "") + `<attributes><divisions>2</divisions></attributes><backup><duration>2</duration></backup>` + timelineNote("2", "", "") + timelineNote("1", `<chord/>`, "") + timelineNumeral(semanticQALocalKey, "")
	assessTimelineBoth(t, (localFixture{body: body}).xml(), func(t *testing.T, r validation.Report) {
		var fails, incomplete int
		for _, f := range semanticQAFindings(r, "MX40-TIMELINE-chord-duration-bound") {
			switch f.Outcome {
			case validation.OutcomeFail:
				fails++
				if len(f.Related) != 1 || f.Cause != nil {
					t.Errorf("proven failure provenance=%+v", f)
				}
			case validation.OutcomeUnknown, validation.OutcomeUnsupported, validation.OutcomeBlocked:
				incomplete++
			}
		}
		if fails != 1 || incomplete != 1 || r.Conformance != validation.ConformanceViolated || r.AssessmentComplete {
			t.Fatalf("prefix violation or suffix gap lost: %s", localFindings(r))
		}
		semanticQAExpect(t, r, "MX40-TIMELINE-numeral-requires-key", "", 0)
	})
}

func TestTimelineSemanticQAFixtureSchemaControls(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal("xmllint required for fixture controls")
		}
		t.Skip("xmllint is not installed")
	}
	schema, err := filepath.Abs(filepath.Join("schema", "musicxml-4.0"))
	if err != nil {
		t.Fatal(err)
	}
	key := `<attributes><divisions>1</divisions><key><fifths>0</fifths></key></attributes>`
	for _, tc := range []struct {
		name, source string
		valid        bool
	}{
		{"partwise-repeated-label", semanticQAScore("partwise", [][2]string{{key, ""}, {timelineNumeral("", ""), timelineNumeral("", "")}}), true},
		{"timewise-repeated-idref-and-label", semanticQAScore("timewise", [][2]string{{key, ""}, {timelineNumeral("", ""), timelineNumeral("", "")}}), true},
		{"chord-bound-is-not-an-xsd-rule", (localFixture{body: semanticQADivisions + timelineNote("1", "", "") + timelineNote("2", `<chord/>`, "")}).xml(), true},
		{"numeral-key-is-not-an-xsd-rule", (localFixture{body: timelineNumeral("", "")}).xml(), true},
		{"cue-principal-normal-chord", (localFixture{body: semanticQADivisions + timelineNote("2", `<cue/>`, "") + timelineNote("1", `<chord/>`, "")}).xml(), true},
		{"normal-principal-cue-chord", (localFixture{body: semanticQADivisions + timelineNote("2", "", "") + timelineNote("1", `<cue/><chord/>`, "")}).xml(), true},
		{"bad-cancel-location", (localFixture{body: `<attributes><key><cancel location="bad-location">0</cancel><fifths>0</fifths></key></attributes>`}).xml(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertXMLLintOutcome(t, xmllint, filepath.Join(schema, "musicxml.xsd"), filepath.Join(schema, "catalog.xml"), tc.source, tc.valid)
		})
	}
}

func TestTimelineAnalysisKeyStaysAtUnshiftedHarmonyOrigin(t *testing.T) {
	key := `<attributes><key><fifths>0</fifths></key></attributes>`
	for _, tc := range []struct {
		name, body string
		want       validation.Outcome
		count      int
	}{
		{"visual-forward-cannot-borrow-future-key", semanticQADivisions + timelineNote("1", "", "") + timelineNumeral("", `<offset>2</offset>`) + timelineNote("1", "", "") + key, validation.OutcomeFail, 1},
		{"visual-backward-does-not-erase-present-key", semanticQADivisions + timelineNote("2", "", "") + key + timelineNote("1", "", "") + timelineNumeral("", `<offset>-2</offset>`), "", 0},
		{"sound-flag-does-not-move-analysis-key", semanticQADivisions + timelineNote("1", "", "") + timelineNumeral("", `<offset sound="yes">2</offset>`) + timelineNote("1", "", "") + key, validation.OutcomeFail, 1},
		{"local-analysis-key-independent", semanticQADivisions + timelineNote("1", "", "") + timelineNumeral(semanticQALocalKey, `<offset>2</offset>`) + timelineNote("1", "", "") + key, "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, form := range []string{"partwise", "timewise"} {
				t.Run(form, func(t *testing.T) {
					assessTimelineBoth(t, semanticQAScore(form, [][2]string{{tc.body, ""}}), func(t *testing.T, r validation.Report) {
						semanticQAExpect(t, r, "MX40-TIMELINE-numeral-requires-key", tc.want, tc.count)
						if !r.AssessmentComplete {
							t.Fatalf("unexpected incomplete %s", localFindings(r))
						}
					})
				})
			}
		})
	}
}
