package musicxml

import (
	"fmt"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// CHECK-context-harmony-none-hide-root: the numeral-step spelling in the
// source advice refers to numeral-root, the actual numeral display override.
// Each advice row remains attached to its own kind in a stacked harmony.
func TestLocalHarmonyNoneNumeralAdviceAndChordScope(t *testing.T) {
	engine := localContextTestEngine(t)
	const rootAdvice = "set-root-step-text-to-empty-to-suppress-root"
	const numeralAdvice = "set-numeral-root-text-to-empty-to-suppress-numeral"
	type wantAdvice struct {
		text string
		app  validation.EvidenceState
		on   bool
	}
	root := `<root><root-step>C</root-step></root>`
	numeral := `<numeral><numeral-root>5</numeral-root></numeral>`
	for _, tc := range []struct {
		name, chords string
		want         []wantAdvice
	}{
		{"numeral-absent-text", numeral + `<kind>none</kind>`, []wantAdvice{{numeralAdvice, validation.EvidenceKnown, true}}},
		{"numeral-visible-text", `<numeral><numeral-root text="V">5</numeral-root></numeral><kind>none</kind>`, []wantAdvice{{numeralAdvice, validation.EvidenceKnown, true}}},
		{"numeral-empty-text", `<numeral><numeral-root text="">5</numeral-root></numeral><kind>none</kind>`, []wantAdvice{{numeralAdvice, validation.EvidenceKnown, true}}},
		{"numeral-whitespace-text", `<numeral><numeral-root text=" &#x9; ">5</numeral-root></numeral><kind>none</kind>`, []wantAdvice{{numeralAdvice, validation.EvidenceKnown, true}}},
		{"root-then-numeral", root + `<kind>none</kind>` + numeral + `<kind>none</kind>`, []wantAdvice{{rootAdvice, validation.EvidenceKnown, true}, {numeralAdvice, validation.EvidenceKnown, true}}},
		{"numeral-then-root", numeral + `<kind>none</kind>` + root + `<kind>none</kind>`, []wantAdvice{{numeralAdvice, validation.EvidenceKnown, true}, {rootAdvice, validation.EvidenceKnown, true}}},
		{"independent-false-condition", numeral + `<kind>none</kind>` + root + `<kind>major</kind>`, []wantAdvice{{numeralAdvice, validation.EvidenceKnown, true}, {rootAdvice, validation.EvidenceKnown, false}}},
		{"function-alone-unchanged", `<function>V</function><kind>none</kind>`, []wantAdvice{{rootAdvice, validation.EvidenceUnknown, false}}},
		{"prior-root-does-not-bind-function", root + `<kind>none</kind><function>V</function><kind>none</kind>`, []wantAdvice{{rootAdvice, validation.EvidenceKnown, true}, {rootAdvice, validation.EvidenceUnknown, false}}},
		{"prior-numeral-does-not-bind-function", numeral + `<kind>none</kind><function>V</function><kind>none</kind>`, []wantAdvice{{numeralAdvice, validation.EvidenceKnown, true}, {rootAdvice, validation.EvidenceUnknown, false}}},
	} {
		for _, timewise := range []bool{false, true} {
			for _, model := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/timewise=%t/model=%t", tc.name, timewise, model), func(t *testing.T) {
					report := localContextAssess(t, engine, localContextFixture(`<harmony>`+tc.chords+`</harmony>`, timewise), model)
					if !report.Satisfied() {
						t.Fatalf("advice became a rejection: %s", localFindings(report))
					}
					var rows []validation.ContextResult
					for _, row := range report.Context {
						if row.Property == "local.advisory-harmony-none-root" {
							rows = append(rows, row)
						}
					}
					if len(rows) != len(tc.want) {
						t.Fatalf("got %d advice rows; want %d", len(rows), len(tc.want))
					}
					for i, want := range tc.want {
						got := rows[i]
						if got.Value.State != validation.EvidenceKnown || got.Value.Value.Text != want.text || got.Layout.Applicability.State != want.app || got.Layout.Applicability.Value != want.on {
							t.Errorf("advice[%d] = %+v; want %+v", i, got, want)
						}
					}
				})
			}
		}
	}
}

func TestLocalHarmonyNoneInvalidKindIsIndependentlyCausal(t *testing.T) {
	engine := localContextTestEngine(t)
	body := `<harmony><root><root-step>C</root-step></root><kind>wrong</kind><numeral><numeral-root>5</numeral-root></numeral><kind>none</kind></harmony>`
	for _, timewise := range []bool{false, true} {
		for _, model := range []bool{false, true} {
			t.Run(fmt.Sprintf("timewise=%t/model=%t", timewise, model), func(t *testing.T) {
				report := localContextAssess(t, engine, localContextFixture(body, timewise), model)
				var rows []validation.ContextResult
				for _, row := range report.Context {
					if row.Property == "local.advisory-harmony-none-root" {
						rows = append(rows, row)
					}
				}
				if len(rows) != 2 {
					t.Fatalf("advice rows = %d; want 2", len(rows))
				}
				if app := rows[0].Layout.Applicability; app.State != validation.EvidenceInvalid || app.Cause == nil {
					t.Errorf("invalid kind lost its cause: %+v", app)
				}
				if row := rows[1]; row.Layout.Applicability.State != validation.EvidenceKnown || !row.Layout.Applicability.Value || row.Value.State != validation.EvidenceKnown || row.Value.Value.Text != "set-numeral-root-text-to-empty-to-suppress-numeral" || row.Value.Cause != nil {
					t.Errorf("independent numeral advice contaminated: %+v", row)
				}
			})
		}
	}
}
