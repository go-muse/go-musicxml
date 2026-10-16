package validation

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unsafe"

	"github.com/go-muse/go-musicxml/internal/xsd"
)

func TestRetainedFactNodeCompactEnvelope(t *testing.T) {
	if n := unsafe.Sizeof(factNode{}); n > 320 {
		t.Fatalf("retained fact node occupies %d bytes; compact inline envelope is 320", n)
	}
}

func TestRetainedFactCanonicalClosureStorage(t *testing.T) {
	s := representationFixture(t, Budgets{}, false)
	defer s.cleanup()
	representationOpen(t, s)
	for id := NodeID(2); id <= 3; id++ {
		if err := s.Open(context.Background(), representationHeader(id, 1, "test.child", "child")); err != nil {
			t.Fatal(err)
		}
		if err := s.Close(context.Background(), representationEnd(id)); err != nil {
			t.Fatal(err)
		}
	}
	a := reflect.ValueOf(s.nodes[2].end).FieldByName("ChildrenComplete")
	b := reflect.ValueOf(s.nodes[3].end).FieldByName("ChildrenComplete")
	if a.Kind() != reflect.Pointer || b.Kind() != reflect.Pointer {
		t.Fatal("ordinary closure completeness is still stored inline per occurrence")
	}
	if a.Pointer() != b.Pointer() {
		t.Fatal("exact canonical completeness is not shared within session")
	}
}

func TestRetainedFactOpenClosureStaysZero(t *testing.T) {
	s := representationFixture(t, Budgets{}, false)
	defer s.cleanup()
	representationOpen(t, s)
	n := s.nodes[1]
	if n.closed || n.end.ChildrenComplete.State != "" || n.end.AttributesComplete.State != "" || n.end.Order.State != "" || n.end.Scalar.Lexeme.State != "" || n.end.Scalar.Value.State != "" {
		t.Fatal("unclosed occurrence gained closure evidence")
	}
}

// Compile-time envelope guards run on every supported host, including 386.
const _ = uint64(retainedFactNodeBytes) - uint64(unsafe.Sizeof(factNode{}))
const _ = uint64(retainedFactPoolBytes) - uint64(unsafe.Sizeof(retainedFactPool{}))
const _ = uint64(retainedComponentBytes) - uint64(unsafe.Sizeof(Evidence[ComponentID]{}))
const _ = uint64(retainedDeclarationBytes) - uint64(unsafe.Sizeof(Evidence[DeclarationID]{}))
const _ = uint64(retainedLexemeBytes) - uint64(unsafe.Sizeof(Evidence[string]{}))
const _ = uint64(retainedAtomicBytes) - uint64(unsafe.Sizeof(Evidence[Atomic]{}))
const _ = uint64(retainedBoolBytes) - uint64(unsafe.Sizeof(Evidence[bool]{}))
const _ = uint64(retainedOrderBytes) - uint64(unsafe.Sizeof(Evidence[OrderKind]{}))
const _ = uint64(retainedFactPointerBytes) - uint64(unsafe.Sizeof(NodeID(0)))
const _ = uint64(retainedFactPointerBytes) - uint64(unsafe.Sizeof((*factNode)(nil)))

// First-group keys/values and control bytes, and a conservative twofold
// hinted-capacity allowance; old/new map overlap is leased separately.
const _ = uint64(retainedSchemaCacheHeaderBytes) - 8*uint64(unsafe.Sizeof(xsd.CacheIdentity{})+unsafe.Sizeof((*Evidence[ComponentID])(nil))+2)
const _ = uint64(retainedSchemaTypeSlotBytes) - 2*uint64(unsafe.Sizeof(xsd.CacheIdentity{})+unsafe.Sizeof((*Evidence[ComponentID])(nil))+2)
const _ = uint64(retainedSchemaDeclarationSlotBytes) - 2*uint64(unsafe.Sizeof(xsd.CacheIdentity{})+unsafe.Sizeof((*Evidence[DeclarationID])(nil))+2)

const _ = uint64(retainedNodeMapSlotBytes) - 2*uint64(unsafe.Sizeof(NodeID(0))+unsafe.Sizeof((*factNode)(nil))+2)

func retainedStorageSession(t *testing.T) *Session {
	t.Helper()
	account, err := NewBudgetAccount(Budgets{})
	if err != nil {
		t.Fatal(err)
	}
	s := &Session{account: account, documentID: "doc", profile: Profile{Target: TargetModel}}
	if err := s.ensureFactPool(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.cleanup)
	return s
}

