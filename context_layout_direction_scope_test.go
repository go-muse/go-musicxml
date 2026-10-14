package musicxml

import (
	"reflect"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// CHECK-context-direction-format-inheritance and
// CHECK-context-direction-type-format-scope: visual sequencing of wrappers
// does not extend the child-component formatting scope across their boundary.
func TestLayoutDirectionWrapperFormattingReset(t *testing.T) {
	for _, tc := range []struct {
		name, header, body, property string
		want                         []layoutMatrixValue
		lastReason                   string
	}{
		{"word-font-reset", `<defaults><word-font font-family="Default"/></defaults>`, `<direction><direction-type><words font-family="Earlier">one</words></direction-type><direction-type><words>two</words></direction-type></direction>`, "font-family", []layoutMatrixValue{lmk("Earlier"), lmk("Default")}, ""},
		{"language-reset", "", `<direction><direction-type><words xml:lang="fr">one</words></direction-type><direction-type><words>two</words></direction-type></direction>`, "language", []layoutMatrixValue{lmk("fr"), lmk("it")}, ""},
		{"element-enclosure-reset", "", `<direction><direction-type><words enclosure="rectangle">one</words></direction-type><direction-type><rehearsal>A</rehearsal></direction-type></direction>`, "enclosure", []layoutMatrixValue{lmk("rectangle"), lmk("square")}, ""},
		{"justify-reset", "", `<direction><direction-type><words justify="right">one</words></direction-type><direction-type><words>two</words></direction-type></direction>`, "justify", []layoutMatrixValue{lmk("right"), lmk("left")}, ""},
		{"application-choice-reset", "", `<direction><direction-type><words font-family="Earlier">one</words></direction-type><direction-type><words>two</words></direction-type></direction>`, "font-family", []layoutMatrixValue{lmk("Earlier"), lmu()}, "application-format-choice"},
		{"prior-invalid-does-not-mask-default", `<defaults><word-font font-style="normal"/></defaults>`, `<direction><direction-type><words font-style="bad">one</words></direction-type><direction-type><words>two</words></direction-type></direction>`, "font-style", []layoutMatrixValue{lmi(), lmk("normal")}, ""},
		{"independent-invalid-default", `<defaults><word-font font-style="bad"/></defaults>`, `<direction><direction-type><words font-style="italic">one</words></direction-type><direction-type><words>two</words></direction-type></direction>`, "font-style", []layoutMatrixValue{lmk("italic"), lmi()}, ""},
		{"local-explicit-overrides-invalid-default-and-predecessor", `<defaults><word-font font-style="bad"/></defaults>`, `<direction><direction-type><words font-style="bad">one</words></direction-type><direction-type><words font-style="normal">two</words></direction-type></direction>`, "font-style", []layoutMatrixValue{lmi(), lmk("normal")}, ""},
		{"explicit-component-only", `<defaults><word-font font-family="Default" font-style="normal"/></defaults>`, `<direction><direction-type><words font-family="Earlier" font-style="italic">one</words></direction-type><direction-type><words font-style="italic">two</words><words>three</words></direction-type></direction>`, "font-family", []layoutMatrixValue{lmk("Earlier"), lmk("Default"), lmk("Default")}, ""},
		{"new-direction-reset", `<defaults><word-font font-family="Default"/></defaults>`, `<direction><direction-type><words font-family="Earlier">one</words></direction-type></direction><direction><direction-type><words>two</words></direction-type></direction>`, "font-family", []layoutMatrixValue{lmk("Earlier"), lmk("Default")}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			layoutReports(t, tc.body, tc.header, func(t *testing.T, report validation.Report) {
				rows := directionScopeTail(t, report, tc.property, tc.want)
				last := rows[len(rows)-1]
				if tc.lastReason != "" && string(last.Value.Reason) != tc.lastReason {
					t.Errorf("last reason %q; want %q", last.Value.Reason, tc.lastReason)
				}
				if tc.name == "independent-invalid-default" {
					if last.Value.Cause == nil || last.Value.Cause.Subject == rows[0].Subject {
						t.Errorf("default cause replaced by prior-wrapper state: %+v", last.Value)
					}
				}
				if last.Value.State == validation.EvidenceKnown && last.Value.Cause != nil {
					t.Errorf("known local/default value retained unrelated invalid cause: %+v", last.Value)
				}
				layoutMatrixNoDefect(t, report)
			})
		})
	}
}

