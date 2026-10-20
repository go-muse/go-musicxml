package validation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
	"unsafe"
)

func TestSourcePresencePlansAreAdditiveAndOwned(t *testing.T) {
	attr := localAttr("dacapo")
	values := []Atomic{localString("yes")}
	legacy, err := NewPresentAttributeValuePlan(attr, values)
	if err != nil {
		t.Fatal(err)
	}
	current, err := NewSourceAttributeValuePlan(attr, values)
	if err != nil {
		t.Fatal(err)
	}
	old, _ := json.Marshal(legacy.local.digest())
	// Legacy digests keep their original eight-field shape.
	var oldShape map[string]json.RawMessage
	if err := json.Unmarshal(old, &oldShape); err != nil {
		t.Fatal(err)
	}
	if len(oldShape) != 8 {
		t.Fatalf("legacy digest shape changed: %s", old)
	}
	now, _ := json.Marshal(current.local.digest())
	if string(old) == string(now) {
		t.Fatal("source presence mode missing from digest")
	}
	values[0] = localString("no")
	n := localNode(1, 0, "sound")
	localSetAttr(n, "dacapo", localString("no"))
	if d := localRun(t, current, n, n); d.Outcome != OutcomeFail {
		t.Fatalf("effective DTD value or input ownership lost: %+v", d)
	}
	n.attributes = nil
	if d := localRun(t, current, n, n); d.Outcome != OutcomeNotApplicable {
		t.Fatalf("XSD-only omission activates source presence: %+v", d)
	}
	owner := Selector{Declaration: &n.header.Declaration.Value}
	trigger := localAttr("x2")
	p, err := NewSourcePresenceImpliesAttributePlan([]AttributeSelector{trigger}, owner, localAttr("type"), localString("continue"))
	if err != nil {
		t.Fatal(err)
	}
	localSetAttr(n, "x2", localString("1"))
	localSetAttr(n, "type", localString("start"))
	if d := localRun(t, p, n, n); d.Outcome != OutcomeFail {
		t.Fatalf("source second control does not activate clause: %+v", d)
	}
	n.attributes[0].Declaration.Value.Occurrence = "other-use"
	if d := localRun(t, p, n, n); d.Outcome != OutcomeNotApplicable {
		t.Fatalf("wrong declaration activates trigger: %+v", d)
	}
}

func sourceResourceFixture(t *testing.T, limits Budgets) (*Engine, *Session, RuleKey) {
	t.Helper()
	base, oldSession, originKey := sourceOriginFixture(t, Budgets{})
	oldSession.cleanup()
	profile, _ := base.Profile("source-schema-test")
	profile.CatalogDigest = ""
	var rules []RuleDefinition
	var plans []PlanBinding
	var schema *CompiledSchema
	for _, key := range profile.Required {
		rule, _ := base.catalog.Rule(key)
		rules = append(rules, rule)
		plan := base.catalog.plans[rule.Predicate]
		plans = append(plans, PlanBinding{ID: rule.Predicate, Plan: plan})
		if plan.schema != nil {
			schema = plan.schema
		}
	}
	p, err := newSourceSchemaHintContextPlan(schema)
	if err != nil {
		t.Fatal(err)
	}
	assembly := AssemblyMusicXML
	rule := RuleDefinition{Key: RuleKey{"test.source-hints", 1}, Targets: []Target{TargetSource}, Scope: ScopeDocument, Category: CategoryNormative, Class: ClassMandatory, Role: RoleContextProvider, Selector: Selector{Assembly: &assembly}, Predicate: "source-hints", Text: RuleText{Title: "Source hints", Explanation: "Project effective hints with physical provenance.", Message: "Hint evidence unavailable."}, Sources: []SourceRef{{IDs: []string{"XML-1.0"}, URL: "https://www.w3.org/TR/xml/", Locator: "3.3.2"}}}
	rules = append(rules, rule)
	plans = append(plans, PlanBinding{ID: rule.Predicate, Plan: p})
	profile.Required = append(profile.Required, rule.Key)
	profile.Support = append(profile.Support, RuleSupport{Rule: rule.Key, State: ImplementationAvailable})
	c, err := NewCatalog(rules, []Profile{profile}, plans)
	if err != nil {
		t.Fatal(err)
	}
	e, err := NewEngine(c)
	if err != nil {
		t.Fatal(err)
	}
	s, err := e.Begin(context.Background(), RunOptions{Context: ContextFamilyAll, DocumentID: "source", Profile: profile.ID, Budgets: limits})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.cleanup)
	return e, s, originKey
}