func TestRetainedFactEndPreservesEveryEvidence(t *testing.T) {
	cause := &DecisionRef{Rule: RuleKey{ID: "actual-cause", Revision: 7}, Subject: NodeRef{DocumentID: "other-document", Node: 999}}
	for _, state := range []EvidenceState{EvidenceKnown, EvidenceAbsent, EvidenceUnknown, EvidenceUnsupported, EvidenceInvalid} {
		for _, reason := range []Code{"", "arbitrary-reason"} {
			if state != EvidenceKnown && state != EvidenceAbsent && reason == "" {
				continue
			}
			t.Run(string(state)+"/"+string(reason), func(t *testing.T) {
				s := retainedStorageSession(t)
				in := NodeEnd{ID: 99, Scalar: Scalar{Lexeme: Evidence[string]{State: state, Reason: reason}, Value: Evidence[Atomic]{State: state, Reason: reason}}, ChildrenComplete: Evidence[bool]{State: EvidenceKnown, Value: true, Reason: reason}, AttributesComplete: Evidence[bool]{State: EvidenceKnown, Reason: reason}, Order: Evidence[OrderKind]{State: EvidenceKnown, Value: OrderModelContent, Reason: reason}}
				if state == EvidenceKnown {
					in.Scalar.Lexeme.Value = "exact lexical spelling"
					in.Scalar.Value.Value = Atomic{Kind: AtomicString, Text: "exact current value"}
				}
				if state == EvidenceInvalid {
					in.Scalar.Lexeme.Cause = cause
					in.Scalar.Value.Cause = cause
				}
				out, err := s.retainFactEnd(in)
				if err != nil {
					t.Fatal(err)
				}
				if got := out.value(in.ID); !reflect.DeepEqual(got, in) {
					t.Fatalf("evidence round trip changed: got=%+v want=%+v", got, in)
				}
				if state == EvidenceInvalid && (out.Scalar.Value.Cause == cause || out.Scalar.Lexeme.Cause == cause) {
					t.Fatal("caller cause aliases retained evidence")
				}
			})
		}
	}
}

func TestRetainedFactCanonicalIsolationAndExceptions(t *testing.T) {
	a, b := retainedStorageSession(t), retainedStorageSession(t)
	in := representationEnd(1)
	one, err := a.retainFactEnd(in)
	if err != nil {
		t.Fatal(err)
	}
	two, err := a.retainFactEnd(in)
	if err != nil {
		t.Fatal(err)
	}
	other, err := b.retainFactEnd(in)
	if err != nil {
		t.Fatal(err)
	}
	if one.ChildrenComplete != two.ChildrenComplete || one.Order != two.Order || one.Scalar.Lexeme != two.Scalar.Lexeme {
		t.Fatal("canonical records were duplicated")
	}
	if one.ChildrenComplete == other.ChildrenComplete || one.Order == other.Order || one.Scalar.Lexeme == other.Scalar.Lexeme {
		t.Fatal("canonical records crossed sessions")
	}
	for _, value := range []Evidence[bool]{{State: EvidenceKnown, Value: true, Reason: "detail"}, {State: EvidenceUnknown, Reason: "missing"}, {State: EvidenceUnsupported, Reason: "not-supported"}, {State: EvidenceInvalid, Reason: "bad", Cause: &DecisionRef{Rule: RuleKey{ID: "bad", Revision: 1}, Subject: NodeRef{"doc", 4}}}} {
		x, err := a.retainBool(value)
		if err != nil {
			t.Fatal(err)
		}
		y, err := a.retainBool(value)
		if err != nil {
			t.Fatal(err)
		}
		if x == y || !reflect.DeepEqual(*x, value) || !reflect.DeepEqual(*y, value) {
			t.Fatal("exceptional evidence was collapsed")
		}
	}
	replacement, err := a.retainOrder(Evidence[OrderKind]{State: EvidenceUnknown, Reason: "only-one"})
	if err != nil {
		t.Fatal(err)
	}
	one.Order = replacement
	if two.Order.State != EvidenceKnown || other.Order.State != EvidenceKnown {
		t.Fatal("one-node replacement changed sibling or session")
	}
}

func TestRetainedFactUniqueAdmissionAndCleanup(t *testing.T) {
	for _, short := range []int64{0, 1} {
		t.Run(fmt.Sprint(short), func(t *testing.T) {
			s := retainedStorageSession(t)
			value := Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicString, Text: "unique"}, Reason: "preserve"}
			cost := retainedAtomicBytes + int64(len(value.Value.Text)) + int64(len(value.Reason))
			used, _ := s.account.Usage(BudgetBufferedBytes)
			hold := s.account.Limits().MaxBufferedBytes - used - cost + short
			if err := s.account.Charge(BudgetBufferedBytes, hold); err != nil {
				t.Fatal(err)
			}
			out, err := s.retainAtomic(value)
			if short == 0 {
				if err != nil || out == nil || !reflect.DeepEqual(*out, value) {
					t.Fatalf("exact admission=%+v %v", out, err)
				}
			} else if !errors.Is(err, ErrResourceLimit) || out != nil {
				t.Fatalf("one-short published evidence: %+v %v", out, err)
			}
			_ = s.account.Release(BudgetBufferedBytes, hold)
			s.cleanup()
			if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
				t.Fatalf("cleanup leaked %d", used)
			}
		})
	}
}

