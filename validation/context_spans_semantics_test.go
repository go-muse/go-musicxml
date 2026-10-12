package validation

import (
	"context"
	"errors"
	"math/big"
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
		{"tremolo-smufl", spanRole_tremolo, spanRole_tremoloSMuFL, SpanTremolo, ContextPropertySpanTremoloSMuFL, spanString("buzzRoll"), spanString("unvalidatedGlyph")},
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
					if tc.family == SpanTremolo {
						e.payload.Role.Value = "unmeasured"
					}
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
				if nd.State != EvidenceUnknown {
					t.Fatalf("unproved normal-dot fallback: %+v", nd)
				}
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

func TestContextSpanEndpointAvailabilityByFamily(t *testing.T) {
	for _, binding := range spanFamilies {
		binding := binding
		t.Run(string(binding.node), func(t *testing.T) {
			for _, axis := range []string{"positive", "invalid-role", "missing-role-capability", "default-number", "explicit-number", "invalid-number"} {
				t.Run(axis, func(t *testing.T) {
					r := spanUnitReader(t)
					note := spanUnitNode(r, spanRole_note, nil)
					n := spanUnitNode(r, binding.node, note)
					role := "start"
					switch binding.family {
					case SpanBeam:
						role = "begin"
					case SpanOctaveShift:
						role = "down"
					case SpanWedge:
						role = "crescendo"
					case SpanNonArpeggiate:
						role = "top"
					}
					roleValue := spanString(role)
					cause := &DecisionRef{Rule: RuleKey{"shape", 1}, Subject: NodeRef{r.s.documentID, n.header.ID}}
					if axis == "invalid-role" {
						roleValue = Evidence[Atomic]{State: EvidenceInvalid, Reason: "test-role-invalid", Cause: cause}
					}
					if axis == "missing-role-capability" {
						roleValue = Evidence[Atomic]{State: EvidenceUnsupported, Reason: "test-role-unavailable"}
					}
					if binding.role != "" {
						spanUnitAttribute(r, n, binding.role, roleValue)
					} else if binding.family == SpanBeam {
						n.end.Scalar.Value = roleValue
					}
					if binding.number != "" && (axis == "explicit-number" || axis == "invalid-number") {
						v := spanInteger(2)
						if binding.family == SpanGrouping || binding.family == SpanPartGroup {
							v = spanString("named")
						}
						if axis == "invalid-number" {
							v = Evidence[Atomic]{State: EvidenceInvalid, Reason: "test-number-invalid", Cause: cause}
						}
						spanUnitAttribute(r, n, binding.number, v)
					}
					domains := map[NodeID]Evidence[NodeRef]{note.header.ID: {State: EvidenceKnown, Value: NodeRef{r.s.documentID, 100}}}
					e := spanExtract(r, RuleKey{"spans", 1}, n, binding, nil, domains, 1, nil)
					if r.err != nil {
						t.Fatal(r.err)
					}
					if binding.role != "" || binding.family == SpanBeam {
						switch axis {
						case "invalid-role":
							if e.payload.Role.State != EvidenceInvalid || e.payload.Role.Cause == nil || *e.payload.Role.Cause != *cause {
								t.Fatal(e.payload.Role)
							}
						case "missing-role-capability":
							if e.payload.Role.State != EvidenceUnsupported {
								t.Fatal(e.payload.Role)
							}
						default:
							if e.payload.Role.State != EvidenceKnown || e.payload.Role.Value != role {
								t.Fatal(e.payload.Role)
							}
						}
					}
					if binding.number != "" {
						switch axis {
						case "invalid-number":
							if e.payload.Number.State != EvidenceInvalid || e.payload.Association.State != EvidenceInvalid || e.payload.Association.Cause == nil {
								t.Fatalf("invalid number lost %+v", e.payload)
							}
						case "default-number":
							if binding.family == SpanTied {
								if e.payload.Identity != SpanIdentityImpliedTie || e.payload.Number.State != EvidenceAbsent {
									t.Fatal(e.payload)
								}
							} else if e.payload.Number.State != EvidenceKnown {
								t.Fatal(e.payload.Number)
							}
						case "explicit-number":
							if e.payload.Number.State != EvidenceKnown || !e.payload.NumberExplicit.Value {
								t.Fatal(e.payload)
							}
						}
					}
				})
			}
		})
	}
}
func TestContextSpanUnmatchedAndUnknownPositionAreNotDefects(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	es := []*spanEndpoint{spanTestEndpoint(t, 1, SpanSlur, "start", 1), spanTestEndpoint(t, 2, SpanSlur, "stop", 1), spanTestEndpoint(t, 3, SpanSlur, "start", 2)}
	for _, e := range es {
		e.payload.Position = stateUnknown[SpanPosition]("unknown-cursor")
	}
	if err := associateSpanEndpoints(context.Background(), s, es, &timelineService{}); err != nil {
		t.Fatal(err)
	}
	if es[0].payload.Association.State != EvidenceKnown || es[1].payload.Association.State != EvidenceKnown {
		t.Fatal("unknown chronology erased document association")
	}
	if es[2].payload.Association.State != EvidenceUnknown {
		t.Fatal("unmatched endpoint fabricated closure")
	}
}

func TestContextSpanOctaveDirectionMeaning(t *testing.T) {
	for _, role := range []string{"up", "down", "stop", "continue"} {
		t.Run(role, func(t *testing.T) {
			r := spanUnitReader(t)
			n := spanUnitNode(r, spanRole_octaveShift, nil)
			e := spanTestEndpoint(t, n.header.ID, SpanOctaveShift, role, 1)
			spanLocalFacts(r, RuleKey{"spans", 1}, n, nil, e)
			got := spanUnitValue(t, r, ContextPropertySpanEndpointMeaning)
			if role == "up" || role == "down" {
				if got.State != EvidenceKnown || got.Value.Text != "display-"+role+"-from-note-pitch" {
					t.Fatal(got)
				}
			} else if got.State != EvidenceUnknown {
				t.Fatal(got)
			}
		})
	}
}

func TestContextSpanVerticalIdentityGapBlocksCertainty(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	a := spanTestEndpoint(t, 1, SpanArpeggiate, "", 1)
	b := spanTestEndpoint(t, 2, SpanArpeggiate, "", 1)
	z, _ := NewRational(big.NewRat(0, 1))
	for _, e := range []*spanEndpoint{a, b} {
		e.payload.Position = Evidence[SpanPosition]{State: EvidenceKnown, Value: SpanPosition{Part: NodeRef{"d", 10}, Measure: NodeRef{"d", 20}, Offset: z}}
	}
	b.payload.Number = Evidence[Atomic]{State: EvidenceInvalid, Reason: "bad-number", Cause: &DecisionRef{Rule: RuleKey{"scalar", 1}, Subject: b.ref}}
	if err := spanVerticalMembership(context.Background(), s, []*spanEndpoint{a, b}); err != nil {
		t.Fatal(err)
	}
	if a.payload.Association.State != EvidenceInvalid || a.payload.Association.Cause == nil {
		t.Fatal(a.payload.Association)
	}
}

func TestContextSpanConditionalFieldsFiveAxes(t *testing.T) {
	type tc struct {
		name              string
		family            SpanFamily
		node, attr        contextRole
		role, otherRole   string
		value, otherValue Evidence[Atomic]
		property          ContextProperty
		want, otherWant   Evidence[Atomic]
	}
	cases := []tc{
		{"pedal-line-types", SpanPedal, spanRole_pedal, spanRole_pedal_line, "change", "change", spanString("yes"), spanString("no"), ContextPropertySpanPedalLineRoleActive, spanBool(true), spanBool(false)},
		{"pedal-sign-default", SpanPedal, spanRole_pedal, spanRole_pedal_line, "start", "start", spanString("no"), spanString("yes"), ContextPropertySpanPedalSign, spanString("yes"), spanString("no")},
		{"pedal-abbreviation", SpanPedal, spanRole_pedal, spanRole_pedal_sign, "start", "stop", spanString("yes"), spanString("yes"), ContextPropertySpanPedalAbbreviatedActive, spanBool(true), spanBool(false)},
		{"pedal-alignment", SpanPedal, spanRole_pedal, spanRole_pedal_sign, "start", "start", spanString("yes"), spanString("no"), ContextPropertySpanPedalAlignmentActive, spanBool(true), spanBool(false)},
		{"bracket-end-length", SpanBracket, spanRole_bracket, spanRole_bracketLineEnd, "start", "start", spanString("up"), spanString("none"), ContextPropertySpanEndLengthApplicable, spanBool(true), spanBool(false)},
		{name: "wedge-spread", family: SpanWedge, node: spanRole_wedge, role: "crescendo", otherRole: "diminuendo", property: ContextPropertySpanWedgeSpreadActive, want: spanBool(false), otherWant: spanBool(true)},
		{name: "wedge-niente-end", family: SpanWedge, node: spanRole_wedge, role: "crescendo", otherRole: "diminuendo", property: ContextPropertySpanWedgeNienteActive, want: spanBool(true), otherWant: spanBool(false)},
		{name: "barre-string-end", family: SpanBarre, node: spanRole_barre, role: "start", otherRole: "stop", property: ContextPropertySpanEndpointMeaning, want: spanString("lower-pitched-higher-numbered-string"), otherWant: spanString("higher-pitched-lower-numbered-string")},
		{name: "barline-wavy", family: SpanWavyLine, node: spanRole_barlineWavy, role: "continue", otherRole: "start", property: ContextPropertySpanBarlineContinueRecommended, want: spanBool(true), otherWant: spanBool(false)},
		{name: "non-arpeggiate-ends", family: SpanNonArpeggiate, node: spanRole_nonArpeggiate, role: "top", otherRole: "bottom", property: ContextPropertySpanEndpointMeaning, want: spanString("top-note-of-graphical-bracket"), otherWant: spanString("bottom-note-of-graphical-bracket")},
		{name: "group-stop-content", family: SpanPartGroup, node: spanRole_partGroup, role: "start", otherRole: "stop", property: ContextPropertySpanGroupStopContentActive, want: spanBool(true), otherWant: spanBool(false)},
		{name: "grouping-stop-content", family: SpanGrouping, node: spanRole_grouping, role: "start", otherRole: "stop", property: ContextPropertySpanGroupStopContentActive, want: spanBool(true), otherWant: stateUnknown[Atomic]("span-grouping-stop-content-may-be-ignored")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, axis := range []string{"positive", "negative", "boundary", "missing-fact", "interaction"} {
				t.Run(axis, func(t *testing.T) {
					r := spanUnitReader(t)
					n := spanUnitNode(r, c.node, nil)
					e := spanTestEndpoint(t, n.header.ID, c.family, c.role, 1)
					want := c.want
					value := c.value
					cause := &DecisionRef{Rule: RuleKey{"scalar", 1}, Subject: e.ref}
					if axis == "boundary" {
						e.payload.Role.Value = c.otherRole
						value = c.otherValue
						want = c.otherWant
					}
					if axis == "negative" || axis == "missing-fact" {
						state, reason := EvidenceInvalid, Code("invalid-antecedent")
						if axis == "missing-fact" {
							state, reason = EvidenceUnknown, "missing-antecedent"
							cause = nil
						}
						want = Evidence[Atomic]{State: state, Reason: reason, Cause: cause}
						if c.attr != "" {
							value = want
						} else {
							e.payload.Role = Evidence[string]{State: state, Reason: reason, Cause: cause}
						}
					}
					if c.attr != "" {
						spanUnitAttribute(r, n, c.attr, value)
					}
					if axis == "interaction" {
						other := spanUnitReader(t)
						on := spanUnitNode(other, c.node, nil)
						oe := spanTestEndpoint(t, on.header.ID, c.family, c.otherRole, 1)
						if c.attr != "" {
							spanUnitAttribute(other, on, c.attr, c.otherValue)
						}
						spanLocalFacts(other, RuleKey{"spans", 1}, on, nil, oe)
					}
					spanLocalFacts(r, RuleKey{"spans", 1}, n, nil, e)
					got := spanUnitValue(t, r, c.property)
					if got.State != want.State || got.State == EvidenceKnown && (got.Value.Text != want.Value.Text || got.Value.Kind != want.Value.Kind) {
						t.Fatalf("got %+v want %+v", got, want)
					}
					if want.Cause != nil && (got.Cause == nil || *want.Cause != *got.Cause) {
						t.Fatal("lost prerequisite cause")
					}
				})
			}
		})
	}
}

