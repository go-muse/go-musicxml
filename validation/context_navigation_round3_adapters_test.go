package validation_test

import (
	"fmt"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestRound3NavigationOrdinaryExports(t *testing.T) {
	const attributes = `<attributes><divisions>1</divisions></attributes>`
	const tempo = `<direction><direction-type><words>tempo</words></direction-type><sound tempo="120"/></direction>`
	const dynamics = `<direction><direction-type><words>dynamics</words></direction-type><sound dynamics="90"/></direction>`
	for _, tc := range []struct {
		name, body, reason string
		kinds              []validation.NavigationKind
	}{
		{"parallel-sounds", tempo + dynamics + repeatR1Note, "", []validation.NavigationKind{validation.NavigationSound, validation.NavigationSound, validation.NavigationNote}},
		{"sound-listening", `<direction><direction-type><words>tempo and sync</words></direction-type><sound tempo="120"/><listening><sync type="bar"/></listening></direction>` + repeatR1Note, "", []validation.NavigationKind{validation.NavigationSound, validation.NavigationListening, validation.NavigationNote}},
		{"sound-before-backward", repeatR1Note + `<sound tempo="60"/><barline><repeat direction="backward"/></barline>`, "navigation-repeat-start-unavailable", []validation.NavigationKind{validation.NavigationNote, validation.NavigationSound, validation.NavigationBackward}},
		{"inactive-coda-before-backward", repeatR1Note + `<barline coda="C"><repeat direction="backward"/></barline>`, "navigation-repeat-start-unavailable", []validation.NavigationKind{validation.NavigationNote, validation.NavigationCoda, validation.NavigationBackward}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repeatRound1Score(t, []string{attributes + tc.body}, nil, func(t *testing.T, r validation.Report) {
				var rows []validation.ContextResult
				var traversal *validation.NavigationContextResult
				for _, row := range r.Context {
					if row.Navigation == nil {
						continue
					}
					if row.Navigation.Kind == validation.NavigationTraversal {
						traversal = row.Navigation
					} else {
						rows = append(rows, row)
					}
				}
				if traversal == nil {
					t.Fatal("missing traversal")
				}
				if tc.reason == "" {
					if traversal.Complete.State != validation.EvidenceKnown || !traversal.Complete.Value {
						t.Errorf("complete = %+v, want known true", traversal.Complete)
					}
				} else if traversal.Complete.State != validation.EvidenceUnknown || traversal.Complete.Reason != validation.Code(tc.reason) {
					t.Errorf("complete = %+v, want unknown %q", traversal.Complete, tc.reason)
				}
				if len(rows) != len(tc.kinds) {
					t.Fatalf("published rows = %d, want %d", len(rows), len(tc.kinds))
				}
				for i, row := range rows {
					if row.Navigation.Kind != tc.kinds[i] {
						t.Errorf("published kind[%d] = %s, want %s", i, row.Navigation.Kind, tc.kinds[i])
					}
					if row.Navigation.Kind == validation.NavigationSound && (row.Navigation.Guard.State != validation.EvidenceKnown || !row.Navigation.Guard.Value) {
						t.Errorf("unrestricted sound guard = %+v, want known true", row.Navigation.Guard)
					}
					if row.Navigation.Kind == validation.NavigationCoda && (row.Navigation.MarkerActive.State != validation.EvidenceKnown || row.Navigation.MarkerActive.Value) {
						t.Errorf("inactive coda = %+v", row.Navigation.MarkerActive)
					}
				}
				if len(traversal.Traversal) != len(rows) {
					t.Errorf("occurrences = %d, want %d", len(traversal.Traversal), len(rows))
					return
				}
				for i, occurrence := range traversal.Traversal {
					if occurrence.Source != rows[i].Subject {
						t.Errorf("occurrence[%d] = %+v, want %+v", i, occurrence.Source, rows[i].Subject)
					}
				}
			})
		})
	}
}

func TestRound3NavigationUnresolvedDalSegnoTarget(t *testing.T) {
	repeatRound1Score(t, []string{`<attributes><divisions>1</divisions></attributes>` + repeatR1Note + `<sound dalsegno="missing"/>`}, nil, func(t *testing.T, r validation.Report) {
		found := false
		for _, row := range r.Context {
			if row.Navigation != nil && row.Navigation.Kind == validation.NavigationDalSegno {
				v := row.Navigation
				if v.Target.State != validation.EvidenceUnknown || v.Target.Reason != "navigation-target-unresolved" || v.Guard.State != validation.EvidenceUnknown || v.Guard.Reason != "navigation-pass-unavailable" || !v.Defaulted || v.TimeOnly.State != validation.EvidenceKnown || v.TimeOnly.Value != "1" {
					t.Errorf("jump prerequisites = %+v", v)
				}
			}
			if row.Navigation != nil && row.Navigation.Kind == validation.NavigationTraversal {
				found = true
				if row.Navigation.Complete.State != validation.EvidenceUnknown || row.Navigation.Complete.Reason != "navigation-target-unresolved" {
					t.Errorf("complete = %+v, want target-unresolved", row.Navigation.Complete)
				}
				if len(row.Navigation.Traversal) != 2 {
					t.Fatalf("prefix occurrences = %d, want 2", len(row.Navigation.Traversal))
				}
				for _, occurrence := range row.Navigation.Traversal {
					if occurrence.Pass.State != validation.EvidenceUnknown || occurrence.Epoch != 0 {
						t.Errorf("unresolved D.S. inferred pass/epoch: %+v", occurrence)
					}
				}
			}
		}
		if !found {
			t.Fatal("missing traversal")
		}
	})
}