func directionScopeTail(t *testing.T, report validation.Report, property string, want []layoutMatrixValue) []validation.ContextResult {
	t.Helper()
	rows := layoutFacts(report, property)
	// Defaults/part-name precede these final direction components. They are
	// publishers or fixture scaffolding, not additional components of the test.
	if len(rows) < len(want) {
		t.Fatalf("%s has %d rows; need %d", property, len(rows), len(want))
	}
	rows = rows[len(rows)-len(want):]
	for i, w := range want {
		got := rows[i].Value
		if got.State != w.state || w.state == validation.EvidenceKnown && got.Value.Text != w.text {
			t.Errorf("%s component %d = %+v; want %+v", property, i, got, w)
		}
		if w.state == validation.EvidenceInvalid && got.Cause == nil {
			t.Errorf("%s component %d lost invalid cause", property, i)
		}
	}
	return rows
}

func TestLayoutDirectionIntraWrapperComponentsStayIndependent(t *testing.T) {
	body := `<direction><direction-type><words font-family="Text" font-style="italic" color="#123456" default-x="1" default-y="2" relative-x="3" relative-y="4">one</words><symbol font-weight="bold">note8thUp</symbol><words font-style="normal">two</words></direction-type><direction-type><words>three</words></direction-type></direction>`
	layoutReports(t, body, `<defaults><word-font font-family="Default" font-style="normal"/></defaults>`, func(t *testing.T, report validation.Report) {
		directionScopeTail(t, report, "font-family", []layoutMatrixValue{lmk("Text"), lmk("Text"), lmk("Text"), lmk("Default")})
		directionScopeTail(t, report, "font-style", []layoutMatrixValue{lmk("italic"), lmk("italic"), lmk("normal"), lmk("normal")})
		directionScopeTail(t, report, "font-weight", []layoutMatrixValue{lmu(), lmk("bold"), lmk("bold"), lmu()})
		directionScopeTail(t, report, "color", []layoutMatrixValue{lmk("#123456"), lmk("#123456"), lmk("#123456"), lmu()})
		for _, property := range []string{"default-x", "default-y", "relative-x", "relative-y"} {
			rows := layoutFacts(report, property)
			if len(rows) < 4 {
				t.Fatalf("missing position rows for %s", property)
			}
			rows = rows[len(rows)-4:]
			if rows[0].Value.State != validation.EvidenceKnown {
				t.Fatalf("explicit %s unavailable: %+v", property, rows[0].Value)
			}
			for _, row := range rows[1:] {
				if row.Value.State != validation.EvidenceAbsent {
					t.Errorf("%s inherited into another component: %+v", property, row)
				}
			}
		}
	})
}

func TestLayoutDirectionIntraWrapperInvalidCauseAndOverride(t *testing.T) {
	body := `<direction><direction-type><words font-style="bad">one</words><words>two</words><words font-style="normal">three</words></direction-type><direction-type><words font-style="italic">four</words></direction-type></direction>`
	layoutReports(t, body, "", func(t *testing.T, report validation.Report) {
		rows := directionScopeTail(t, report, "font-style", []layoutMatrixValue{lmi(), lmi(), lmk("normal"), lmk("italic")})
		if !reflect.DeepEqual(rows[0].Value.Cause, rows[1].Value.Cause) {
			t.Errorf("same-wrapper inherited cause changed: %+v, %+v", rows[0].Value, rows[1].Value)
		}
		for _, row := range rows[2:] {
			if row.Value.Cause != nil || row.Layout.Origin != row.Subject {
				t.Errorf("explicit component inherited unrelated state: %+v", row)
			}
		}
	})
}