func TestContextSpanHammerTextAndGroupSymbolFiveAxes(t *testing.T) {
	for _, family := range []SpanFamily{SpanHammerOn, SpanPullOff, SpanPartGroup} {
		t.Run(string(family), func(t *testing.T) {
			for _, axis := range []string{"positive", "negative", "boundary", "missing-fact", "interaction"} {
				t.Run(axis, func(t *testing.T) {
					r := spanUnitReader(t)
					nr := spanRole_hammerOn
					if family == SpanPullOff {
						nr = spanRole_pullOff
					}
					if family == SpanPartGroup {
						nr = spanRole_partGroup
					}
					n := spanUnitNode(r, nr, nil)
					e := spanTestEndpoint(t, n.header.ID, family, "start", 1)
					prop := ContextPropertySpanHammerMarking
					want := spanString("H")
					cause := &DecisionRef{Rule: RuleKey{"scalar", 1}, Subject: e.ref}
					if family == SpanPartGroup {
						prop = ContextPropertySpanGroupSymbol
						want = spanString("none")
					}
					switch axis {
					case "negative":
						want = Evidence[Atomic]{State: EvidenceInvalid, Reason: "test-scalar-invalid", Cause: cause}
					case "missing-fact":
						want = stateUnknown[Atomic]("test-scalar-unavailable")
					case "boundary":
						if family == SpanPartGroup {
							want = spanString("brace")
						} else {
							want = stateUnknown[Atomic]("span-hammer-marking-application-dependent")
						}
					}
					if family == SpanPartGroup {
						if axis == "boundary" || axis == "negative" {
							ch := spanUnitNode(r, spanRole_groupSymbol, n)
							ch.end.Scalar.Value = want
						}
						if axis == "missing-fact" {
							n.end.ChildrenComplete = Evidence[bool]{State: EvidenceUnknown, Reason: "test-scalar-unavailable"}
						}
					} else {
						n.end.Scalar.Value = want
						if axis == "boundary" {
							n.end.Scalar.Value = spanString("")
						}
					}
					if axis == "interaction" {
						other := spanUnitReader(t)
						on := spanUnitNode(other, nr, nil)
						on.end.Scalar.Value = spanString("different")
						oe := spanTestEndpoint(t, on.header.ID, family, "stop", 1)
						spanLocalFacts(other, RuleKey{"spans", 1}, on, nil, oe)
					}
					spanLocalFacts(r, RuleKey{"spans", 1}, n, nil, e)
					got := spanUnitValue(t, r, prop)
					if got.State != want.State || got.State == EvidenceKnown && got.Value.Text != want.Value.Text {
						t.Fatalf("got %+v want %+v", got, want)
					}
					if want.Cause != nil && (got.Cause == nil || *got.Cause != *cause) {
						t.Fatal("lost cause")
					}
				})
			}
		})
	}
}