func sourceResourceAdmit(t *testing.T, s *Session, key RuleKey, kind SourceOriginKind, name QName, value string, physical bool) SourceOriginContextResult {
	t.Helper()
	ctx := context.Background()
	if s.root == nil {
		h := schemaPendingHeader()
		h.Location = sourceOriginPhysical().Physical.Value
		if err := s.OpenNode(ctx, h); err != nil {
			t.Fatal(err)
		}
		if err := s.sourceOrigin(ctx, key, sourceOriginPhysical()); err != nil {
			t.Fatal(err)
		}
	}
	loc := Location{Target: TargetSource, DocumentID: "source", Path: "/root/@" + name.Local}
	if physical {
		loc.Position = &SourcePosition{ByteOffset: 22, Line: 2, Column: 10, CoordinateSpace: "xml"}
	}
	a := sourceOriginAttribute(name, loc)
	a.Scalar.Lexeme = stateKnown(value)
	if err := s.Attribute(ctx, a); err != nil {
		t.Fatal(err)
	}
	o := SourceOriginContextResult{Subject: NodeRef{"source", 1}, Attribute: &name, Kind: kind, Specified: stateKnown(physical), Physical: stateAbsent[Location](), Declaration: stateAbsent[Location](), Use: stateAbsent[Location](), AncestryComplete: stateKnown(true)}
	if physical {
		o.Physical = stateKnown(loc)
	}
	if kind == SourceOriginDefault || kind == SourceOriginEntity {
		o.Declaration = stateKnown(sourceOriginPhysical().Physical.Value)
		o.Use = o.Declaration
		if kind == SourceOriginEntity {
			o.Ancestry = []SourceOriginStep{{Entity: "entity", Declaration: o.Declaration, Use: o.Use}}
		}
	}
	if err := s.sourceOrigin(ctx, key, o); err != nil {
		t.Fatal(err)
	}
	return o
}

func TestSourceResourceHintEffectivePairsDespiteAbsentPhysicalRaw(t *testing.T) {
	for _, kind := range []SourceOriginKind{SourceOriginDefault, SourceOriginEntity, SourceOriginPhysical} {
		t.Run(string(kind), func(t *testing.T) {
			_, s, key := sourceResourceFixture(t, Budgets{})
			name := QName{Space: "http://www.w3.org/2001/XMLSchema-instance", Local: "schemaLocation"}
			o := sourceResourceAdmit(t, s, key, kind, name, "urn:a a.xsd trailing", kind == SourceOriginPhysical)
			raw := stateAbsent[string]()
			if kind == SourceOriginPhysical {
				raw = stateKnown(" urn:a  a.xsd trailing ")
			}
			if err := s.observeSourceAttributeLexical(context.Background(), key, o.Subject, name, raw); err != nil {
				t.Fatal(err)
			}
			if err := s.CloseNode(context.Background(), schemaLexemeEnd("1")); err != nil {
				t.Fatal(err)
			}
			r := &contextReader{ctx: context.Background(), s: s, sourceInfoset: true}
			if err := projectResourceHintNode(r, RuleKey{"test.source-hints", 1}, s.root); err != nil {
				t.Fatal(err)
			}
			got := s.contextResults[len(s.contextResults)-1].Resource.SchemaHints
			if got.Raw.State != raw.State || got.Raw.Value != raw.Value || len(got.Pairs) != 1 || got.Pairs[0].Namespace.Value != "urn:a" || got.Pairs[0].Location.Value != "a.xsd" || got.Trailing.Value != "trailing" || got.Complete.State != EvidenceKnown || got.Complete.Value {
				t.Fatalf("lost effective pairing or fabricated raw: %+v", got)
			}
			if kind != SourceOriginPhysical && got.Pairs[0].Origin.Position != nil {
				t.Fatalf("fabricated hint coordinates: %+v", got.Pairs[0].Origin)
			}
		})
	}
}