func TestRetainedFactPoolAdmissionAndCleanup(t *testing.T) {
	for _, short := range []int64{0, 1} {
		account, err := NewBudgetAccount(Budgets{MaxBufferedBytes: retainedFactPoolBytes - short})
		if err != nil {
			t.Fatal(err)
		}
		s := &Session{account: account}
		err = s.ensureFactPool()
		if short == 0 {
			if err != nil || s.facts == nil {
				t.Fatalf("exact pool admission: %v", err)
			}
		} else if !errors.Is(err, ErrResourceLimit) || s.facts != nil {
			t.Fatalf("pool allocated before admission: %v", err)
		}
		s.cleanup()
		if used, _ := account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("pool leaked %d", used)
		}
	}
}

func TestRetainedFactSchemaCacheAdmissionAndIdentity(t *testing.T) {
	for _, short := range []int64{0, 1} {
		s := retainedStorageSession(t)
		typeKeys, _ := retainedTestSchema(t, s, 2)
		typ := Evidence[ComponentID]{State: EvidenceKnown, Value: ComponentID{Assembly: AssemblyMusicXML, Kind: ComponentType, Name: QName{Local: "note"}}}
		cost := retainedSchemaCacheHeaderBytes + 8*retainedSchemaTypeSlotBytes + retainedComponentBytes
		used, _ := s.account.Usage(BudgetBufferedBytes)
		hold := s.account.Limits().MaxBufferedBytes - used - cost + short
		if err := s.account.Charge(BudgetBufferedBytes, hold); err != nil {
			t.Fatal(err)
		}
		out, err := s.retainSchemaType(context.Background(), typeKeys[0], typ)
		if short == 0 {
			if err != nil || out == nil {
				t.Fatalf("exact cache admission=%v", err)
			}
			again, err := s.retainSchemaType(context.Background(), typeKeys[0], typ)
			if err != nil || again != out {
				t.Fatal("exact compiled identity was not reused")
			}
		} else if !errors.Is(err, ErrResourceLimit) || out != nil || len(s.facts.types) != 0 {
			t.Fatal("unadmitted cache entry published")
		}
		_ = s.account.Release(BudgetBufferedBytes, hold)
		s.cleanup()
		if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("cache leaked %d", used)
		}
	}
	s := retainedStorageSession(t)
	_, declarationKeys := retainedTestSchema(t, s, 2)
	a := Evidence[DeclarationID]{State: EvidenceKnown, Value: DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "owner-one", Occurrence: "same-name"}}
	b := a
	b.Value.OwnerOccurrence = "owner-two"
	x, err := s.retainSchemaDeclaration(context.Background(), declarationKeys[0], a)
	if err != nil {
		t.Fatal(err)
	}
	y, err := s.retainSchemaDeclaration(context.Background(), declarationKeys[1], b)
	if err != nil {
		t.Fatal(err)
	}
	if x == y || x.Value == y.Value {
		t.Fatal("distinct exact declaration uses were merged")
	}
}

func TestRetainedFactReplacementFailureIsAtomic(t *testing.T) {
	for _, short := range []int64{0, 1} {
		s := retainedStorageSession(t)
		end, err := s.retainFactEnd(representationEnd(1))
		if err != nil {
			t.Fatal(err)
		}
		node := &factNode{end: end, modelRepresentation: &modelRepresentationRecord{Problems: []ModelProblem{{Axis: ModelAxisChildren, Code: "model-children-invalid"}}}}
		sibling := &factNode{end: end}
		decision := Decision{Rule: RuleKey{ID: "representation", Revision: 1}, Subject: NodeRef{"doc", 1}}
		cost := int64(128+len(decision.Rule.ID)+len(decision.Subject.DocumentID)) + retainedBoolBytes + retainedOrderBytes
		used, _ := s.account.Usage(BudgetBufferedBytes)
		hold := s.account.Limits().MaxBufferedBytes - used - cost + short
		if err := s.account.Charge(BudgetBufferedBytes, hold); err != nil {
			t.Fatal(err)
		}
		err = s.invalidateModelRepresentation(context.Background(), node, decision)
		if short == 0 {
			if err != nil || node.end.Order.State != EvidenceInvalid || node.end.ChildrenComplete.State != EvidenceInvalid {
				t.Fatalf("exact invalidation=%v", err)
			}
		} else if !errors.Is(err, ErrResourceLimit) || node.end.Order != end.Order || node.end.ChildrenComplete != end.ChildrenComplete {
			t.Fatalf("partial invalidation=%v", err)
		}
		if sibling.end.Order.State != EvidenceKnown || sibling.end.ChildrenComplete.State != EvidenceKnown {
			t.Fatal("invalidation changed sibling")
		}
		_ = s.account.Release(BudgetBufferedBytes, hold)
		s.cleanup()
		if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("invalidation leaked %d", used)
		}
	}
}