func TestContextSpanBeamContinuationFanCause(t *testing.T) {
	r := spanUnitReader(t)
	a := spanUnitNode(r, spanRole_beam, nil)
	b := spanUnitNode(r, spanRole_beam, nil)
	spanUnitAttribute(r, a, spanRole_beam_fan, spanString("accel"))
	cause := &DecisionRef{Rule: RuleKey{"scalar", 1}, Subject: NodeRef{r.s.documentID, b.header.ID}}
	spanUnitAttribute(r, b, spanRole_beam_fan, Evidence[Atomic]{State: EvidenceInvalid, Reason: "fan-invalid", Cause: cause})
	ae := spanTestEndpoint(t, a.header.ID, SpanBeam, "begin", 1)
	be := spanTestEndpoint(t, b.header.ID, SpanBeam, "continue", 1)
	ae.ref.DocumentID = r.s.documentID
	be.ref.DocumentID = r.s.documentID
	for _, e := range []*spanEndpoint{ae, be} {
		e.payload.Association = Evidence[bool]{State: EvidenceKnown, Value: true}
		e.payload.Members = []NodeRef{ae.ref, be.ref}
	}
	spanRelationalFacts(r, RuleKey{"spans", 1}, []*spanEndpoint{ae, be}, nil)
	got := spanUnitValue(t, r, ContextPropertySpanFanApplicable)
	if got.State != EvidenceInvalid || got.Cause == nil || *got.Cause != *cause {
		t.Fatal(got)
	}
}