func TestRound3NavigationInterleavedControlPermutations(t *testing.T) {
	permutations := [][3]int{{0, 1, 2}, {0, 2, 1}, {1, 0, 2}, {1, 2, 0}, {2, 0, 1}, {2, 1, 0}}
	for _, jump := range []string{"dalsegno", "tocoda"} {
		for _, passive := range []struct {
			name, xml string
			kind      validation.NavigationKind
		}{
			{"sound", `<sound tempo="60"/>`, validation.NavigationSound},
			{"listening", `<listening><sync type="bar"/></listening>`, validation.NavigationListening},
			{"inactive-coda", `<barline location="middle" coda="C"/>`, validation.NavigationCoda},
		} {
			for i, order := range permutations {
				t.Run(fmt.Sprintf("%s/%s/%d", jump, passive.name, i), func(t *testing.T) {
					pieces := []string{`<sound ` + jump + `="missing"/>`, passive.xml, `<barline location="middle"><repeat direction="backward"/></barline>`}
					kinds := []validation.NavigationKind{validation.NavigationKind(jump), passive.kind, validation.NavigationBackward}
					body := `<attributes><divisions>1</divisions></attributes>` + repeatR1Note
					for _, n := range order {
						body += pieces[n]
					}
					repeatRound1Score(t, []string{body}, nil, func(t *testing.T, r validation.Report) {
						var rows []validation.ContextResult
						var traversal *validation.NavigationContextResult
						for _, row := range r.Context {
							if row.Navigation == nil {
								continue
							}
							if row.Navigation.Kind == validation.NavigationTraversal {
								traversal = row.Navigation
							} else {
								rows = append(rows, row)
							}
						}
						if traversal == nil {
							t.Fatal("missing traversal")
						}
						if traversal.Complete.State != validation.EvidenceUnknown || traversal.Complete.Reason != "navigation-simultaneous-control-unresolved" {
							t.Errorf("competing controls accepted: %+v", traversal.Complete)
						}
						if len(rows) != 4 {
							t.Fatalf("rows=%d, want 4", len(rows))
						}
						for j, n := range order {
							if rows[j+1].Navigation.Kind != kinds[n] {
								t.Errorf("stable row %d: kind %s, want %s", j+1, rows[j+1].Navigation.Kind, kinds[n])
							}
						}
						wantPrefix := 1
						if order[0] == 1 {
							wantPrefix++
						}
						if len(traversal.Traversal) != wantPrefix {
							t.Errorf("prefix=%d, want %d", len(traversal.Traversal), wantPrefix)
						}
						for j, occurrence := range traversal.Traversal {
							if occurrence.Source != rows[j].Subject {
								t.Errorf("prefix row %d out of source order", j)
							}
						}
					})
				})
			}
		}
	}
}

func TestRound3NavigationPassiveRowsPreserveExplicitRepeat(t *testing.T) {
	repeatRound1Score(t, []string{`<attributes><divisions>1</divisions></attributes><barline location="left"><repeat direction="forward"/></barline>` + repeatR1Note + `<sound tempo="60"/><barline coda="C"><repeat direction="backward" times="2"/></barline>`}, nil, func(t *testing.T, r validation.Report) {
		var rows []validation.ContextResult
		var traversal *validation.NavigationContextResult
		for _, row := range r.Context {
			if row.Navigation == nil {
				continue
			}
			if row.Navigation.Kind == validation.NavigationTraversal {
				traversal = row.Navigation
			} else {
				rows = append(rows, row)
			}
		}
		if traversal == nil {
			t.Fatal("missing traversal")
		}
		if traversal.Complete.State != validation.EvidenceKnown || !traversal.Complete.Value {
			t.Errorf("complete=%+v", traversal.Complete)
		}
		if len(rows) != 5 || len(traversal.Traversal) != 10 {
			t.Fatalf("rows=%d occurrences=%d", len(rows), len(traversal.Traversal))
		}
		for i, occurrence := range traversal.Traversal {
			if occurrence.Source != rows[i%len(rows)].Subject || occurrence.Epoch != 0 {
				t.Errorf("unexpected repeat occurrence %d: %+v", i, occurrence)
			}
			if len(occurrence.Frames) != 1 || occurrence.Frames[0].Pass.String() != fmt.Sprint(i/len(rows)+1) {
				t.Errorf("repeat frame %d: %+v", i, occurrence.Frames)
			}
		}
	})
}

