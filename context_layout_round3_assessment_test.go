package musicxml

import (
	"fmt"
	"github.com/go-muse/go-musicxml/validation"
	"testing"
)

const layoutRound3Margins = `<page-layout><page-margins type="odd"><left-margin>10</left-margin><right-margin>10</right-margin><top-margin>10</top-margin><bottom-margin>10</bottom-margin></page-margins><page-margins type="even"><left-margin>20</left-margin><right-margin>20</right-margin><top-margin>20</top-margin><bottom-margin>20</bottom-margin></page-margins></page-layout>`

func TestLayoutRound3NewSystemOwnsIncompleteWidth(t *testing.T) {
	header := layoutRound1Supports + `<defaults><page-layout><page-height>100</page-height><page-width>100</page-width><page-margins><left-margin>10</left-margin><right-margin>10</right-margin><top-margin>10</top-margin><bottom-margin>10</bottom-margin></page-margins></page-layout><system-layout><system-margins><left-margin>0</left-margin><right-margin>0</right-margin></system-margins></system-layout></defaults>`
	layoutRound1Reports(t, header, [][]string{{``, ``, `<print new-system="yes"/><attributes><staff-details print-object="invalid"/></attributes>`, `<attributes><staff-details print-object="yes"/></attributes>`}}, []string{"40", "40", "40", "80"}, func(t *testing.T, r validation.Report) {
		rows := layoutFacts(r, "width-outcome")
		if len(rows) != 2 {
			t.Fatalf("width rows: %+v", rows)
		}
		if rows[0].Value.State != validation.EvidenceKnown || rows[0].Value.Value.Text != "consistent" {
			t.Errorf("completed previous system downgraded: %+v", rows[0].Value)
		}
		if rows[1].Value.State != validation.EvidenceUnknown || rows[1].Value.Reason != "system-width-prerequisite-unavailable" {
			t.Errorf("incomplete new system certified: %+v", rows[1].Value)
		}
	})
}

func TestLayoutRound3BlankPageConsensus(t *testing.T) {
	for _, tc := range []struct {
		name, other string
		want        validation.EvidenceState
		causal      bool
	}{
		{"invalid-break", `<print new-page="bad"/>`, validation.EvidenceInvalid, true},
		{"conflicting-break", `<print new-page="no"/>`, validation.EvidenceUnknown, false},
		{"omitted-blank", `<print new-page="yes"/>`, validation.EvidenceUnknown, false},
		{"matching-blank", `<print new-page="yes" blank-page="1"/>`, validation.EvidenceKnown, false},
	} {
		for _, initial := range []bool{true, false} {
			for _, reverse := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/initial=%t/reverse=%t", tc.name, initial, reverse), func(t *testing.T) {
					parts := [][]string{{`<print new-page="yes" blank-page="1"/>`}, {tc.other}}
					if reverse {
						parts[0], parts[1] = parts[1], parts[0]
					}
					if !initial {
						for i := range parts {
							parts[i] = append([]string{``}, parts[i]...)
						}
					}
					layoutRound1Reports(t, layoutRound1Supports+`<defaults>`+layoutRound3Margins+`</defaults>`, parts, nil, func(t *testing.T, r validation.Report) {
						rows := layoutFacts(r, "page-left-margin")
						if !initial {
							rows = rows[2:]
						}
						for _, row := range rows {
							if row.Value.State != tc.want || tc.causal && row.Value.Cause == nil {
								t.Errorf("blank consensus lost: %+v", row.Value)
							}
							if tc.want == validation.EvidenceKnown {
								want := "20"
								if !initial {
									want = "10"
								}
								if row.Value.Value.Decimal.String() != want {
									t.Errorf("parity: %+v", row.Value)
								}
							}
						}
					})
				})
			}
		}
	}
}

func TestLayoutRound3StaffApplicabilityPreservesCause(t *testing.T) {
	for _, attr := range []string{`print-object="invalid"`, `print-object="no" print-spacing="invalid"`} {
		t.Run(attr, func(t *testing.T) {
			layoutRound1Reports(t, layoutRound1Supports+`<defaults><staff-layout><staff-distance>12</staff-distance></staff-layout></defaults>`, [][]string{{`<attributes><staff-details ` + attr + `/></attributes>`}}, nil, func(t *testing.T, r validation.Report) {
				rows := layoutFacts(r, "staff-distance")
				if len(rows) != 1 {
					t.Fatalf("rows: %+v", rows)
				}
				if got := rows[0].Layout.Applicability; got.State != validation.EvidenceInvalid || got.Cause == nil {
					t.Errorf("applicability lost cause: %+v", got)
				}
			})
		})
	}
}

func TestLayoutRound3AllCutawayPublisherIsUnresolved(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		t.Run(fmt.Sprint(reverse), func(t *testing.T) {
			hidden := `<attributes><staff-details print-object="no" print-spacing="no"/></attributes>`
			parts := [][]string{{`<print><system-layout><system-distance>73</system-distance></system-layout></print>` + hidden}, {hidden}}
			if reverse {
				parts[0], parts[1] = parts[1], parts[0]
			}
			layoutRound1Reports(t, `<defaults><system-layout><system-distance>20</system-distance></system-layout></defaults>`, parts, nil, func(t *testing.T, r validation.Report) {
				for _, row := range layoutFacts(r, "system-distance") {
					if row.Value.State != validation.EvidenceUnknown {
						t.Errorf("cutaway publisher guessed: %+v", row.Value)
					}
				}
			})
		})
	}
}