func TestContextSpanSingleAndLetRingAssociationAxes(t *testing.T) {
	for _, tc := range []struct {
		family SpanFamily
		role   string
	}{{SpanTied, "let-ring"}, {SpanOtherNotation, "single"}, {SpanGrouping, "single"}, {SpanTremolo, "single"}} {
		t.Run(string(tc.family), func(t *testing.T) {
			for _, axis := range []string{"positive", "negative", "boundary", "missing-fact", "interaction"} {
				t.Run(axis, func(t *testing.T) {
					s := rationalSession(t, Budgets{})
					defer s.cleanup()
					e := spanTestEndpoint(t, 1, tc.family, tc.role, 1)
					es := []*spanEndpoint{e}
					want := EvidenceKnown
					if axis == "negative" || axis == "missing-fact" {
						want = EvidenceInvalid
						if axis == "missing-fact" {
							want = EvidenceUnsupported
						}
						cause := &DecisionRef{Rule: RuleKey{"scalar", 1}, Subject: e.ref}
						e.payload.Role = Evidence[string]{State: want, Reason: "endpoint-type-unavailable", Cause: cause}
						e.complete = Evidence[bool]{State: want, Reason: "endpoint-type-unavailable", Cause: cause}
					}
					if axis == "boundary" {
						e.payload.Role.Value = "start"
						want = EvidenceUnknown
					}
					if axis == "interaction" {
						es = append(es, spanTestEndpoint(t, 2, tc.family, tc.role, 1))
					}
					if err := associateSpanEndpoints(context.Background(), s, es, nil); err != nil {
						t.Fatal(err)
					}
					if e.payload.Association.State != want {
						t.Fatal(e.payload)
					}
					if want == EvidenceKnown && len(e.payload.Members) != 1 {
						t.Fatal(e.payload)
					}
					if len(es) == 2 && es[0].payload.Members[0] == es[1].payload.Members[0] {
						t.Fatal("single marks merged")
					}
				})
			}
		})
	}
}
func TestContextSpanUnmeasuredMarksScalarAxes(t *testing.T) {
	for _, axis := range []string{"zero", "nonzero", "invalid", "missing", "separate-note"} {
		t.Run(axis, func(t *testing.T) {
			r := spanUnitReader(t)
			n := spanUnitNode(r, spanRole_tremolo, nil)
			e := spanTestEndpoint(t, n.header.ID, SpanTremolo, "unmeasured", 1)
			n.end.Scalar.Value = spanInteger(0)
			want := spanBool(true)
			switch axis {
			case "nonzero":
				n.end.Scalar.Value = spanInteger(8)
				want = spanBool(false)
			case "invalid":
				want = Evidence[Atomic]{State: EvidenceInvalid, Reason: "invalid-marks", Cause: &DecisionRef{Rule: RuleKey{"scalar", 1}, Subject: e.ref}}
				n.end.Scalar.Value = want
			case "missing":
				want = stateUnknown[Atomic]("missing-marks")
				n.end.Scalar.Value = want
			case "separate-note":
				other := spanUnitNode(r, spanRole_tremolo, nil)
				other.end.Scalar.Value = spanInteger(8)
			}
			spanLocalFacts(r, RuleKey{"spans", 1}, n, nil, e)
			got := spanUnitValue(t, r, ContextPropertySpanTremoloMarksRecommended)
			if got.State != want.State || got.State == EvidenceKnown && got.Value.Text != want.Value.Text {
				t.Fatalf("%+v want %+v", got, want)
			}
			if want.Cause != nil && (got.Cause == nil || *got.Cause != *want.Cause) {
				t.Fatal("cause lost")
			}
		})
	}
}

