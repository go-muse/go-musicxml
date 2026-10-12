package validation

import (
	"context"
	"errors"
	"testing"
)

func spanUnitReader(t *testing.T) *contextReader {
	t.Helper()
	s := rationalSession(t, Budgets{})
	t.Cleanup(s.cleanup)
	s.documentID = "span-unit"
	s.nodes = map[NodeID]*factNode{}
	b := &contextBindings{Roles: map[contextRole][]SchemaUse{}, Attributes: map[contextRole]map[ComponentID]AttributeSelector{}, AttributeUses: map[contextRole]map[DeclarationID]AttributeSelector{}}
	for _, spec := range contextSpanBindingSpecs() {
		b.Roles[spec.Role] = []SchemaUse{{Name: QName{Local: spec.Name}, Declaration: localDecl(string(spec.Role)), OwnerType: contextNamedType(spec.Owner), OwnerTypeKnown: true, DeclaredType: contextNamedType(spec.Type), DeclaredTypeKnown: true, Attribute: spec.Attribute}}
		if spec.Attribute {
			u := b.Roles[spec.Role][0]
			a := AttributeSelector{Name: u.Name, Declaration: u.Declaration}
			b.Attributes[spec.Role] = map[ComponentID]AttributeSelector{u.OwnerType: a}
			b.AttributeUses[spec.Role] = map[DeclarationID]AttributeSelector{u.Declaration: a}
		}

	}
	return &contextReader{ctx: context.Background(), s: s, b: b}
}
func spanUnitNode(r *contextReader, role contextRole, parent *factNode) *factNode {
	id := NodeID(len(r.s.sequence) + 1)
	pid := NodeID(0)
	if parent != nil {
		pid = parent.header.ID
	}
	u := r.b.Roles[role][0]
	n := localNode(id, pid, string(role))
	n.header.Declaration.Value = u.Declaration
	n.header.Type = Evidence[ComponentID]{State: EvidenceKnown, Value: u.DeclaredType}
	n.header.Location.DocumentID = r.s.documentID
	n.header.Location.Target = TargetSource
	r.s.nodes[id] = n
	r.s.sequence = append(r.s.sequence, id)
	if parent != nil {
		parent.children = append(parent.children, n)
	} else {
		r.s.root = n
	}
	return n
}
func spanUnitAttribute(r *contextReader, n *factNode, role contextRole, v Evidence[Atomic]) {
	u := r.b.Roles[role][0]
	n.attributes = append(n.attributes, AttributeFact{Owner: n.header.ID, Name: u.Name, Declaration: Evidence[DeclarationID]{State: EvidenceKnown, Value: u.Declaration}, Presence: Evidence[bool]{State: EvidenceKnown, Value: true}, Scalar: Scalar{Value: v}, Location: n.header.Location})
}
func spanUnitValue(t *testing.T, r *contextReader, p ContextProperty) Evidence[Atomic] {
	t.Helper()
	if r.err != nil {
		t.Fatal(r.err)
	}
	for _, v := range r.s.contextResults {
		if v.Property == p {
			return v.Value
		}
	}
	t.Fatalf("no property %s", p)
	return Evidence[Atomic]{}
}
func TestContextSpanDefaultFieldsFiveAxes(t *testing.T) {
	for _, tc := range []struct {
		name            string
		node, attribute contextRole
		family          SpanFamily
		property        ContextProperty
		def, override   Evidence[Atomic]
	}{
		{"octave-size", spanRole_octaveShift, spanRole_octave_shift_size, SpanOctaveShift, ContextPropertySpanSize, spanInteger(8), spanInteger(15)},
		{"bracket-line", spanRole_bracket, spanRole_bracket_line_type, SpanBracket, ContextPropertySpanLineType, spanString("solid"), spanString("dashed")},
		{"glissando-line", spanRole_glissando, spanRole_glissando_line_type, SpanGlissando, ContextPropertySpanLineType, spanString("wavy"), spanString("solid")},
		{"slide-line", spanRole_slide, spanRole_slide_line_type, SpanSlide, ContextPropertySpanLineType, spanString("solid"), spanString("wavy")},
		{"beam-fan", spanRole_beam, spanRole_beam_fan, SpanBeam, ContextPropertySpanFan, spanString("none"), spanString("accel")},
		{"tuplet-shape", spanRole_tuplet, spanRole_tuplet_line_shape, SpanTuplet, ContextPropertySpanLineShape, spanString("straight"), spanString("curved")},
		{"tuplet-number-display", spanRole_tuplet, spanRole_tuplet_show_number, SpanTuplet, ContextPropertySpanTupletShowNumber, spanString("actual"), spanString("both")},
		{"tuplet-type-display", spanRole_tuplet, spanRole_tuplet_show_type, SpanTuplet, ContextPropertySpanTupletShowType, spanString("none"), spanString("actual")},
		{"arpeggio-direction", spanRole_arpeggiate, spanRole_arpeggiate_arpeggio_direction, SpanArpeggiate, ContextPropertySpanArpeggioDirection, spanString("up"), spanString("down")},
		{"wedge-niente", spanRole_wedge, spanRole_wedge_niente, SpanWedge, ContextPropertySpanWedgeNiente, spanString("no"), spanString("yes")},
		{"pedal-line", spanRole_pedal, spanRole_pedal_line, SpanPedal, ContextPropertySpanPedalLine, spanString("no"), spanString("yes")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, axis := range []string{"positive", "negative", "boundary", "missing-fact", "interaction"} {
				t.Run(axis, func(t *testing.T) {
					r := spanUnitReader(t)
					note := spanUnitNode(r, spanRole_note, nil)
					n := spanUnitNode(r, tc.node, note)
					want := tc.def
					cause := &DecisionRef{Rule: RuleKey{"scalar", 1}, Subject: NodeRef{r.s.documentID, n.header.ID}}
					switch axis {
					case "negative":
						want = Evidence[Atomic]{State: EvidenceInvalid, Reason: "test-scalar-invalid", Cause: cause}
						spanUnitAttribute(r, n, tc.attribute, want)
					case "boundary":
						want = tc.override
						spanUnitAttribute(r, n, tc.attribute, want)
					case "missing-fact":
						n.end.AttributesComplete = Evidence[bool]{State: EvidenceUnknown, Reason: "test-attributes-gap"}
						want = stateUnknown[Atomic]("test-attributes-gap")
					case "interaction":
						other := spanUnitReader(t)
						on := spanUnitNode(other, tc.node, nil)
						spanUnitAttribute(other, on, tc.attribute, tc.override)
						e := spanTestEndpoint(t, 1, tc.family, "start", 1)
						spanLocalFacts(other, RuleKey{"spans", 1}, on, nil, e)
					}
					e := spanTestEndpoint(t, n.header.ID, tc.family, "start", 1)
					spanLocalFacts(r, RuleKey{"spans", 1}, n, note, e)
					got := spanUnitValue(t, r, tc.property)
					if got.State != want.State {
						t.Fatalf("%+v want %+v", got, want)
					}
					if got.State == EvidenceKnown {
						if got.Value.Kind != want.Value.Kind || got.Value.Text != want.Value.Text || got.Value.Integer.String() != want.Value.Integer.String() {
							t.Fatalf("%+v want %+v", got, want)
						}
					}
					if axis == "negative" && (got.Cause == nil || *got.Cause != *cause) {
						t.Fatal("lost cause")
					}
				})
			}
		})
	}
}
func TestContextSpanTupletFallbackIndependentFields(t *testing.T) {
	for _, axis := range []string{"fallback", "override-number-only", "normal-dot-separate", "invalid-actual", "missing-modification", "missing-normal-type"} {
		t.Run(axis, func(t *testing.T) {
			r := spanUnitReader(t)
			note := spanUnitNode(r, spanRole_note, nil)
			typ := spanUnitNode(r, spanRole_noteType, note)
			typ.end.Scalar.Value = spanString("eighth")
			spanUnitNode(r, spanRole_noteDot, note)
			var mod *factNode
			if axis != "missing-modification" {
				mod = spanUnitNode(r, spanRole_noteModification, note)
				a := spanUnitNode(r, spanRole_actualNotes, mod)
				a.end.Scalar.Value = spanInteger(3)
				b := spanUnitNode(r, spanRole_normalNotes, mod)
				b.end.Scalar.Value = spanInteger(2)
				if axis != "missing-normal-type" {
					nt := spanUnitNode(r, spanRole_normalType, mod)
					nt.end.Scalar.Value = spanString("quarter")
				}
				if axis == "normal-dot-separate" {
					spanUnitNode(r, spanRole_normalDot, mod)
				}
				if axis == "invalid-actual" {
					a.end.Scalar.Value = Evidence[Atomic]{State: EvidenceInvalid, Reason: "invalid-actual", Cause: &DecisionRef{Rule: RuleKey{"scalar", 1}, Subject: NodeRef{r.s.documentID, a.header.ID}}}
				}
			}
			n := spanUnitNode(r, spanRole_tuplet, note)
			if axis == "override-number-only" {
				portion := spanUnitNode(r, spanRole_tupletActual, n)
				num := spanUnitNode(r, spanRole_tupletNumber, portion)
				num.end.Scalar.Value = spanInteger(5)
			}
			spanTupletFacts(r, RuleKey{"spans", 1}, n, note)
			a := spanUnitValue(t, r, ContextPropertySpanTupletActualNumber)
			nt := spanUnitValue(t, r, ContextPropertySpanTupletNormalType)
			nd := spanUnitValue(t, r, ContextPropertySpanTupletNormalDots)
			ad := spanUnitValue(t, r, ContextPropertySpanTupletActualDots)
			if ad.State != EvidenceKnown || ad.Value.Integer.String() != "1" {
				t.Fatalf("actual dots %+v", ad)
			}
			switch axis {
			case "invalid-actual":
				if a.State != EvidenceInvalid || a.Cause == nil {
					t.Fatal(a)
				}
			case "missing-modification":
				if a.State != EvidenceUnknown || nt.State != EvidenceUnknown {
					t.Fatalf("invented fallback %+v %+v", a, nt)
				}
			case "override-number-only":
				if a.Value.Integer.String() != "5" || nt.Value.Text != "quarter" {
					t.Fatalf("partial override %+v %+v", a, nt)
				}
			case "normal-dot-separate":
				if nd.Value.Integer.String() != "1" {
					t.Fatal(nd)
				}
			case "missing-normal-type":
				if nt.Value.Text != "eighth" {
					t.Fatal(nt)
				}
			default:
				if a.Value.Integer.String() != "3" || nd.Value.Integer.String() != "0" {
					t.Fatalf("fallback %+v %+v", a, nd)
				}
			}
		})
	}
}
func TestContextSpanResourcesCancellationAndOwnership(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	if err := associateSpanEndpoints(ctx, s, nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if s.retained != 0 {
		t.Fatal("allocated before cancellation")
	}
	tiny := rationalSession(t, Budgets{MaxBufferedBytes: 1})
	defer tiny.cleanup()
	if _, err := cloneSpanContext(context.Background(), tiny, &SpanContextResult{}); !errors.Is(err, ErrResourceLimit) {
		t.Fatal(err)
	}
	if tiny.retained != 0 {
		t.Fatal("allocated before admission")
	}
	r := spanUnitReader(t)
	root := spanUnitNode(r, spanRole_partList, nil)
	a := spanUnitNode(r, spanRole_partGroup, root)
	spanUnitAttribute(r, a, spanRole_part_group_type, spanString("start"))
	b := spanUnitNode(r, spanRole_partGroup, root)
	spanUnitAttribute(r, b, spanRole_part_group_type, spanString("stop"))
	p := &contextPlan{Bindings: r.b}
	key := RuleKey{"spans", 1}
	if _, _, err := evaluateContextSpans(context.Background(), r.s, root, key, p, true); err != nil {
		t.Fatal(err)
	}
	count, retained := len(r.s.contextResults), r.s.retained
	if _, _, err := evaluateContextSpans(context.Background(), r.s, root, key, p, true); err != nil {
		t.Fatal(err)
	}
	if len(r.s.contextResults) != count || r.s.retained != retained {
		t.Fatal("repeated query rebuilt service")
	}
}