func TestSourceAttributeLexicalOwnershipAndMissingEvidence(t *testing.T) {
	e, s, key := sourceResourceFixture(t, Budgets{})
	if got, ok := e.sourceAttributeLexicalProvider("source-schema-test", AssemblyMusicXML); !ok || got != key {
		t.Fatalf("mode election=%+v/%v", got, ok)
	}
	name := QName{Local: "id"}
	o := sourceResourceAdmit(t, s, key, SourceOriginPhysical, name, "normalized", true)
	p, raw, err := s.sourcePhysicalAttribute(context.Background(), s.root, &s.root.attributes[0], stateKnown(true))
	if err != nil || p.State != EvidenceKnown || !p.Value || raw.State != EvidenceUnknown {
		t.Fatalf("missing raw fabricated: %+v %+v %v", p, raw, err)
	}
	cause := &DecisionRef{Rule: RuleKey{"lexical-gap", 1}, Subject: o.Subject}
	in := Evidence[string]{State: EvidenceInvalid, Reason: "reference-spelling-invalid", Cause: cause}
	if err := s.observeSourceAttributeLexical(context.Background(), key, o.Subject, name, in); err != nil {
		t.Fatal(err)
	}
	cause.Rule.ID = "mutated"
	_, raw, err = s.sourcePhysicalAttribute(context.Background(), s.root, &s.root.attributes[0], stateKnown(true))
	if err != nil || raw.Cause == nil || raw.Cause.Rule.ID != "lexical-gap" {
		t.Fatalf("raw evidence borrowed caller state: %+v %v", raw, err)
	}
	if err := s.observeSourceAttributeLexical(context.Background(), key, o.Subject, name, in); err == nil {
		t.Fatal("accepted duplicate lexical ingress")
	}
}

func TestSourceAttributeLexicalGatesAndBudgets(t *testing.T) {
	for _, mode := range []string{"wrong-provider", "wrong-document", "wrong-node", "wrong-slot", "synthetic-known", "physical-absent", "closed", "canceled", "invalid-utf8", "scalar-budget", "buffer-budget"} {
		t.Run(mode, func(t *testing.T) {
			_, s, key := sourceResourceFixture(t, Budgets{})
			name := QName{Local: "id"}
			kind, physical := SourceOriginPhysical, true
			if mode == "synthetic-known" {
				kind, physical = SourceOriginDefault, false
			}
			o := sourceResourceAdmit(t, s, key, kind, name, "id", physical)
			subject := o.Subject
			raw := stateKnown(" id ")
			ctx := context.Background()
			switch mode {
			case "wrong-provider":
				key = RuleKey{"other", 1}
			case "wrong-document":
				subject.DocumentID = "other"
			case "wrong-node":
				subject.Node++
			case "wrong-slot":
				name.Local = "other"
			case "physical-absent":
				raw = stateAbsent[string]()
			case "closed":
				if err := s.CloseNode(ctx, schemaLexemeEnd("1")); err != nil {
					t.Fatal(err)
				}
			case "canceled":
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			case "invalid-utf8":
				raw = stateKnown(string([]byte{0xff}))
			case "scalar-budget":
				raw = stateKnown(strings.Repeat("x", int(s.account.Limits().MaxScalarBytes)+1))
			case "buffer-budget":
				used, _ := s.account.Usage(BudgetBufferedBytes)
				if err := s.account.Charge(BudgetBufferedBytes, s.account.Limits().MaxBufferedBytes-used); err != nil {
					t.Fatal(err)
				}
			}
			before := s.retained
			err := s.observeSourceAttributeLexical(ctx, key, subject, name, raw)
			if err == nil {
				t.Fatal("accepted invalid lexical ingress")
			}
			if mode == "canceled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("lost cancellation: %v", err)
			}
			if strings.HasSuffix(mode, "budget") && !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("lost resource cause: %v", err)
			}
			if s.retained != before || len(s.sourceAttributeLexical) != 0 {
				t.Fatal("failed lexical admission retained state")
			}
		})
	}
}