func TestContextSpanDoubleTremoloRatioDurationAxes(t *testing.T) {
	for _, axis := range []string{"positive", "invalid-ratio", "invalid-type", "nested-ratio", "missing-modification", "missing-timing", "separate-note"} {
		t.Run(axis, func(t *testing.T) {
			r := spanUnitReader(t)
			note := spanUnitNode(r, spanRole_note, nil)
			typ := spanUnitNode(r, spanRole_noteType, note)
			typ.end.Scalar.Value = spanString("quarter")
			var actual *factNode
			if axis != "missing-modification" {
				mod := spanUnitNode(r, spanRole_noteModification, note)
				actual = spanUnitNode(r, spanRole_actualNotes, mod)
				actual.end.Scalar.Value = spanInteger(2)
				normal := spanUnitNode(r, spanRole_normalNotes, mod)
				normal.end.Scalar.Value = spanInteger(1)
			}
			n := spanUnitNode(r, spanRole_tremolo, note)
			e := spanTestEndpoint(t, n.header.ID, SpanTremolo, "start", 1)
			duration, _ := NewRational(big.NewRat(1, 2))
			ts := &timelineService{Facts: map[NodeID]timelineEventFact{note.header.ID: {Duration: Evidence[Rational]{State: EvidenceKnown, Value: duration}}}}
			cause := &DecisionRef{Rule: RuleKey{"scalar", 1}, Subject: e.ref}
			switch axis {
			case "invalid-ratio":
				actual.end.Scalar.Value = Evidence[Atomic]{State: EvidenceInvalid, Reason: "invalid-ratio", Cause: cause}
			case "invalid-type":
				typ.end.Scalar.Value = Evidence[Atomic]{State: EvidenceInvalid, Reason: "invalid-type", Cause: cause}
			case "nested-ratio":
				actual.end.Scalar.Value = spanInteger(3)
			case "missing-timing":
				ts = nil
			case "separate-note":
				other := spanUnitNode(r, spanRole_note, nil)
				ot := spanUnitNode(r, spanRole_noteType, other)
				ot.end.Scalar.Value = spanString("half")
			}
			spanTremoloFacts(r, RuleKey{"spans", 1}, n, e, ts)
			ratio := spanUnitValue(t, r, ContextPropertySpanTremoloRatioRecommended)
			d := spanUnitValue(t, r, ContextPropertySpanTremoloDurationRecommended)
			switch axis {
			case "invalid-ratio":
				if ratio.State != EvidenceInvalid || ratio.Cause == nil || d.State != EvidenceInvalid || d.Cause == nil {
					t.Fatalf("%+v %+v", ratio, d)
				}
			case "invalid-type":
				if d.State != EvidenceInvalid || d.Cause == nil {
					t.Fatal(d)
				}
			case "nested-ratio":
				if ratio.State != EvidenceUnknown || d.State != EvidenceUnknown {
					t.Fatalf("%+v %+v", ratio, d)
				}
			case "missing-modification":
				if ratio.Value.Text != "false" || d.Value.Text != "true" {
					t.Fatalf("%+v %+v", ratio, d)
				}
			case "missing-timing":
				if ratio.Value.Text != "true" || d.State != EvidenceUnknown {
					t.Fatalf("%+v %+v", ratio, d)
				}
			default:
				if ratio.Value.Text != "true" || d.Value.Text != "true" {
					t.Fatalf("%+v %+v", ratio, d)
				}
			}
		})
	}
}