func TestRetainedFactSnapshotOutlivesPool(t *testing.T) {
	s := snapshotTestSession(t, "not-an-integer")
	_, snapshot, err := s.FinishWithResourceSnapshot(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed})
	if err != nil {
		t.Fatal(err)
	}
	before, err := snapshot.Report(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if s.facts != nil {
		t.Fatal("finished session retained evidence pool")
	}
	another := snapshotTestSession(t, "42")
	if _, err := another.Finish(context.Background(), Completion{InputComplete: true, Syntax: SyntaxWellFormed}); err != nil {
		t.Fatal(err)
	}
	after, err := snapshot.Report(context.Background())
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("independent assessment changed snapshot: %v", err)
	}
	after.Findings[0].Location.Path = "mutated getter"
	final, err := snapshot.Report(context.Background())
	if err != nil || !reflect.DeepEqual(before, final) {
		t.Fatal("snapshot getter exposes compact retained storage")
	}
	if err := snapshot.Close(); err != nil {
		t.Fatal(err)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("snapshot leaked %d", used)
	}
}

func TestRetainedFactRejectedRootAdmission(t *testing.T) {
	for _, short := range []int64{0, 1} {
		s := representationFixture(t, Budgets{}, false)
		h := representationHeader(1, 0, "test.root", "unexpected")
		h.Name.Space = strings.Repeat("n", 400)
		cost := int64(32 + len(h.Name.Space) + len(h.Name.Local))
		used, _ := s.account.Usage(BudgetBufferedBytes)
		hold := s.account.Limits().MaxBufferedBytes - used - cost + short
		if err := s.account.Charge(BudgetBufferedBytes, hold); err != nil {
			t.Fatal(err)
		}
		err := s.Open(context.Background(), h)
		if short == 0 {
			var rejected *UnsupportedRootError
			if !errors.As(err, &rejected) || s.rejectedRootName != h.Name {
				t.Fatalf("exact rejected-root admission=%v", err)
			}
		} else if !errors.Is(err, ErrResourceLimit) || s.rejectedRootName != (QName{}) {
			t.Fatalf("unadmitted root copied: %v", err)
		}
		if s.root != nil || len(s.nodes) != 0 {
			t.Fatal("rejected root became an admitted occurrence")
		}
		_ = s.account.Release(BudgetBufferedBytes, hold)
		s.cleanup()
		if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("rejected root leaked %d", used)
		}
	}
}

func TestRetainedFactBindingValidationAdmission(t *testing.T) {
	for _, short := range []int64{0, 1} {
		s := retainedStorageSession(t)
		h := representationHeader(1, 0, "test.root", "root")
		h.Bindings = []Binding{{"a", "urn:a"}, {"b", "urn:b"}}
		cost := int64(256 + len(h.Bindings)*96)
		before, _ := s.account.Usage(BudgetBufferedBytes)
		hold := s.account.Limits().MaxBufferedBytes - before - cost + short
		if err := s.account.Charge(BudgetBufferedBytes, hold); err != nil {
			t.Fatal(err)
		}
		err := s.validateRetainedHeader(h)
		if short == 0 && err != nil {
			t.Fatal(err)
		}
		if short != 0 && !errors.Is(err, ErrResourceLimit) {
			t.Fatalf("one-short validation=%v", err)
		}
		_ = s.account.Release(BudgetBufferedBytes, hold)
		if used, _ := s.account.Usage(BudgetBufferedBytes); used != before {
			t.Fatalf("temporary validation map leaked %d", used-before)
		}
	}
}

func TestRetainedFactVectorCapacityAdmission(t *testing.T) {
	for _, capacity := range []int{0, 1, 2, 8, 32} {
		for _, short := range []int64{0, 1} {
			s := retainedStorageSession(t)
			if err := s.retain(int64(capacity) * retainedFactPointerBytes); err != nil {
				t.Fatal(err)
			}
			original := make([]NodeID, capacity)
			for i := range original {
				original[i] = NodeID(i + 1)
			}
			newCapacity := max(1, capacity*2)
			cost := int64(newCapacity) * retainedFactPointerBytes
			before, _ := s.account.Usage(BudgetBufferedBytes)
			hold := s.account.Limits().MaxBufferedBytes - before - cost + short
			if err := s.account.Charge(BudgetBufferedBytes, hold); err != nil {
				t.Fatal(err)
			}
			out, err := growFactSlice(context.Background(), s, original)
			if short == 0 {
				if err != nil || cap(out) != newCapacity || !reflect.DeepEqual(out, original) {
					t.Fatalf("growth=%v cap=%d", err, cap(out))
				}
			} else if !errors.Is(err, ErrResourceLimit) || cap(out) != capacity {
				t.Fatal("unadmitted vector growth")
			}
			_ = s.account.Release(BudgetBufferedBytes, hold)
			s.cleanup()
			if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
				t.Fatalf("vector leaked %d", used)
			}
		}
	}
}