func TestRound3NavigationSoundGuardAndMixedDirectives(t *testing.T) {
	for _, tc := range []struct {
		name, attrs string
		state       validation.EvidenceState
		reason      validation.Code
		cause       bool
	}{
		{"unrestricted", `tempo="120"`, validation.EvidenceKnown, "", false},
		{"restricted", `tempo="120" time-only="2"`, validation.EvidenceUnknown, "navigation-pass-unavailable", false},
		{"invalid-selector", `tempo="120" time-only="bad"`, validation.EvidenceInvalid, "", true},
		{"default-jump", `dalsegno="missing"`, validation.EvidenceUnknown, "navigation-pass-unavailable", false},
		{"mixed-directives", `dacapo="yes" dalsegno="missing"`, validation.EvidenceUnknown, "navigation-pass-unavailable", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repeatRound1Score(t, []string{`<attributes><divisions>1</divisions></attributes>` + repeatR1Note + `<sound ` + tc.attrs + `/>`}, nil, func(t *testing.T, r validation.Report) {
				found := false
				for _, row := range r.Context {
					if row.Navigation == nil {
						continue
					}
					v := row.Navigation
					if v.Kind == validation.NavigationSound || v.Kind == validation.NavigationDalSegno || v.Kind == validation.NavigationDaCapo {
						found = true
						if tc.cause && (v.TimeOnly.Cause == nil || v.Guard.Cause == nil || *v.TimeOnly.Cause != *v.Guard.Cause || v.TimeOnly.Reason != v.Guard.Reason) {
							t.Errorf("guard did not preserve selector cause: guard=%+v selector=%+v", v.Guard, v.TimeOnly)
						}
						if v.Guard.State != tc.state || tc.reason != "" && v.Guard.Reason != tc.reason || tc.cause && v.Guard.Cause == nil || tc.state == validation.EvidenceKnown && !v.Guard.Value {
							t.Errorf("guard=%+v", v.Guard)
						}
					}
					if tc.name == "mixed-directives" && v.Kind == validation.NavigationTraversal && (v.Complete.State != validation.EvidenceUnknown || v.Complete.Reason != "navigation-multiple-directives-unresolved") {
						t.Errorf("mixed directives changed policy: %+v", v.Complete)
					}
				}
				if !found {
					t.Error("missing sound row")
				}
			})
		})
	}
}

func TestRound3NavigationEqualPositionSoundUpdatesRetainOrder(t *testing.T) {
	repeatRound1Score(t, []string{`<attributes><divisions>1</divisions></attributes>` + repeatR1Note + `<sound tempo="80"/><sound tempo="90"/>` + repeatR1Note}, nil, func(t *testing.T, r validation.Report) {
		found := false
		for _, row := range r.Context {
			if row.Navigation == nil || row.Navigation.Kind != validation.NavigationTraversal {
				continue
			}
			found = true
			v := row.Navigation
			if v.Complete.State != validation.EvidenceKnown || !v.Complete.Value || len(v.Traversal) != 4 {
				t.Fatalf("traversal=%+v", v)
			}
			for i, want := range []string{"80", "90", "90"} {
				tempo := v.Traversal[i+1].Tempo
				if tempo.State != validation.EvidenceKnown || tempo.Value.String() != want {
					t.Errorf("occurrence %d tempo=%+v, want %s", i+1, tempo, want)
				}
			}
		}
		if !found {
			t.Error("missing traversal")
		}
	})
}

func TestRound3NavigationFalseToCodaSkipsMissingTarget(t *testing.T) {
	repeatRound1Score(t, []string{`<attributes><divisions>1</divisions></attributes>` + repeatR1Note + `<sound tocoda="missing"/>`}, nil, func(t *testing.T, r validation.Report) {
		target, traversal := false, false
		for _, row := range r.Context {
			if row.Navigation == nil {
				continue
			}
			v := row.Navigation
			if v.Kind == validation.NavigationToCoda {
				target = true
				if v.Target.State != validation.EvidenceUnknown || v.Target.Reason != "navigation-target-unresolved" {
					t.Errorf("target=%+v", v.Target)
				}
			}
			if v.Kind == validation.NavigationTraversal {
				traversal = true
				if v.Complete.State != validation.EvidenceKnown || !v.Complete.Value || len(v.Traversal) != 2 {
					t.Fatalf("skipped jump=%+v", v)
				}
				played := v.Traversal[1].Played
				if played.State != validation.EvidenceKnown || played.Value {
					t.Errorf("second-pass jump applied on first pass: %+v", played)
				}
			}
		}
		if !target || !traversal {
			t.Errorf("missing target=%v traversal=%v", target, traversal)
		}
	})
}