func TestSourceResourceAttributeSeparatesEffectiveAndPhysicalEvidence(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		specified             bool
		origin, raw           bool
		wantPresence, wantRaw EvidenceState
	}{
		{"physical", true, true, true, EvidenceKnown, EvidenceKnown},
		{"physical-missing-raw", true, true, false, EvidenceKnown, EvidenceUnknown},
		{"default", false, true, false, EvidenceAbsent, EvidenceAbsent},
		{"entity-markup", false, true, false, EvidenceAbsent, EvidenceAbsent},
		{"missing-origin", true, false, false, EvidenceUnknown, EvidenceUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := localNode(1, 0, "image")
			n.header.Type = testEvidence(stateKnown(ComponentID{Assembly: AssemblyMusicXML, Kind: ComponentType, Name: QName{Local: "image"}}))
			localSetAttr(n, "source", localString("effective.png"))
			n.attributes[0].Scalar.Lexeme = stateKnown("effective.png")
			s := rationalSession(t, Budgets{})
			defer s.cleanup()
			role := resourceRole("image.@source")
			selector := localAttr("source")
			b := &contextBindings{Attributes: map[contextRole]map[ComponentID]AttributeSelector{role: {n.header.Type.Value: selector}}}
			slot := sourceOriginSubject{Node: 1, Attribute: selector.Name}
			if tc.origin {
				s.sourceOriginSeen = map[sourceOriginSubject]*SourceOriginContextResult{slot: {Specified: stateKnown(tc.specified)}}
			}
			if tc.raw {
				s.sourceAttributeLexical = map[sourceOriginSubject]Evidence[string]{slot: stateKnown("  original.png  ")}
			}
			r := &contextReader{ctx: context.Background(), s: s, b: b, sourceInfoset: true}
			got := resourceAttribute(r, n, role)
			if r.err != nil || got.Presence.State != tc.wantPresence || got.Raw.State != tc.wantRaw || got.Value.State != EvidenceKnown || got.Value.Value.Text != "effective.png" {
				t.Fatalf("projection=%+v error=%v", got, r.err)
			}
			if tc.raw && got.Raw.Value != "  original.png  " {
				t.Fatalf("raw replaced by effective value: %+v", got)
			}
			r.sourceInfoset = false
			old := resourceAttribute(r, n, role)
			if old.Presence.State != EvidenceKnown || !old.Presence.Value || old.Raw.Value != "effective.png" {
				t.Fatalf("legacy mode changed: %+v", old)
			}
			r.sourceInfoset = true
			n.attributes[0].Declaration.Value.Occurrence = "wrong-declaration"
			wrong := resourceAttribute(r, n, role)
			if wrong.Presence.State != EvidenceAbsent || wrong.Raw.State != EvidenceAbsent || wrong.Value.State != EvidenceAbsent {
				t.Fatalf("same-name origin borrowed for different declaration: %+v", wrong)
			}
		})
	}
}

func TestSourceLexicalModeIsSelectedAndSessionLocal(t *testing.T) {
	legacy, old, oldKey := sourceOriginFixture(t, Budgets{})
	defer old.cleanup()
	if key, ok := legacy.sourceAttributeLexicalProvider("source-schema-test", AssemblyMusicXML); ok || key != (RuleKey{}) {
		t.Fatal("legacy source origin acquired lexical transport")
	}
	e, current, key := sourceResourceFixture(t, Budgets{})
	if _, ok := e.sourceAttributeLexicalProvider("source-schema-test", AssemblyOpus); ok {
		t.Fatal("elected wrong assembly")
	}
	if _, ok := e.sourceAttributeLexicalProvider("unknown", AssemblyMusicXML); ok {
		t.Fatal("elected unknown profile")
	}
	name := QName{Local: "id"}
	sourceResourceAdmit(t, old, oldKey, SourceOriginPhysical, name, "old", true)
	o := sourceResourceAdmit(t, current, key, SourceOriginPhysical, name, "new", true)
	if err := old.observeSourceAttributeLexical(context.Background(), oldKey, o.Subject, name, stateKnown("old")); err == nil {
		t.Fatal("legacy mode accepted new transport")
	}
	if err := current.observeSourceAttributeLexical(context.Background(), key, o.Subject, name, stateKnown(" current ")); err != nil {
		t.Fatal(err)
	}
	_, independent, independentKey := sourceResourceFixture(t, Budgets{})
	sourceResourceAdmit(t, independent, independentKey, SourceOriginPhysical, name, "other", true)
	_, raw, err := independent.sourcePhysicalAttribute(context.Background(), independent.root, &independent.root.attributes[0], stateKnown(true))
	if err != nil || raw.State != EvidenceUnknown {
		t.Fatalf("lexical state crossed sessions: %+v %v", raw, err)
	}
	current.cleanup()
	if current.sourceAttributeLexical != nil || current.sourceOriginSeen != nil {
		t.Fatal("cleanup retained source indexes")
	}
}