func TestRetainedFactNodeMapGrowthAndCancellation(t *testing.T) {
	for _, oldCapacity := range []int{0, 8, 16, 32} {
		for _, short := range []int64{0, 1} {
			s := retainedStorageSession(t)
			if oldCapacity > 0 {
				s.nodeCapacity = oldCapacity
				oldBytes := retainedNodeMapHeaderBytes + int64(oldCapacity)*retainedNodeMapSlotBytes
				if err := s.retain(oldBytes); err != nil {
					t.Fatal(err)
				}
				s.nodes = make(map[NodeID]*factNode, oldCapacity)
				for i := 0; i < oldCapacity; i++ {
					s.nodes[NodeID(i+1)] = &factNode{}
				}
			}
			before, _ := s.account.Usage(BudgetBufferedBytes)
			cost := retainedNodeMapHeaderBytes + int64(max(8, oldCapacity*2))*retainedNodeMapSlotBytes
			hold := s.account.Limits().MaxBufferedBytes - before - cost + short
			if err := s.account.Charge(BudgetBufferedBytes, hold); err != nil {
				t.Fatal(err)
			}
			err := s.growFactNodes(context.Background())
			if short == 0 {
				if err != nil || s.nodeCapacity != max(8, oldCapacity*2) || len(s.nodes) != oldCapacity {
					t.Fatalf("map growth=%v capacity=%d", err, s.nodeCapacity)
				}
			} else if !errors.Is(err, ErrResourceLimit) || s.nodeCapacity != oldCapacity {
				t.Fatal("unadmitted map growth")
			}
			_ = s.account.Release(BudgetBufferedBytes, hold)
			s.cleanup()
			if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
				t.Fatalf("map leaked %d", used)
			}
		}
	}
	s := retainedStorageSession(t)
	before, _ := s.account.Usage(BudgetBufferedBytes)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := s.growFactNodes(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if used, _ := s.account.Usage(BudgetBufferedBytes); used != before || s.nodes != nil || s.nodeCapacity != 0 {
		t.Fatal("canceled map growth changed retained storage")
	}
}

func TestRetainedFactSchemaCacheKeyWorkAndGrowth(t *testing.T) {
	for _, count := range []int{0, 1, 7, 8, 15, 16, 31, 32} {
		for _, short := range []int64{0, 1} {
			t.Run(fmt.Sprintf("entries-%d/short-%d", count, short), func(t *testing.T) {
				s := retainedStorageSession(t)
				typeKeys, _ := retainedTestSchema(t, s, count+1)
				key := ComponentID{Assembly: AssemblyMusicXML, Kind: ComponentType, Name: QName{Space: "schema-space", Local: "type-000"}}
				for i := 0; i < count; i++ {
					key.Name.Local = fmt.Sprintf("type-%03d", i)
					if _, err := s.retainSchemaType(context.Background(), typeKeys[i], Evidence[ComponentID]{State: EvidenceKnown, Value: key}); err != nil {
						t.Fatal(err)
					}
				}
				// Independent arithmetic: the compiled owner/ordinal key has fixed
				// width, so each lookup, insertion and growth-copy costs one unit.
				key.Name.Local = "type-new"
				keyWork := int64(1)
				want := keyWork
				if count != 0 {
					want += keyWork
				}
				if count == s.facts.typeCapacity {
					want += 1 + int64(count)*keyWork
				}
				used, _ := s.account.Usage(BudgetRuleSteps)
				limit := s.account.Limits().MaxRuleSteps
				hold := limit - used - want + short
				if err := s.account.Charge(BudgetRuleSteps, hold); err != nil {
					t.Fatal(err)
				}
				out, err := s.retainSchemaType(context.Background(), typeKeys[count], Evidence[ComponentID]{State: EvidenceKnown, Value: key})
				if short == 0 {
					if err != nil || out == nil || len(s.facts.types) != count+1 {
						t.Fatalf("exact key/copy work=%d: %v", want, err)
					}
					got, _ := s.account.Usage(BudgetRuleSteps)
					if got != used+hold+want {
						t.Fatalf("key/copy work delta=%d want=%d", got-used-hold, want)
					}
				} else if !errors.Is(err, ErrResourceLimit) || out != nil || len(s.facts.types) != count {
					t.Fatalf("one-short published cache entry: count=%d err=%v", len(s.facts.types), err)
				}
				s.cleanup()
				if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
					t.Fatalf("cache growth leaked %d", used)
				}
			})
		}
	}
}

