package validation

import (
	"fmt"
	"reflect"
	"runtime"
	"testing"
)

// Keep the straightforward constructor as an independent fixture oracle and
// benchmark baseline. This is test setup, never assessment work or a quota.
func stateBindingsReference() *timelineBindings {
	b := &timelineBindings{}
	for role := timelineRoleInvalid + 1; role < timelineRoleLimit; role++ {
		b.Roles[role] = []SchemaUse{{Declaration: DeclarationID{Occurrence: string(rune(1000 + role))}}}
	}
	for _, role := range []timelineRole{timelineRoleKeyNumber, timelineRoleTimeNumber, timelineRoleClefNumber, timelineRoleTransposeNumber, timelineRoleForPartNumber, timelineRoleClefAdditional, timelineRoleClefSize, timelineRoleClefAfterBarline, timelineRoleDoubleAbove, timelineRoleKeyOctaveNumber, timelineRoleKeyOctaveCancel, timelineRoleCancelLocation, timelineRoleKeyAccidentalSMuFL, timelineRoleTimeSymbol, timelineRoleTimeSeparator, timelineRoleInterchangeableSymbol, timelineRoleInterchangeableSeparator, timelineRoleInstrumentID, timelineRoleInstrumentChangeID} {
		b.Roles[role][0].Attribute = true
		b.Roles[role][0].Name = QName{Local: string(rune(1000 + role))}
	}
	for _, shape := range timelineRoleShapes {
		if shape.attribute {
			b.Roles[shape.role][0].Attribute = true
			b.Roles[shape.role][0].Name = QName{Local: string(rune(1000 + shape.role))}
		}
	}
	return b
}

func TestTimelineStateBindingsFixtureKeepsOwnedRoles(t *testing.T) {
	want := stateBindingsReference()
	for worker := range 4 {
		t.Run(fmt.Sprintf("worker%d", worker), func(t *testing.T) {
			t.Parallel()
			got, untouched := stateBindings(), stateBindings()
			if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(untouched, want) {
				t.Fatal("fixture descriptors differ from independent construction")
			}
			for role := timelineRoleInvalid + 1; role < timelineRoleLimit; role++ {
				if len(got.Roles[role]) != 1 || cap(got.Roles[role]) != 1 {
					t.Fatalf("role%d no longer owns one isolated descriptor", role)
				}
				got.Roles[role][0].Declaration.Occurrence = "caller-mutated"
				got.Roles[role][0].Name.Local = "caller-mutated"
				got.Roles[role][0].Attribute = !got.Roles[role][0].Attribute
				got.Roles[role] = append(got.Roles[role], SchemaUse{})
				got.Roles[role][1].Declaration.Occurrence = "appended"
				if role+1 < timelineRoleLimit && !reflect.DeepEqual(got.Roles[role+1], want.Roles[role+1]) {
					t.Fatalf("role%d append mutated the next role", role)
				}
			}
			if !reflect.DeepEqual(untouched, want) || !reflect.DeepEqual(stateBindings(), want) {
				t.Fatal("caller mutation escaped into another fixture")
			}
		})
	}
}

func TestTimelineStateBindingsFixtureAllocationBudget(t *testing.T) {
	// Serial because AllocsPerRun observes process-wide allocation counters.
	var fixture *timelineBindings
	allocations := testing.AllocsPerRun(20, func() {
		fixture = stateBindings()
	})
	runtime.KeepAlive(fixture)
	if allocations > 2 {
		t.Fatalf("immutable fixture setup made %.0f allocations; want at most 2 owned backing allocations", allocations)
	}
}

func BenchmarkTimelineStateBindingsFixture(b *testing.B) {
	for _, tc := range []struct {
		name  string
		build func() *timelineBindings
	}{{"reference", stateBindingsReference}, {"owned", stateBindings}} {
		b.Run(tc.name, func(b *testing.B) {
			var fixture *timelineBindings
			b.ReportAllocs()
			for b.Loop() {
				fixture = tc.build()
			}
			runtime.KeepAlive(fixture)
		})
	}
}