func TestSourceOriginIndexAccountingEnvelopeAndOwnedPointers(t *testing.T) {
	// Reserve two map-entry generations, with a full control word per slot,
	// including the indexed pointer. The initial 512-byte charge covers map and
	// table headers. This bound is host-layout based and is also run on GOARCH=386.
	indexed := unsafe.Sizeof(sourceOriginSubject{}) + unsafe.Sizeof((*SourceOriginContextResult)(nil)) + unsafe.Sizeof(uint64(0))
	if int64(2*indexed) > sourceOriginIndexEntryBytes {
		t.Fatalf("source-origin index envelope %d exceeds 128-byte charge", 2*indexed)
	}
	lexical := unsafe.Sizeof(sourceOriginSubject{}) + unsafe.Sizeof(Evidence[string]{}) + unsafe.Sizeof(uint64(0))
	if int64(2*lexical) > sourceAttributeLexicalEntryBytes {
		t.Fatalf("source lexical index envelope %d exceeds 256-byte charge", 2*lexical)
	}
	_, s, key := sourceResourceFixture(t, Budgets{})
	name := QName{Local: "id"}
	in := sourceResourceAdmit(t, s, key, SourceOriginPhysical, name, "effective", true)
	slot := sourceOriginSubject{Node: 1, Attribute: name}
	owned := s.sourceOriginSeen[slot]
	published := s.contextResults[len(s.contextResults)-1].SourceOrigin
	if owned != published || owned == &in || owned.Attribute == in.Attribute || owned.Physical.Value.Position == in.Physical.Value.Position {
		t.Fatal("index is not the separately owned published origin")
	}
	in.Specified.Value = false
	in.Attribute.Local = "mutated"
	in.Physical.Value.Position.ByteOffset = 999
	if owned.Specified.Value != true || owned.Attribute.Local != "id" || owned.Physical.Value.Position.ByteOffset != 22 {
		t.Fatalf("origin input mutation reached index: %+v", owned)
	}
	snapshot := stateClone(s.contextResults[len(s.contextResults)-1])
	snapshot.SourceOrigin.Specified.Value = false
	snapshot.SourceOrigin.Attribute.Local = "snapshot"
	snapshot.SourceOrigin.Physical.Value.Position.ByteOffset = 500
	presence, raw, err := s.sourcePhysicalAttribute(context.Background(), s.root, &s.root.attributes[0], stateKnown(true))
	if err != nil || presence.State != EvidenceKnown || !presence.Value || raw.State != EvidenceUnknown || owned.Attribute.Local != "id" || owned.Physical.Value.Position.ByteOffset != 22 {
		t.Fatalf("projection clone mutation reached index: %+v %+v %v", presence, raw, err)
	}
}