func TestRetainedFactSchemaCacheCapacityAdmission(t *testing.T) {
	for _, capacity := range []int{0, 8, 16, 32} {
		for _, short := range []int64{0, 1} {
			t.Run(fmt.Sprintf("capacity-%d/short-%d", capacity, short), func(t *testing.T) {
				s := retainedStorageSession(t)
				_, declarationKeys := retainedTestSchema(t, s, capacity+1)
				for i := 0; i < capacity; i++ {
					value := Evidence[DeclarationID]{State: EvidenceKnown, Value: DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "owner", Occurrence: fmt.Sprint(i)}}
					if _, err := s.retainSchemaDeclaration(context.Background(), declarationKeys[i], value); err != nil {
						t.Fatal(err)
					}
				}
				next := max(8, 2*capacity)
				newMap := retainedSchemaCacheHeaderBytes + int64(next)*retainedSchemaDeclarationSlotBytes
				cost := retainedDeclarationBytes + newMap
				used, _ := s.account.Usage(BudgetBufferedBytes)
				limit := s.account.Limits().MaxBufferedBytes
				hold := limit - used - cost + short
				if err := s.account.Charge(BudgetBufferedBytes, hold); err != nil {
					t.Fatal(err)
				}
				out, err := s.retainSchemaDeclaration(context.Background(), declarationKeys[capacity], Evidence[DeclarationID]{State: EvidenceKnown, Value: DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "owner", Occurrence: "new"}})
				if short == 0 {
					if err != nil || out == nil || s.facts.declarationCapacity != next {
						t.Fatalf("exact capacity lease: %v", err)
					}
					oldMap := int64(0)
					if capacity != 0 {
						oldMap = retainedSchemaCacheHeaderBytes + int64(capacity)*retainedSchemaDeclarationSlotBytes
					}
					got, _ := s.account.Usage(BudgetBufferedBytes)
					if got != used+hold+cost-oldMap {
						t.Fatalf("live cache lease=%d want=%d", got, used+hold+cost-oldMap)
					}
				} else if !errors.Is(err, ErrResourceLimit) || out != nil || s.facts.declarationCapacity != capacity || len(s.facts.declarations) != capacity {
					t.Fatalf("unadmitted capacity published: %v", err)
				}
				_ = s.account.Release(BudgetBufferedBytes, hold)
				s.cleanup()
				if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
					t.Fatalf("cache growth leaked %d", used)
				}
			})
		}
	}
}

