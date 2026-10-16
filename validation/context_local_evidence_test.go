package validation

import (
	"context"
	"testing"
)

func localEvidenceFixture(t *testing.T, state EvidenceState, role contextRole) (*Session, *factNode, *contextReader) {
	t.Helper()
	s := rationalSession(t, Budgets{})
	s.documentID = "doc"
	n := localNode(2, 1, "subject")
	n.header.Type = testEvidence(Evidence[ComponentID]{State: EvidenceKnown, Value: contextNamedType("typed-text")})
	n.end.Scalar.Value = testEvidence(Evidence[Atomic]{State: state, Reason: "component-unavailable", Cause: &DecisionRef{Rule: RuleKey{"source-prerequisite", 1}, Subject: NodeRef{"doc", 2}}})
	s.nodes = map[NodeID]*factNode{2: n}
	r := &contextReader{ctx: context.Background(), s: s, b: &contextBindings{Roles: map[contextRole][]SchemaUse{role: {{Declaration: n.header.Declaration.Value}}}, Attributes: map[contextRole]map[ComponentID]AttributeSelector{}}}
	return s, n, r
}
func localRequireEvidence(t *testing.T, s *Session, p ContextProperty, state EvidenceState, app bool) {
	t.Helper()
	count := 0
	for _, row := range s.contextResults {
		if row.Property != p {
			continue
		}
		count++
		got := row.Value.State
		cause := row.Value.Cause
		if app {
			got = row.Layout.Applicability.State
			cause = row.Layout.Applicability.Cause
		}
		if got != state || cause == nil || cause.Rule.ID != "source-prerequisite" {
			t.Fatalf("%s state/cause replaced %+v", p, row)
		}
	}
	if count != 1 {
		t.Fatalf("%s rows%d", p, count)
	}
}
func TestLocalStructuralUnavailableScalarMatrix(t *testing.T) {
	cases := []struct {
		role     contextRole
		property ContextProperty
	}{{localMode, ContextMinorMode}, {localNumeralMode, ContextMinorMode}, {localKind, ContextHarmonyActive}, {localEnsemble, ContextEnsembleSize}, {localText, ContextLyricLiteralText}, {localLine, ContextStaffLineBasis}, {localPitchOctave, ContextOctaveBasis}, {localAlter, ContextAlterSemitones}, {localFigureNumber, ContextFigureNumber}, {localClefSign, ContextDeprecatedNotation}}
	for _, c := range cases {
		for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
			t.Run(string(c.property)+"/"+string(state), func(t *testing.T) {
				s, n, r := localEvidenceFixture(t, state, c.role)
				defer s.cleanup()
				if err := localStructuralNode(r.ctx, s, RuleKey{"local-test", 1}, n, r); err != nil {
					t.Fatal(err)
				}
				localRequireEvidence(t, s, c.property, state, false)
			})
		}
	}
}
func TestLocalStructuralUnavailableAttributeMatrix(t *testing.T) {
	cases := []struct {
		element, attribute contextRole
		name               string
		property           ContextProperty
	}{{"", localColor, "color", ContextColorRGB}, {"", localColor, "color", ContextColorAlpha}, {"", localFontFamily, "font-family", ContextFontFamilyOrder}, {localCreator, localMetadataType, "type", ContextMetadataRole}, {localDistance, localDistanceType, "type", ContextDistanceType}, {localLineWidth, localLineWidthType, "type", ContextLineWidthType}}
	for _, c := range cases {
		for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
			t.Run(string(c.property)+"/"+string(state), func(t *testing.T) {
				s, n, r := localEvidenceFixture(t, state, c.element)
				defer s.cleanup()
				a := localAttr(c.name)
				r.b.Attributes[c.attribute] = map[ComponentID]AttributeSelector{n.header.Type.Value: a}
				localSetAttr(n, c.name, localString("unused"))
				n.attributes[0].Scalar.Value = *n.end.Scalar.Value
				if err := localStructuralNode(r.ctx, s, RuleKey{"local-test", 1}, n, r); err != nil {
					t.Fatal(err)
				}
				localRequireEvidence(t, s, c.property, state, false)
			})
		}
	}
}
func TestLocalAdviceUnavailableFactStream(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		for _, c := range []struct {
			role     contextRole
			property ContextProperty
		}{{localAdvisoryRole("duration"), "local.advisory-integral-divisions"}, {localClefSign, "local.advisory-deprecated-clef"}, {localText, "local.advisory-lyric-literal-hyphen"}} {
			s, n, r := localEvidenceFixture(t, state, c.role)
			if err := localAdvisoryNode(r.ctx, s, RuleKey{"local-test", 1}, n, r); err != nil {
				t.Fatal(err)
			}
			localRequireEvidence(t, s, c.property, state, true)
			s.cleanup()
		}
	}
}