func TestSourceResourceModeCloneDigestAndTargetGates(t *testing.T) {
	schema := miniatureSourceSchema(t)
	legacy, err := newSchemaHintContextPlan(schema)
	if err != nil {
		t.Fatal(err)
	}
	source, err := newSourceSchemaHintContextPlan(schema)
	if err != nil {
		t.Fatal(err)
	}
	old, _ := json.Marshal(legacy.context.digest(legacy.kind))
	current, _ := json.Marshal(source.context.digest(source.kind))
	if string(old) == string(current) {
		t.Fatal("source resource mode missing from digest")
	}
	copied := source.clone()
	got, _ := json.Marshal(copied.context.digest(copied.kind))
	if string(current) != string(got) || !copied.context.sourceInfoset {
		t.Fatal("clone changed source resource mode")
	}
	assembly := AssemblyMusicXML
	rule := RuleDefinition{Targets: []Target{TargetModel}, Category: CategoryNormative, Class: ClassMandatory, Role: RoleContextProvider, Scope: ScopeDocument, Selector: Selector{Assembly: &assembly}}
	if err := source.validateFor(rule); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("source resource plan accepted model target: %v", err)
	}
	local, err := NewSourceAttributeValuePlan(localAttr("dacapo"), []Atomic{localString("yes")})
	if err != nil {
		t.Fatal(err)
	}
	rule.Role, rule.Scope = RolePredicate, ScopeNode
	if err := local.validateFor(rule); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("source presence plan accepted model target: %v", err)
	}
	if _, err := newSourceLocalLinkContextPlan(nil); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("nil link schema: %v", err)
	}
	if _, err := newSourceSchemaHintContextPlan(nil); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("nil hint schema: %v", err)
	}
}

func TestSourceResourceModeRequiresSelectedOriginProvider(t *testing.T) {
	e, _, origin := sourceResourceFixture(t, Budgets{})
	profile, _ := e.Profile("source-schema-test")
	profile.CatalogDigest = ""
	required := profile.Required[:0]
	for _, key := range profile.Required {
		if key != origin {
			required = append(required, key)
		}
	}
	profile.Required = required
	support := profile.Support[:0]
	for _, item := range profile.Support {
		if item.Rule != origin {
			support = append(support, item)
		}
	}
	profile.Support = support
	var rules []RuleDefinition
	for _, rule := range e.catalog.rules {
		rules = append(rules, rule)
	}
	var plans []PlanBinding
	for id, plan := range e.catalog.plans {
		plans = append(plans, PlanBinding{ID: id, Plan: plan})
	}
	if _, err := NewCatalog(rules, []Profile{profile}, plans); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("source resource composition without origin provider: %v", err)
	}
}

func TestSourceAttributeLexicalExactAdmissionAndCancellation(t *testing.T) {
	ctx := context.Background()
	name := QName{Local: "id"}
	raw := stateKnown(" original ")
	_, probe, key := sourceResourceFixture(t, Budgets{})
	o := sourceResourceAdmit(t, probe, key, SourceOriginPhysical, name, "effective", true)
	before := rationalUsage(t, probe, BudgetBufferedBytes)
	counter := &rationalCancelContext{Context: ctx, at: 1 << 30}
	if err := probe.observeSourceAttributeLexical(counter, key, o.Subject, name, raw); err != nil {
		t.Fatal(err)
	}
	bytes := rationalUsage(t, probe, BudgetBufferedBytes) - before
	for _, delta := range []int64{-1, 0} {
		_, s, key := sourceResourceFixture(t, Budgets{})
		sourceResourceAdmit(t, s, key, SourceOriginPhysical, name, "effective", true)
		used := rationalUsage(t, s, BudgetBufferedBytes)
		fill := s.account.Limits().MaxBufferedBytes - used - bytes - delta
		if err := s.account.Charge(BudgetBufferedBytes, fill); err != nil {
			t.Fatal(err)
		}
		before := s.retained
		err := s.observeSourceAttributeLexical(ctx, key, o.Subject, name, raw)
		if delta == 0 {
			if err != nil || len(s.sourceAttributeLexical) != 1 || s.retained != before+bytes {
				t.Fatalf("exact admission: %v %+v", err, s.sourceAttributeLexical)
			}
		} else if !errors.Is(err, ErrResourceLimit) || s.sourceAttributeLexical != nil || s.retained != before {
			t.Fatalf("one-under admitted partial raw: %v", err)
		}
		if err := s.account.Release(BudgetBufferedBytes, fill); err != nil {
			t.Fatal(err)
		}
		s.cleanup()
		if used := rationalUsage(t, s, BudgetBufferedBytes); used != 0 {
			t.Fatalf("lexical cleanup leaked %d", used)
		}
	}
	for at := 1; at <= counter.calls; at++ {
		_, s, key := sourceResourceFixture(t, Budgets{})
		sourceResourceAdmit(t, s, key, SourceOriginPhysical, name, "effective", true)
		before := s.retained
		err := s.observeSourceAttributeLexical(&rationalCancelContext{Context: ctx, at: at}, key, o.Subject, name, raw)
		if !errors.Is(err, context.Canceled) || s.sourceAttributeLexical != nil || s.retained != before {
			t.Fatalf("cancellation %d/%d: %v retained=%d/%d", at, counter.calls, err, s.retained, before)
		}
	}
}