func TestContextSpanHistoricalVersionCause(t *testing.T) {
	for _, state := range []EvidenceState{EvidenceInvalid, EvidenceUnknown, EvidenceUnsupported} {
		t.Run(string(state), func(t *testing.T) {
			r := spanUnitReader(t)
			root := spanUnitNode(r, spanRole_note, nil)
			u := r.b.Roles[spanRole_version][0]
			r.b.Attributes[spanRole_version][root.header.Type.Value] = AttributeSelector{Name: u.Name, Declaration: u.Declaration}
			cause := &DecisionRef{Rule: RuleKey{"version-owner", 1}, Subject: NodeRef{r.s.documentID, root.header.ID}}
			spanUnitAttribute(r, root, spanRole_version, Evidence[Atomic]{State: state, Reason: "version-unavailable", Cause: cause})
			lyric := spanUnitNode(r, spanRole_noteLyric, root)
			n := spanUnitNode(r, spanRole_lyricExtend, lyric)
			var b spanFamilyBinding
			for _, v := range spanFamilies {
				if v.family == SpanLyricExtend {
					b = v
					break
				}
			}
			e := spanExtract(r, RuleKey{"spans", 1}, n, b, nil, nil, 1, nil)
			if r.err != nil {
				t.Fatal(r.err)
			}
			if e.payload.Role.State != state || e.payload.Role.Cause == nil || *e.payload.Role.Cause != *cause {
				t.Fatal(e.payload.Role)
			}
		})
	}
}

func TestContextSpanOctaveSizeInterpretations(t *testing.T) {
	for _, c := range []struct{ size, want int }{{8, 1}, {15, 2}, {22, 3}, {17, 0}} {
		t.Run(spanSmallIntegers[c.size].String(), func(t *testing.T) {
			r := spanUnitReader(t)
			n := spanUnitNode(r, spanRole_octaveShift, nil)
			spanUnitAttribute(r, n, spanRole_octave_shift_size, spanInteger(c.size))
			e := spanTestEndpoint(t, n.header.ID, SpanOctaveShift, "down", 1)
			spanLocalFacts(r, RuleKey{"spans", 1}, n, nil, e)
			v := spanUnitValue(t, r, ContextPropertySpanDisplayOctaves)
			if c.want == 0 {
				if v.State != EvidenceUnknown {
					t.Fatal("closed positive integer domain", v)
				}
			} else if v.State != EvidenceKnown || v.Value.Integer.String() != spanSmallIntegers[c.want].String() {
				t.Fatal(v)
			}
		})
	}
}