func TestRetainedFactSchemaCacheCancellationAndCausalEvidence(t *testing.T) {
	s := retainedStorageSession(t)
	typeKeys, _ := retainedTestSchema(t, s, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	v := Evidence[ComponentID]{State: EvidenceKnown, Value: ComponentID{Assembly: AssemblyMusicXML, Kind: ComponentType, Name: QName{Local: "one"}}}
	if _, err := s.retainSchemaType(ctx, typeKeys[0], v); !errors.Is(err, context.Canceled) || len(s.facts.types) != 0 {
		t.Fatalf("canceled key operation: %v", err)
	}
	v.Cause = &DecisionRef{Rule: RuleKey{ID: "reason", Revision: 1}, Subject: NodeRef{DocumentID: "doc", Node: 1}}
	a, err := s.retainSchemaType(context.Background(), typeKeys[0], v)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.retainSchemaType(context.Background(), typeKeys[0], v)
	if err != nil {
		t.Fatal(err)
	}
	if a == b || a.Cause == v.Cause || b.Cause == v.Cause || a.Cause == b.Cause || len(s.facts.types) != 0 {
		t.Fatal("causal compiled evidence was shared or cached")
	}
}

func TestRetainedFactAtomicKindsAndHeaderOwnership(t *testing.T) {
	s := retainedStorageSession(t)
	integer := localInteger(t, -9007199254740993)
	decimal := Atomic{Kind: AtomicDecimal, Decimal: localDecimal(t, "-123456789.000000001")}
	for _, value := range []Atomic{{Kind: AtomicString, Text: "text"}, {Kind: AtomicBoolean, Text: "true"}, {Kind: AtomicBoolean, Text: "false"}, integer, decimal, {Kind: AtomicQName, Name: QName{Space: "arbitrary-namespace", Local: "local"}}} {
		in := Evidence[Atomic]{State: EvidenceKnown, Value: value, Reason: "known-with-reason"}
		out, err := s.retainAtomic(in)
		if err != nil || !reflect.DeepEqual(*out, in) {
			t.Fatalf("atomic %s round trip: %+v %v", value.Kind, out, err)
		}
	}
	cause := &DecisionRef{Rule: RuleKey{ID: "caller-rule", Revision: 7}, Subject: NodeRef{DocumentID: "cause-doc", Node: 10}}
	in := representationHeader(99, 0, "declaration", "root")
	in.Type.Cause = cause
	in.Location.Position = &SourcePosition{Line: 23, Column: 17, CoordinateSpace: "caller-position"}
	in.Bindings = []Binding{{Prefix: "p", URI: "caller-namespace"}}
	want := cloneHeader(in)
	out, err := s.retainFactHeader(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	cause.Rule.ID = "mutated-rule"
	in.Location.Position.Line = 999
	in.Bindings[0].URI = "mutated-namespace"
	if got := out.value(); !reflect.DeepEqual(got, want) {
		t.Fatalf("caller mutation reached retained header: got=%+v want=%+v", got, want)
	}
}

func TestRetainedFactInvalidHeaderPrecedesNewStorageAdmission(t *testing.T) {
	s := representationFixture(t, Budgets{}, false)
	account := s.account
	used, _ := account.Usage(BudgetBufferedBytes)
	// Validation's duplicate-binding scratch fits, but the pool/node do not.
	hold := account.Limits().MaxBufferedBytes - used - 256
	if err := account.Charge(BudgetBufferedBytes, hold); err != nil {
		t.Fatal(err)
	}
	h := representationHeader(1, 0, "root", "root")
	h.Name.Local = ""
	err := s.Open(context.Background(), h)
	var session *SessionError
	if !errors.As(err, &session) || errors.Is(err, ErrResourceLimit) || s.facts != nil || len(s.nodes) != 0 {
		t.Fatalf("invalid header gained storage or changed precedence: %v", err)
	}
	_ = account.Release(BudgetBufferedBytes, hold)
	s.cleanup()
	if used, _ := account.Usage(BudgetBufferedBytes); used != 0 {
		t.Fatalf("invalid header leaked %d", used)
	}
}

func TestRetainedFactPendingSourceSharingIsExact(t *testing.T) {
	s := retainedStorageSession(t)
	typ := Evidence[ComponentID]{State: EvidenceUnknown, Reason: "schema-binding-pending"}
	decl := Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "schema-binding-pending"}
	atomic := Evidence[Atomic]{State: EvidenceUnknown, Reason: "schema-binding-pending"}
	a, err := s.retainType(typ)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.retainDeclaration(decl)
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.retainAtomic(atomic)
	if err != nil {
		t.Fatal(err)
	}
	if a != &s.facts.pendingType || b != &s.facts.pendingDeclaration || c != &s.facts.pendingAtomic {
		t.Fatal("exact pending-source facts were not reused")
	}
	other := retainedStorageSession(t)
	d, err := other.retainAtomic(atomic)
	if err != nil {
		t.Fatal(err)
	}
	if c == d {
		t.Fatal("pending-source evidence crosses sessions")
	}
	for _, mutate := range []func(*Evidence[Atomic]){
		func(v *Evidence[Atomic]) { v.Reason = "other-pending-reason" },
		func(v *Evidence[Atomic]) { v.Value = Atomic{Kind: AtomicString, Text: "nonzero"} },
		func(v *Evidence[Atomic]) {
			v.Cause = &DecisionRef{Rule: RuleKey{ID: "cause"}, Subject: NodeRef{DocumentID: "source", Node: 1}}
		},
	} {
		v := atomic
		mutate(&v)
		out, err := s.retainAtomic(v)
		if err != nil {
			t.Fatal(err)
		}
		if out == c || !reflect.DeepEqual(*out, v) {
			t.Fatal("noncanonical unknown source evidence collapsed")
		}
	}
}

func retainedTestSchema(t *testing.T, s *Session, count int) ([]xsd.CacheIdentity, []xsd.CacheIdentity) {
	t.Helper()
	data := SchemaData{Types: map[SchemaName]*SchemaType{}, Elements: map[SchemaName]*SchemaElement{}}
	for i := 0; i < count; i++ {
		name := SchemaName{Local: fmt.Sprintf("type-%03d", i)}
		data.Types[name] = &SchemaType{Simple: &SchemaSimple{}}
		use := SchemaName{Local: fmt.Sprintf("use-%03d", i)}
		data.Elements[use] = &SchemaElement{Name: use, Type: SchemaTypeRef{Name: name}}
	}
	schema, err := CompileSchema(AssemblyMusicXML, data)
	if err != nil {
		t.Fatal(err)
	}
	s.schema = &schemaSession{active: schema}
	types := make([]xsd.CacheIdentity, 0, count)
	for _, typ := range schema.kernel.Components() {
		if typ.Name().Space == "" && strings.HasPrefix(typ.Name().Local, "type-") {
			types = append(types, typ.CacheIdentity())
		}
	}
	uses := make([]xsd.CacheIdentity, 0, count)
	for i := 0; i < count; i++ {
		use, ok := schema.kernel.Root(xsd.QName{Local: fmt.Sprintf("use-%03d", i)})
		if !ok {
			t.Fatal("missing test schema use")
		}
		uses = append(uses, use.CacheIdentity())
	}
	return types, uses
}

func TestRetainedFactSchemaCacheRejectsForeignAndZeroIdentity(t *testing.T) {
	s := retainedStorageSession(t)
	own, _ := retainedTestSchema(t, s, 1)
	other := retainedStorageSession(t)
	foreign, _ := retainedTestSchema(t, other, 1)
	v := Evidence[ComponentID]{State: EvidenceKnown, Value: ComponentID{Assembly: AssemblyMusicXML, Kind: ComponentType, Name: QName{Local: "one"}}}
	for _, key := range []xsd.CacheIdentity{{}, foreign[0]} {
		a, err := s.retainSchemaType(context.Background(), key, v)
		if err != nil {
			t.Fatal(err)
		}
		b, err := s.retainSchemaType(context.Background(), key, v)
		if err != nil {
			t.Fatal(err)
		}
		if a == b || len(s.facts.types) != 0 || !reflect.DeepEqual(*a, v) {
			t.Fatal("foreign/zero identity entered schema cache")
		}
	}
	for _, state := range []EvidenceState{EvidenceUnknown, EvidenceInvalid, EvidenceUnsupported} {
		unknown := Evidence[ComponentID]{State: state, Reason: "not-known"}
		a, err := s.retainSchemaType(context.Background(), own[0], unknown)
		if err != nil {
			t.Fatal(err)
		}
		b, err := s.retainSchemaType(context.Background(), own[0], unknown)
		if err != nil {
			t.Fatal(err)
		}
		if a == b || len(s.facts.types) != 0 || !reflect.DeepEqual(*a, unknown) {
			t.Fatal("non-known evidence entered schema cache")
		}
	}
}

func TestRetainedFactSchemaCacheGrowthRefusesOverflow(t *testing.T) {
	maxInt := int64(^uint(0) >> 1)
	for _, tc := range []struct {
		name                string
		capacity, slotBytes int64
	}{
		{"multiplication", 1<<63 - 1, retainedSchemaTypeSlotBytes},
		{"header-addition", (1<<63 - 1) - retainedSchemaCacheHeaderBytes + 1, 1},
		{"native-allocation-width", maxInt/retainedSchemaTypeSlotBytes + 1, retainedSchemaTypeSlotBytes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := retainedSchemaCacheBytes(tc.capacity, tc.slotBytes); !errors.Is(err, ErrResourceLimit) {
				t.Fatalf("production cache capacity guard did not refuse overflow: %v", err)
			}
		})
	}
}