func TestSourceHintNoNamespaceValueSurvivesPhysicalAbsence(t *testing.T) {
	_, s, key := sourceResourceFixture(t, Budgets{})
	name := QName{Space: "http://www.w3.org/2001/XMLSchema-instance", Local: "noNamespaceSchemaLocation"}
	o := sourceResourceAdmit(t, s, key, SourceOriginDefault, name, "  hints.xsd  ", false)
	if err := s.observeSourceAttributeLexical(context.Background(), key, o.Subject, name, stateAbsent[string]()); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseNode(context.Background(), schemaLexemeEnd("1")); err != nil {
		t.Fatal(err)
	}
	r := &contextReader{ctx: context.Background(), s: s, sourceInfoset: true}
	if err := projectResourceHintNode(r, RuleKey{"test.source-hints", 1}, s.root); err != nil {
		t.Fatal(err)
	}
	got := s.contextResults[len(s.contextResults)-1].Resource.SchemaHints.NoNamespace
	if got.Presence.State != EvidenceAbsent || got.Raw.State != EvidenceAbsent || got.Value.State != EvidenceKnown || got.Value.Value.Text != "hints.xsd" {
		t.Fatalf("default no-namespace hint: %+v", got)
	}
}

// Compile-time envelopes also establish the host-layout bound when a cross-built
// test binary cannot run on the current kernel.
var _ [sourceOriginIndexEntryBytes - 2*int64(unsafe.Sizeof(sourceOriginSubject{})+unsafe.Sizeof((*SourceOriginContextResult)(nil))+unsafe.Sizeof(uint64(0)))]struct{}
var _ [sourceAttributeLexicalEntryBytes - 2*int64(unsafe.Sizeof(sourceOriginSubject{})+unsafe.Sizeof(Evidence[string]{})+unsafe.Sizeof(uint64(0)))]struct{}

func TestSourceResourceInstrumentReferenceRetainsExactRuleIdentity(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	s.documentID = "source"
	n := localNode(1, 0, "instrument-link")
	original := RuleKey{ID: "MX40-LOCAL-instrument-link-target/local", Revision: 1}
	revised := original
	revised.Revision = 2
	s.decisions = map[decisionID]Decision{
		{rule: original, node: 1}: {Outcome: OutcomeFail, Reason: "original-target-failure"},
		{rule: revised, node: 1}:  {Outcome: OutcomeUnsupported, Reason: "wrong-new-identity"},
	}
	r := &contextReader{ctx: context.Background(), s: s, sourceInfoset: true}
	got := resourceInstrumentTarget(r, n)
	if got.State != EvidenceInvalid || got.Reason != "original-target-failure" || got.Cause == nil || got.Cause.Rule != original {
		t.Fatalf("instrument target identity changed: %+v", got)
	}
}

func TestSourceAttributeLexicalRaisedLimitMidScanCancellation(t *testing.T) {
	raw := stateKnown(strings.Repeat("aé中🪕", 4096))
	_, s, key := sourceResourceFixture(t, Budgets{MaxScalarBytes: 1 << 28})
	name := QName{Local: "id"}
	o := sourceResourceAdmit(t, s, key, SourceOriginPhysical, name, "effective", true)
	before := s.retained
	ctx := &rationalCancelContext{Context: context.Background(), at: 4}
	err := s.observeSourceAttributeLexical(ctx, key, o.Subject, name, raw)
	if !errors.Is(err, context.Canceled) || s.retained != before || s.sourceAttributeLexical != nil {
		t.Fatalf("raw scan ignored cancellation: %v retained=%d/%d", err, s.retained, before)
	}
}