func TestLayoutRound3DerivedProvenance(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			attr := ""
			want := validation.ContextDocumentedDefault
			if explicit {
				attr = ` default-x="2" default-y="3"`
				want = validation.ContextExplicit
			}
			header := `<defaults><scaling><millimeters>7</millimeters><tenths>40</tenths></scaling><page-layout><page-height>100</page-height><page-width>100</page-width><page-margins><left-margin>10</left-margin><right-margin>10</right-margin><top-margin>10</top-margin><bottom-margin>10</bottom-margin></page-margins></page-layout><system-layout><system-margins><left-margin>0</left-margin><right-margin>0</right-margin></system-margins><system-distance>30</system-distance><system-dividers><left-divider` + attr + `/><right-divider` + attr + `/></system-dividers></system-layout></defaults>`
			layoutRound1Reports(t, header, [][]string{{`<print/>`}}, nil, func(t *testing.T, r validation.Report) {
				for _, p := range []string{"left-divider-x", "left-divider-y", "right-divider-x", "right-divider-y", "effective-left-margin", "effective-right-margin", "millimeters-per-tenth"} {
					expected := validation.ContextExplicit
					if p != "effective-left-margin" && p != "effective-right-margin" && p != "millimeters-per-tenth" {
						expected = want
					}
					rows := layoutFacts(r, p)
					if len(rows) != 1 {
						t.Fatalf("%s rows %+v", p, rows)
					}
					if rows[0].Layout.Provenance != expected {
						t.Errorf("%s provenance %q want %q", p, rows[0].Layout.Provenance, expected)
					}
					if !explicit && p != "effective-left-margin" && p != "effective-right-margin" && p != "millimeters-per-tenth" && rows[0].Layout.Raw.State != validation.EvidenceAbsent {
						t.Errorf("%s synthesized source coordinate: %+v", p, rows[0].Layout.Raw)
					}
				}
			})
		})
	}
}

func TestLayoutRound3BlankPageAbsentAndIgnoredControls(t *testing.T) {
	for _, tc := range []struct{ name, p1, p2, want string }{
		{"all-absent", `<print new-page="yes"/>`, `<print new-page="yes"/>`, "10"},
		{"ignored", `<print new-page="no" blank-page="1"/>`, `<print new-page="no"/>`, "10"},
		{"invalid-without-blank", `<print new-page="bad"/>`, `<print/>`, ""},
	} {
		for _, reverse := range []bool{false, true} {
			t.Run(tc.name+fmt.Sprint(reverse), func(t *testing.T) {
				parts := [][]string{{tc.p1}, {tc.p2}}
				if reverse {
					parts[0], parts[1] = parts[1], parts[0]
				}
				layoutRound1Reports(t, layoutRound1Supports+`<defaults>`+layoutRound3Margins+`</defaults>`, parts, nil, func(t *testing.T, r validation.Report) {
					for _, row := range layoutFacts(r, "page-left-margin") {
						if tc.want == "" {
							if row.Value.State != validation.EvidenceInvalid || row.Value.Cause == nil {
								t.Errorf("initial cause: %+v", row.Value)
							}
						} else if row.Value.State != validation.EvidenceKnown || row.Value.Value.Decimal.String() != tc.want {
							t.Errorf("absence/ignored parity: %+v", row.Value)
						}
					}
				})
			})
		}
	}
}

func TestLayoutRound3AllCutawayWithoutOverrideKeepsDefaults(t *testing.T) {
	layoutRound1Reports(t, `<defaults><system-layout><system-distance>20</system-distance></system-layout></defaults>`, [][]string{{`<attributes><staff-details print-object="no" print-spacing="no"/></attributes>`}}, nil, func(t *testing.T, r validation.Report) {
		layoutMatrixValues(t, r, "system-distance", lmk("20"))
	})
}

func TestLayoutRound3CutawayPublisherRepairsAtNextSystem(t *testing.T) {
	header := layoutRound1Supports + `<defaults><page-layout><page-height>100</page-height><page-width>100</page-width></page-layout><system-layout><system-distance>20</system-distance></system-layout></defaults>`
	layoutRound1Reports(t, header, [][]string{{`<print><system-layout><system-distance>73</system-distance></system-layout></print><attributes><staff-details print-object="no" print-spacing="no"/></attributes>`, `<print new-system="yes"/><attributes><staff-details print-object="yes"/></attributes>`}}, nil, func(t *testing.T, r validation.Report) {
		layoutMatrixValues(t, r, "system-distance", lmu(), lmk("20"))
		layoutMatrixValues(t, r, "page-height", lmk("100"), lmk("100"))
	})
}

func TestLayoutRound3BlankDisagreementDoesNotHideInvalidBreak(t *testing.T) {
	for _, initial := range []bool{false, true} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("initial=%t/reverse=%t", initial, reverse), func(t *testing.T) {
				parts := [][]string{{`<print new-page="bad"/>`}, {`<print new-page="yes" blank-page="1"/>`}, {`<print new-page="yes"/>`}}
				if reverse {
					parts[0], parts[2] = parts[2], parts[0]
				}
				if !initial {
					for i := range parts {
						parts[i] = append([]string{``}, parts[i]...)
					}
				}
				layoutRound1Reports(t, layoutRound1Supports+`<defaults>`+layoutRound3Margins+`</defaults>`, parts, nil, func(t *testing.T, r validation.Report) {
					rows := layoutFacts(r, "page-left-margin")
					if !initial {
						rows = rows[3:]
					}
					if len(rows) != 3 {
						t.Fatalf("rows: %+v", rows)
					}
					for _, row := range rows {
						if row.Value.State != validation.EvidenceInvalid || row.Value.Cause == nil {
							t.Errorf("blank disagreement hid invalid break: %+v", row.Value)
						}
					}
				})
			})
		}
	}
}
