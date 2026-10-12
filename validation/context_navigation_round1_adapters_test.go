package validation_test

import (
	"github.com/go-muse/go-musicxml/validation"
	"reflect"
	"testing"
)

func TestRound1NavigationDaCapoInsideRepeatTerminates(t *testing.T) {
	repeatRound1Score(t, []string{`<attributes><divisions>1</divisions></attributes><barline location="left"><repeat direction="forward"/></barline>` + repeatR1Note, `<direction><direction-type><words>D.C.</words></direction-type><sound dacapo="yes"/></direction>` + repeatR1Note + `<barline><repeat direction="backward"/></barline>`}, nil, func(t *testing.T, r validation.Report) {
		for _, c := range r.Context {
			if c.Navigation != nil && c.Navigation.Kind == validation.NavigationTraversal {
				if c.Navigation.Complete.State != validation.EvidenceUnknown {
					t.Error("ambiguous jump must remain unknown")
				}
				if len(c.Navigation.Traversal) > 8 {
					t.Error("unbounded jump prefix")
				}
			}
		}
	})
}
func TestRound1NavigationBarlineGraphicAndEndingGuard(t *testing.T) {
	repeatAssessBoth(t, `<attributes><divisions>1</divisions></attributes><barline location="left" segno="S"><segno/><ending number="1" type="start"/></barline>`+repeatR1Note+`<barline><ending number="1" type="stop"/><repeat direction="backward"/></barline>`, func(t *testing.T, r validation.Report) {
		for _, c := range r.Context {
			if c.Navigation == nil {
				continue
			}
			v := c.Navigation
			if v.Kind == validation.NavigationGraphicSegno {
				t.Error("barline glyph got direction sound advice")
			}
			if v.Kind == validation.NavigationEnding && v.Guard.State == "" {
				t.Error("zero ending guard")
			}
			if v.Kind == validation.NavigationTraversal && v.InitialSyncMode.State == "" {
				t.Error("zero initial sync")
			}
			if v.Complete.Reason == "navigation-simultaneous-control-unresolved" {
				t.Error("ending stop and repeat order unresolved")
			}
		}
	})
}

func TestRound1PublicRepeatNavigationEvidenceStates(t *testing.T) {
	repeatRound1Score(t, []string{`<attributes><divisions>1</divisions></attributes><barline location="left"><repeat direction="forward"/></barline>` + repeatR1Note + `<barline><repeat direction="backward" times="2"/></barline>`, repeatR1Start + repeatR1Note, repeatR1Stop}, nil, func(t *testing.T, r validation.Report) {
		var walk func(reflect.Value, string)
		walk = func(v reflect.Value, path string) {
			if !v.IsValid() {
				return
			}
			if v.Kind() == reflect.Pointer {
				if !v.IsNil() {
					walk(v.Elem(), path)
				}
				return
			}
			if v.Kind() == reflect.Slice {
				for i := 0; i < v.Len(); i++ {
					walk(v.Index(i), path)
				}
				return
			}
			if v.Kind() != reflect.Struct {
				return
			}
			if state := v.FieldByName("State"); state.IsValid() && state.Type() == reflect.TypeOf(validation.EvidenceState("")) {
				if state.String() == "" {
					t.Errorf("zero evidence at %s", path)
				}
				if state.String() == string(validation.EvidenceKnown) {
					walk(v.FieldByName("Value"), path+".Value")
				}
				return
			}
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					walk(v.Field(i), path+"."+v.Type().Field(i).Name)
				}
			}
		}
		for _, c := range r.Context {
			if c.Repeat != nil {
				walk(reflect.ValueOf(c.Repeat), "Repeat")
			}
			if c.Navigation != nil {
				walk(reflect.ValueOf(c.Navigation), "Navigation")
			}
		}
	})
}
func TestRound1NavigationOrdinaryPrefixBeforeEnding(t *testing.T) {
	repeatRound1Score(t, []string{`<attributes><divisions>1</divisions></attributes><barline location="left"><repeat direction="forward"/></barline>` + repeatR1Note + `<barline><repeat direction="backward" times="2"/></barline>`, `<barline location="left"><repeat direction="forward"/></barline>` + repeatR1Note, `<barline location="left"><ending number="1" type="start"/></barline>` + repeatR1Note + `<barline><ending number="1" type="stop"/><repeat direction="backward"/></barline>`}, nil, func(t *testing.T, r validation.Report) {
		for _, c := range r.Context {
			if c.Navigation != nil && c.Navigation.Kind == validation.NavigationTraversal {
				if len(c.Navigation.Traversal) < 7 || c.Navigation.Complete.State != validation.EvidenceUnknown {
					t.Errorf("lost proved repeat prefix: %+v", c.Navigation)
				}
			}
		}
	})
}