func TestRetainedFactDeclarationCacheCancellationAndCausalEvidence(t *testing.T) {
	s := retainedStorageSession(t)
	_, keys := retainedTestSchema(t, s, 1)
	v := Evidence[DeclarationID]{State: EvidenceKnown, Value: DeclarationID{Assembly: AssemblyMusicXML, OwnerOccurrence: "owner", Occurrence: "one"}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if out, err := s.retainSchemaDeclaration(ctx, keys[0], v); !errors.Is(err, context.Canceled) || out != nil || len(s.facts.declarations) != 0 {
		t.Fatalf("canceled declaration operation: %+v, %v", out, err)
	}
	for _, variant := range []string{"cause", "reason"} {
		t.Run(variant, func(t *testing.T) {
			in := v
			if variant == "cause" {
				in.Cause = &DecisionRef{Rule: RuleKey{ID: "reason", Revision: 1}, Subject: NodeRef{DocumentID: "doc", Node: 1}}
			} else {
				in.Reason = "known-with-reason"
			}
			a, err := s.retainSchemaDeclaration(context.Background(), keys[0], in)
			if err != nil {
				t.Fatal(err)
			}
			b, err := s.retainSchemaDeclaration(context.Background(), keys[0], in)
			if err != nil {
				t.Fatal(err)
			}
			if a == b || len(s.facts.declarations) != 0 || !reflect.DeepEqual(*a, in) || !reflect.DeepEqual(*b, in) {
				t.Fatal("noncanonical declaration evidence was shared, cached or changed")
			}
			if in.Cause != nil && (a.Cause == in.Cause || b.Cause == in.Cause || a.Cause == b.Cause) {
				t.Fatal("causal declaration evidence retained caller/shared cause")
			}
		})
	}
}