func TestSourceAttributeLexicalChunkBoundariesAndCopyCancellation(t *testing.T) {
	for _, raw := range []string{strings.Repeat("a", resourceCopyChunk-1) + "é", strings.Repeat("a", resourceCopyChunk-2) + "🪕", strings.Repeat("a", resourceCopyChunk) + string([]byte{0x80}), strings.Repeat("a", resourceCopyChunk) + string([]byte{0xf0, 0x9f})} {
		_, s, key := sourceResourceFixture(t, Budgets{MaxScalarBytes: 1 << 28})
		name := QName{Local: "id"}
		o := sourceResourceAdmit(t, s, key, SourceOriginPhysical, name, "effective", true)
		before := s.retained
		err := s.observeSourceAttributeLexical(context.Background(), key, o.Subject, name, stateKnown(raw))
		if utf8.ValidString(raw) {
			if err != nil || s.sourceAttributeLexical[sourceOriginSubject{Node: 1, Attribute: name}].Value != raw {
				t.Fatalf("valid chunk boundary rejected: %v", err)
			}
		} else if !errors.Is(err, ErrInvalidConfiguration) || s.sourceAttributeLexical != nil || s.retained != before {
			t.Fatalf("invalid UTF-8 chunk boundary accepted: %v", err)
		}
	}
	raw := strings.Repeat("aé中🪕", 4096)
	_, probe, key := sourceResourceFixture(t, Budgets{MaxScalarBytes: 1 << 28})
	name := QName{Local: "id"}
	o := sourceResourceAdmit(t, probe, key, SourceOriginPhysical, name, "effective", true)
	counter := &rationalCancelContext{Context: context.Background(), at: 1 << 30}
	if err := probe.observeSourceAttributeLexical(counter, key, o.Subject, name, stateKnown(raw)); err != nil {
		t.Fatal(err)
	}
	if counter.calls < 20 {
		t.Fatalf("too few bounded scan/copy polls: %d", counter.calls)
	}
	for at := 1; at <= counter.calls; at++ {
		_, s, key := sourceResourceFixture(t, Budgets{MaxScalarBytes: 1 << 28})
		sourceResourceAdmit(t, s, key, SourceOriginPhysical, name, "effective", true)
		before := s.retained
		err := s.observeSourceAttributeLexical(&rationalCancelContext{Context: context.Background(), at: at}, key, o.Subject, name, stateKnown(raw))
		if !errors.Is(err, context.Canceled) || s.sourceAttributeLexical != nil || s.retained != before {
			t.Fatalf("scan/copy cancellation %d/%d: %v retained=%d/%d", at, counter.calls, err, s.retained, before)
		}
	}
}

func TestSourceAttributeLexicalBoundsMetadataBeforeSizing(t *testing.T) {
	for _, field := range []string{"namespace", "local-name", "reason", "cause-rule", "cause-document"} {
		t.Run(field, func(t *testing.T) {
			_, s, key := sourceResourceFixture(t, Budgets{})
			name := QName{Local: "id"}
			o := sourceResourceAdmit(t, s, key, SourceOriginPhysical, name, "effective", true)
			raw := stateKnown("original")
			oversized := strings.Repeat("x", 1<<20)
			switch field {
			case "namespace":
				name.Space = oversized
			case "local-name":
				name.Local = oversized
			case "reason":
				raw.Reason = Code(oversized)
			case "cause-rule":
				raw = Evidence[string]{State: EvidenceInvalid, Reason: "invalid-raw", Cause: &DecisionRef{Rule: RuleKey{RuleID(oversized), 1}, Subject: o.Subject}}
			case "cause-document":
				raw = Evidence[string]{State: EvidenceInvalid, Reason: "invalid-raw", Cause: &DecisionRef{Rule: RuleKey{"raw-cause", 1}, Subject: NodeRef{oversized, 1}}}
			}
			before := s.retained
			err := s.observeSourceAttributeLexical(context.Background(), key, o.Subject, name, raw)
			if !errors.Is(err, ErrInvalidConfiguration) || s.sourceAttributeLexical != nil || s.retained != before {
				t.Fatalf("unbounded metadata admission: %v", err)
			}
		})
	}
}
