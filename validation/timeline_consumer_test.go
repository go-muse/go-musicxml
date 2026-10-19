package validation

import (
	"context"
	"errors"
	"testing"
)

func consumerFixture(t *testing.T) (*Session, *timelineConsumerPlan) {
	t.Helper()
	s := rationalSession(t, Budgets{})
	s.documentID = "d"
	s.nodes = map[NodeID]*factNode{}
	p := &timelineConsumerPlan{Provider: RuleKey{"timeline", 1}, Bindings: &timelineBindings{}}
	for role, name := range map[timelineRole]string{
		timelineRoleNote: "note", timelineRoleNoteChord: "chord", timelineRoleNoteGrace: "grace", timelineRoleNoteDuration: "duration",
		timelineRoleHarmony: "harmony", timelineRoleHarmonyNumeral: "numeral", timelineRoleNumeralKey: "numeral-key",
		timelineRoleNumeralKeyFifths: "numeral-fifths", timelineRoleNumeralKeyMode: "numeral-mode",
	} {
		p.Bindings.Roles[role] = []SchemaUse{{Declaration: localDecl(name)}}
	}
	s.timeline = map[RuleKey]*timelineService{p.Provider: {Facts: map[NodeID]timelineEventFact{}, Final: true}}
	return s, p
}
func consumerNode(s *Session, id NodeID, parent *factNode, name string) *factNode {
	pid := NodeID(0)
	if parent != nil {
		pid = parent.header.ID
	}
	n := localNode(id, pid, name)
	s.nodes[id] = n
	if parent != nil {
		parent.children = append(parent.children, n)
	}
	return n
}
func consumerEvaluate(t *testing.T, s *Session, n *factNode, p *timelineConsumerPlan, kind planKind) Decision {
	t.Helper()
	d, pending, e := s.evaluateTimelineConsumer(context.Background(), n, p, kind, true)
	if e != nil || pending {
		t.Fatalf("decision=%+v pending=%v error=%v", d, pending, e)
	}
	return d
}
func consumerLocalKey(t *testing.T, s *Session, n *factNode) *factNode {
	t.Helper()
	key := consumerNode(s, 30, n, "numeral-key")
	fifths := consumerNode(s, 31, key, "numeral-fifths")
	fifths.end.Scalar.Value = testEvidence(stateKnown(localInteger(t, 0)))
	mode := consumerNode(s, 32, key, "numeral-mode")
	mode.end.Scalar.Value = testEvidence(stateKnown(localString("major")))
	return key
}
func TestTimelineConsumerChordBoundUsesPrincipal(t *testing.T) {
	for _, tc := range []struct {
		name     string
		duration int64
		want     Outcome
	}{
		{"shorter", 1, OutcomePass}, {"increasing-tone-still-below-principal", 3, OutcomePass}, {"equal", 4, OutcomePass}, {"longer", 5, OutcomeFail},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, p := consumerFixture(t)
			defer s.cleanup()
			principal := consumerNode(s, 1, nil, "note")
			n := consumerNode(s, 2, nil, "note")
			consumerNode(s, 3, n, "chord")
			consumerNode(s, 4, n, "duration")
			s.timeline[p.Provider].Facts[2] = timelineEventFact{Duration: stateKnown(mustRational(t, tc.duration, 4)), PrincipalDuration: stateKnown(mustRational(t, 4, 4)), Principal: stateKnown(NodeRef{"d", 1}), Position: stateUnknown[musicalPosition]("unrelated-position-gap")}
			d := consumerEvaluate(t, s, n, p, planChordDuration)
			if d.Outcome != tc.want {
				t.Fatalf("got %+v want %s", d, tc.want)
			}
			if tc.want == OutcomeFail && (len(d.Related) != 1 || d.Related[0] != principal.header.Location) {
				t.Fatalf("missing principal location %+v", d)
			}
		})
	}
}
func TestTimelineConsumerChordApplicabilityAndCausality(t *testing.T) {
	cause := &DecisionRef{Rule: RuleKey{"xsd.scalar", 1}, Subject: NodeRef{"d", 4}}
	for _, tc := range []struct {
		name                        string
		chord, grace                bool
		duration, principalDuration Evidence[Rational]
		principal                   Evidence[NodeRef]
		want                        Outcome
		reason                      Code
		causal                      bool
	}{
		{name: "nonchord", want: OutcomeNotApplicable},
		{name: "grace-chord", chord: true, grace: true, want: OutcomeNotApplicable},
		{name: "principal-grace-duration-absent", chord: true, duration: stateKnown(mustRational(t, 1, 1)), principalDuration: stateAbsent[Rational](), principal: stateKnown(NodeRef{"d", 1}), want: OutcomeUnknown},
		{name: "missing-duration", chord: true, duration: stateAbsent[Rational](), principalDuration: stateKnown(mustRational(t, 1, 1)), principal: stateKnown(NodeRef{"d", 1}), want: OutcomeUnknown},
		{name: "invalid-duration", chord: true, duration: Evidence[Rational]{State: EvidenceInvalid, Reason: "duration-invalid", Cause: cause}, principal: stateKnown(NodeRef{"d", 1}), want: OutcomeBlocked, reason: "duration-invalid", causal: true},
		{name: "interrupted-principal", chord: true, duration: stateKnown(mustRational(t, 1, 1)), principal: stateUnknown[NodeRef]("chord-principal-interrupted"), want: OutcomeUnknown, reason: "chord-principal-interrupted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, p := consumerFixture(t)
			defer s.cleanup()
			consumerNode(s, 1, nil, "note")
			n := consumerNode(s, 2, nil, "note")
			if tc.chord {
				consumerNode(s, 3, n, "chord")
			}
			if tc.grace {
				consumerNode(s, 5, n, "grace")
			} else {
				consumerNode(s, 4, n, "duration")
			}
			s.timeline[p.Provider].Facts[2] = timelineEventFact{Duration: tc.duration, PrincipalDuration: tc.principalDuration, Principal: tc.principal}
			d := consumerEvaluate(t, s, n, p, planChordDuration)
			if d.Outcome != tc.want || (tc.reason != "" && d.Reason != tc.reason) || (tc.causal && (d.Cause == nil || *d.Cause != *cause)) {
				t.Fatalf("got %+v", d)
			}
		})
	}
}
func TestTimelineConsumerLocalNumeralKeyAlternative(t *testing.T) {
	cause := &DecisionRef{Rule: RuleKey{"xsd.scalar", 1}, Subject: NodeRef{"d", 32}}
	for _, tc := range []struct {
		name       string
		change     func(*Session, *factNode)
		want       Outcome
		conclusive bool
	}{
		{"valid-without-service", nil, OutcomePass, true},
		{"malformed-mode", func(s *Session, _ *factNode) {
			s.nodes[32].end.Scalar.Value = testEvidence(Evidence[Atomic]{State: EvidenceInvalid, Reason: "invalid-mode", Cause: cause})
		}, OutcomeBlocked, true},
		{"missing-mode", func(_ *Session, k *factNode) { k.children = k.children[:1] }, OutcomeUnknown, true},
		{"incomplete-key", func(_ *Session, k *factNode) {
			k.end.ChildrenComplete = testEvidence(stateUnknown[bool]("incomplete-key"))
		}, OutcomeUnknown, true},
		{"incomplete-numeral", func(s *Session, _ *factNode) {
			s.nodes[2].end.ChildrenComplete = testEvidence(stateUnknown[bool]("incomplete-numeral"))
		}, OutcomeUnknown, true},
		{"wrong-exact-use", func(_ *Session, k *factNode) {
			k.header.Declaration = testEvidence(stateKnown(localDecl("foreign-key")))
		}, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, p := consumerFixture(t)
			defer s.cleanup()
			s.timeline = nil
			harmony := consumerNode(s, 1, nil, "harmony")
			n := consumerNode(s, 2, harmony, "numeral")
			k := consumerLocalKey(t, s, n)
			if tc.change != nil {
				tc.change(s, k)
			}
			d, yes, e := s.localNumeralKeyProof(context.Background(), n, p)
			if e != nil || yes != tc.conclusive || (yes && d.Outcome != tc.want) {
				t.Fatalf("proof=%+v yes=%v error=%v", d, yes, e)
			}
			if tc.name == "malformed-mode" && (d.Cause == nil || *d.Cause != *cause) {
				t.Fatalf("lost scalar cause %+v", d)
			}
		})
	}
}
func consumerKey(t *testing.T, traditional bool, fifths int64) keyState {
	t.Helper()
	v := keyState{Traditional: traditional, Fifths: stateAbsent[Integer](), Mode: stateAbsent[string](), Cancel: stateAbsent[Integer](), CancelLocation: stateAbsent[string]()}
	if traditional {
		v.Fifths = stateKnown(stateInt(t, fifths))
	}
	return v
}
func TestTimelineConsumerNumeralEffectiveKeyEvidence(t *testing.T) {
	cause := &DecisionRef{Rule: RuleKey{"xsd.scalar", 1}, Subject: NodeRef{"d", 40}}
	for _, tc := range []struct {
		name    string
		prepare func(*testing.T, *Session, *timelineService)
		want    Outcome
		causal  bool
	}{
		{"closed-absence", nil, OutcomeFail, false},
		{"traditional-zero", func(t *testing.T, s *Session, v *timelineService) {
			statePublish(t, s, &v.State.Keys, statePosition(t, 0, 0, 1), stateScope{AllStaves: true}, stateKnown(consumerKey(t, true, 0)), 40)
		}, OutcomePass, false},
		{"empty-nontraditional", func(t *testing.T, s *Session, v *timelineService) {
			statePublish(t, s, &v.State.Keys, statePosition(t, 0, 0, 1), stateScope{AllStaves: true}, stateKnown(consumerKey(t, false, 0)), 40)
		}, OutcomePass, false},
		{"same-position-known-conflict-still-encoded", func(t *testing.T, s *Session, v *timelineService) {
			for i := int64(0); i < 2; i++ {
				statePublish(t, s, &v.State.Keys, statePosition(t, 0, 0, 1), stateScope{AllStaves: true}, stateKnown(consumerKey(t, true, i)), NodeID(40+i))
			}
		}, OutcomePass, false},
		{"other-staff-key", func(t *testing.T, s *Session, v *timelineService) {
			statePublish(t, s, &v.State.Keys, statePosition(t, 0, 0, 1), stateScope{Staff: stateInt(t, 2)}, stateKnown(consumerKey(t, false, 0)), 40)
		}, OutcomeFail, false},
		{"other-part-key", func(t *testing.T, s *Session, v *timelineService) {
			at := statePosition(t, 0, 0, 1)
			at.Part.Node = 9
			statePublish(t, s, &v.State.Keys, at, stateScope{AllStaves: true}, stateKnown(consumerKey(t, false, 0)), 40)
		}, OutcomeFail, false},
		{"unclosed-state", func(_ *testing.T, _ *Session, v *timelineService) { v.State.Closures = nil }, OutcomeUnknown, false},
		{"invalid-key", func(t *testing.T, s *Session, v *timelineService) {
			statePublish(t, s, &v.State.Keys, statePosition(t, 0, 0, 1), stateScope{AllStaves: true}, Evidence[keyState]{State: EvidenceInvalid, Reason: "bad-key", Cause: cause}, 40)
		}, OutcomeBlocked, true},
		{"invalid-key-component", func(t *testing.T, s *Session, v *timelineService) {
			key := consumerKey(t, true, 0)
			key.Mode = Evidence[string]{State: EvidenceInvalid, Reason: "bad-key-mode", Cause: cause}
			statePublish(t, s, &v.State.Keys, statePosition(t, 0, 0, 1), stateScope{AllStaves: true}, stateKnown(key), 40)
		}, OutcomeBlocked, true},
		{"position-gap", func(_ *testing.T, _ *Session, v *timelineService) {
			f := v.Facts[1]
			f.Origin = Evidence[musicalPosition]{State: EvidenceInvalid, Reason: "bad-position", Cause: cause}
			v.Facts[1] = f
		}, OutcomeBlocked, true},
		{"staff-gap", func(_ *testing.T, _ *Session, v *timelineService) {
			f := v.Facts[1]
			f.Staff = stateUnknown[Integer]("staff-unresolved")
			v.Facts[1] = f
		}, OutcomeUnknown, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, p := consumerFixture(t)
			defer s.cleanup()
			v := s.timeline[p.Provider]
			h := consumerNode(s, 1, nil, "harmony")
			n := consumerNode(s, 2, h, "numeral")
			v.Facts[1] = timelineEventFact{Origin: stateKnown(statePosition(t, 0, 1, 1)), Staff: stateKnown(stateInt(t, 1))}
			stateClose(t, s, &v.State, 0)
			if tc.prepare != nil {
				tc.prepare(t, s, v)
			}
			d := consumerEvaluate(t, s, n, p, planNumeralKey)
			if d.Outcome != tc.want || (tc.causal && (d.Cause == nil || *d.Cause != *cause)) {
				t.Fatalf("got %+v want %s", d, tc.want)
			}
		})
	}
}
func TestTimelineConsumerPendingAndCancellation(t *testing.T) {
	s, p := consumerFixture(t)
	defer s.cleanup()
	h := consumerNode(s, 1, nil, "harmony")
	n := consumerNode(s, 2, h, "numeral")
	s.timeline[p.Provider].Final = false
	d, pending, e := s.evaluateTimelineConsumer(context.Background(), n, p, planNumeralKey, false)
	if e != nil || !pending {
		t.Fatalf("premature result %+v %v %v", d, pending, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, e = s.localNumeralKeyProof(ctx, n, p)
	if !errors.Is(e, context.Canceled) {
		t.Fatalf("ignored cancellation %v", e)
	}
}

func consumerSchema(t *testing.T) (*CompiledSchema, *timelineConsumerPlan) {
	t.Helper()
	sequence := func(children ...*SchemaParticle) *SchemaParticle {
		return &SchemaParticle{Kind: SchemaParticleSequence, Occurrence: SchemaOccurrence{Min: 1, Max: 1}, Children: children}
	}
	types := map[QName]*SchemaType{}
	complex := func(name string, particle *SchemaParticle) {
		types[QName{Local: name}] = &SchemaType{Complex: &SchemaComplex{Form: SchemaComplexDirect, Particle: particle}}
	}
	complex("root", localChoice(localElement("harmony", "harmony")))
	complex("harmony", localChoice(localElement("numeral", "numeral")))
	key := localElement("numeral-key", "numeral-key")
	key.Occurrence.Min = 0
	complex("numeral", sequence(key))
	fifths := localElement("numeral-fifths", "integer")
	fifths.Element.Type = localBuiltin("integer")
	complex("numeral-key", sequence(fifths, localElement("numeral-mode", "mode")))
	types[QName{Local: "mode"}] = &SchemaType{Simple: &SchemaSimple{Form: SchemaSimpleRestriction, Base: &SchemaSimpleMember{Name: localBuiltin("token").Name}, Enumerations: []string{"major", "minor"}}}
	schema, e := CompileSchema(AssemblyMusicXML, SchemaData{Types: types, Elements: map[QName]*SchemaElement{{Local: "root"}: {Name: QName{Local: "root"}, Type: localType("root")}}})
	if e != nil {
		t.Fatal(e)
	}
	p := &timelineConsumerPlan{Schema: schema, Provider: RuleKey{"timeline", 1}, Bindings: &timelineBindings{}}
	for _, v := range []struct {
		role        timelineRole
		owner, name string
	}{
		{timelineRoleHarmony, "root", "harmony"}, {timelineRoleHarmonyNumeral, "harmony", "numeral"}, {timelineRoleNumeralKey, "numeral", "numeral-key"},
		{timelineRoleNumeralKeyFifths, "numeral-key", "numeral-fifths"}, {timelineRoleNumeralKeyMode, "numeral-key", "numeral-mode"},
	} {
		p.Bindings.Roles[v.role] = []SchemaUse{localUse(t, schema, v.owner, v.name, false)}
	}
	return schema, p
}
func TestTimelineConsumerNumeralLocalSchemaProof(t *testing.T) {
	schema, p := consumerSchema(t)
	for _, target := range []Target{TargetSource, TargetModel} {
		for _, variant := range []string{"valid", "invalid-mode", "invalid-fifths", "missing-mode", "duplicate-fifths", "wrong-order", "foreign-child", "incomplete-key", "nested-mode", "incomplete-mode"} {
			t.Run(string(target)+"/"+variant, func(t *testing.T) {
				fifths := &localTree{name: "numeral-fifths", text: "0"}
				v := localInteger(t, 0)
				fifths.value = &v
				mode := &localTree{name: "numeral-mode", text: "major"}
				key := lt("numeral-key", fifths, mode)
				switch variant {
				case "invalid-mode":
					mode.text = "wrong"
				case "invalid-fifths":
					fifths.text = "wrong"
					v := localString("wrong")
					fifths.value = &v
				case "missing-mode":
					key.children = key.children[:1]
				case "duplicate-fifths":
					other := *fifths
					key.children = []*localTree{fifths, &other, mode}
				case "wrong-order":
					key.children = []*localTree{mode, fifths}
				case "foreign-child":
					key.children = append(key.children, lt("foreign"))
				case "incomplete-key":
					key.incomplete = true
				case "nested-mode":
					mode.children = []*localTree{lt("foreign")}
				case "incomplete-mode":
					mode.incomplete = true
				}
				s := localAdmit(t, localEngine(t, schema, target), target, lt("root", lt("harmony", lt("numeral", key))), nil)
				defer s.cleanup()
				var n *factNode
				for _, node := range s.nodes {
					if node.header.Name.Local == "numeral" {
						n = node
						break
					}
				}
				d, yes, e := s.localNumeralKeyProof(context.Background(), n, p)
				if e != nil || !yes {
					t.Fatalf("proof=%+v conclusive=%v error=%v", d, yes, e)
				}
				if variant == "valid" {
					if d.Outcome != OutcomePass {
						t.Fatalf("valid key %+v", d)
					}
					return
				}
				if variant == "incomplete-key" || variant == "incomplete-mode" {
					if d.Outcome != OutcomeUnknown {
						t.Fatalf("incomplete key %+v", d)
					}
					return
				}
				if d.Outcome != OutcomeBlocked || d.Cause == nil {
					t.Fatalf("malformed key became presence proof %+v", d)
				}
				want := RuleID("xsd.content")
				if variant == "invalid-mode" || variant == "invalid-fifths" || variant == "nested-mode" {
					want = "xsd.scalar"
				}
				if d.Cause.Rule.ID != want {
					t.Fatalf("cause=%+v want %s", d.Cause, want)
				}
			})
		}
	}
}
func TestTimelineConsumerNumeralOwnLocalKeyOnly(t *testing.T) {
	s, p := consumerFixture(t)
	defer s.cleanup()
	h := consumerNode(s, 1, nil, "harmony")
	sibling := consumerNode(s, 2, h, "numeral")
	consumerLocalKey(t, s, sibling)
	subject := consumerNode(s, 3, h, "numeral")
	d, yes, e := s.localNumeralKeyProof(context.Background(), subject, p)
	if e != nil || yes {
		t.Fatalf("borrowed sibling key %+v %v %v", d, yes, e)
	}
}
func TestTimelineConsumerExactProofInterruptionGuard(t *testing.T) {
	for _, kind := range []planKind{planChordDuration, planNumeralKey} {
		for _, variant := range []string{"matching", "other-node", "other-plan", "other-family", "nested", "other-producer"} {
			t.Run(string(kind)+"/"+variant, func(t *testing.T) {
				s, p := consumerFixture(t)
				defer s.cleanup()
				n := consumerNode(s, 1, nil, "note")
				s.profile = Profile{Target: TargetSource}
				s.accumulator = newReportAccumulator(s.profile, s.account)
				s.decisions = map[decisionID]Decision{}
				rule := RuleDefinition{Key: RuleKey{"consumer", 1}, Class: ClassMandatory, Category: CategoryNormative, Role: RolePredicate}
				plan := Plan{kind: kind, timelineConsumer: p}
				cause := &ResourceError{Kind: BudgetBufferedBytes, Limit: 1, Requested: 128}
				proof := &provedPlanInterruption{node: n.header.ID, timeline: p, kind: kind, decision: Decision{Outcome: OutcomeFail, Reason: "proved-consumer-failure"}, cause: cause}
				var interrupted error = proof
				switch variant {
				case "other-node":
					proof.node = 2
				case "other-plan":
					other := *p
					proof.timeline = &other
				case "other-family":
					if kind == planChordDuration {
						proof.kind = planNumeralKey
					} else {
						proof.kind = planChordDuration
					}
				case "nested":
					interrupted = errors.Join(proof)
				case "other-producer":
					proof.local = &localPlan{}
				}
				e := s.recordProvedPlanInterruption(rule, n, plan, interrupted)
				if !errors.Is(e, cause) {
					t.Fatalf("cause lost %v", e)
				}
				want := uint64(0)
				if variant == "matching" {
					want = 1
				}
				if s.accumulator.report.Counts.Fail != want {
					t.Fatalf("incorrect producer attribution %+v", s.accumulator.report.Counts)
				}
			})
		}
	}
}
func TestTimelineConsumerFailureSurvivesRelatedRetentionStop(t *testing.T) {
	for _, kind := range []planKind{planChordDuration, planNumeralKey} {
		t.Run(string(kind), func(t *testing.T) {
			s, p := consumerFixture(t)
			defer s.cleanup()
			n := consumerNode(s, 1, nil, "note")
			// First establish the actual sealed predicate's false consequence.
			var d Decision
			if kind == planChordDuration {
				principal := consumerNode(s, 2, nil, "note")
				consumerNode(s, 3, n, "chord")
				consumerNode(s, 4, n, "duration")
				s.timeline[p.Provider].Facts[1] = timelineEventFact{Duration: stateKnown(mustRational(t, 2, 1)), PrincipalDuration: stateKnown(mustRational(t, 1, 1)), Principal: stateKnown(NodeRef{"d", principal.header.ID})}
				d = consumerEvaluate(t, s, n, p, kind)
			} else {
				n.header.Declaration = testEvidence(stateKnown(localDecl("numeral")))
				h := consumerNode(s, 2, nil, "harmony")
				n.header.Parent = 2
				s.timeline[p.Provider].Facts[h.header.ID] = timelineEventFact{Origin: stateKnown(statePosition(t, 0, 0, 1)), Staff: stateKnown(stateInt(t, 1))}
				stateClose(t, s, &s.timeline[p.Provider].State, 0)
				d = consumerEvaluate(t, s, n, p, kind)
			}
			if d.Outcome != OutcomeFail {
				t.Fatalf("expected established failure %+v", d)
			}
			oldAccount := s.account
			oldRetained := s.retained
			account, e := NewBudgetAccount(Budgets{MaxBufferedBytes: 1})
			if e != nil {
				t.Fatal(e)
			}
			s.account = account
			s.retained = 0
			s.accumulator = newReportAccumulator(Profile{Target: TargetSource}, account)
			s.decisions = map[decisionID]Decision{}
			got, e := s.withTimelineProvedRelated(n, p, kind, d, n.header.Location)
			if got.Outcome != OutcomeFail || !errors.Is(e, ErrResourceLimit) {
				t.Fatalf("proof lost %+v %v", got, e)
			}
			rule := RuleDefinition{Key: RuleKey{"consumer", 1}, Class: ClassMandatory, Category: CategoryNormative, Role: RolePredicate}
			e = s.recordProvedPlanInterruption(rule, n, Plan{kind: kind, timelineConsumer: p}, e)
			if !errors.Is(e, ErrResourceLimit) || s.accumulator.report.Counts.Fail != 1 {
				t.Fatalf("failure not latched %+v %v", s.accumulator.report.Counts, e)
			}
			s.cleanup()
			s.account = oldAccount
			s.retained = oldRetained
		})
	}
}
func TestTimelineConsumerWideLocalScanUsesWorkBudget(t *testing.T) {
	s, p := consumerFixture(t)
	defer s.cleanup()
	n := consumerNode(s, 1, nil, "numeral")
	for i := NodeID(2); i < 100; i++ {
		consumerNode(s, i, n, "other")
	}
	account, e := NewBudgetAccount(Budgets{MaxRuleSteps: 20})
	if e != nil {
		t.Fatal(e)
	}
	s.account = account
	d, _, e := s.localNumeralKeyProof(context.Background(), n, p)
	if !errors.Is(e, ErrResourceLimit) || d.Outcome == OutcomeFail || d.Outcome == OutcomePass {
		t.Fatalf("unmetered or fabricated result %+v %v", d, e)
	}
}

func TestTimelineConsumerLocalKeyInvalidComponentSurvivesEarlierGap(t *testing.T) {
	s, p := consumerFixture(t)
	defer s.cleanup()
	h := consumerNode(s, 1, nil, "harmony")
	n := consumerNode(s, 2, h, "numeral")
	consumerLocalKey(t, s, n)
	s.nodes[31].end.Scalar.Value = testEvidence(stateUnknown[Atomic]("fifths-unavailable"))
	cause := &DecisionRef{Rule: RuleKey{"xsd.scalar", 1}, Subject: NodeRef{"d", 32}}
	s.nodes[32].end.Scalar.Value = testEvidence(Evidence[Atomic]{State: EvidenceInvalid, Reason: "invalid-mode", Cause: cause})
	d, yes, e := s.localNumeralKeyProof(context.Background(), n, p)
	if e != nil || !yes || d.Outcome != OutcomeBlocked || d.Cause == nil || *d.Cause != *cause {
		t.Fatalf("known scalar violation hidden by earlier gap %+v %v %v", d, yes, e)
	}
}
