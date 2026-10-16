package validation

import (
	"context"
	"testing"
)

func TestLocalRound1MalformedKnownFretIsUnavailable(t *testing.T) {
	s, n, r := localEvidenceFixture(t, EvidenceKnown, localFret)
	defer s.cleanup()
	n.end.Scalar.Value = testEvidence(localKnownText("not-an-integer"))
	if err := localStructuralNode(r.ctx, s, RuleKey{"local-test", 1}, n, r); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, row := range s.contextResults {
		if row.Property == ContextOpenString {
			count++
			if row.Value.State != EvidenceUnknown {
				t.Fatalf("malformed atom became a known answer: %+v", row)
			}
		}
	}
	if count != 1 {
		t.Fatalf("rows %d", count)
	}
}
func TestLocalRound1MalformedColorNeverPanicsOrClaimsKnown(t *testing.T) {
	for _, v := range []string{"#-1ABCDEF", "#GG123456", "#12345G", "!123456", "#abcdef", "#１２３４５６", "#12345", "#123456789"} {
		t.Run(v, func(t *testing.T) {
			s, n, r := localEvidenceFixture(t, EvidenceKnown, "")
			defer s.cleanup()
			r.b.Attributes[localColor] = map[ComponentID]AttributeSelector{n.header.Type.Value: localAttr("color")}
			localSetAttr(n, "color", localString(v))
			if err := localStructuralNode(r.ctx, s, RuleKey{"local-test", 1}, n, r); err != nil {
				t.Fatal(err)
			}
			for _, row := range s.contextResults {
				if (row.Property == ContextColorRGB || row.Property == ContextColorAlpha) && row.Value.State == EvidenceKnown {
					t.Fatalf("malformed color became known: %+v", row)
				}
			}
		})
	}
}
func TestLocalRound1AbsentMetadataDoesNotInventDefault(t *testing.T) {
	s, n, r := localEvidenceFixture(t, EvidenceKnown, localCreator)
	defer s.cleanup()
	r.b.Attributes[localMetadataType] = map[ComponentID]AttributeSelector{n.header.Type.Value: localAttr("type")}
	if err := localStructuralNode(r.ctx, s, RuleKey{"local-test", 1}, n, r); err != nil {
		t.Fatal(err)
	}
	for _, row := range s.contextResults {
		if row.Property == ContextMetadataRole && row.Layout.Provenance == ContextDocumentedDefault {
			t.Fatal("absence is not a documented default")
		}
	}
}
func TestLocalRound1FontConstructionDoesNotRetainTemporary(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	before, _ := s.account.Usage(BudgetBufferedBytes)
	if _, err := localFontCategories(context.Background(), s, localKnownText("music, serif")); err != nil {
		t.Fatal(err)
	}
	after, _ := s.account.Usage(BudgetBufferedBytes)
	if after != before {
		t.Fatalf("temporary retained: before %d after %d", before, after)
	}
}
func TestLocalRound1AdvisoryPropertyValidationDoesNotAllocate(t *testing.T) {
	if got := testing.AllocsPerRun(100, func() {
		if !localAdvisoryPropertyValid("local.advisory-integral-divisions.offset") {
			panic("missing property")
		}
	}); got != 0 {
		t.Fatalf("allocations %g", got)
	}
}

func TestLocalRound1FigureNumberSufficientNumericSubset(t *testing.T) {
	for _, text := range []string{"6", "10", "0006", "99999999999999999999999999999999999999999999999999999999999999999"} {
		s, n, _ := localEvidenceFixture(t, EvidenceKnown, localFigureNumber)
		n.end.Scalar.Value = testEvidence(localKnownText(text))
		got, _, err := evaluateFigureNumberNumeric(context.Background(), s, n)
		if err != nil || got.Outcome != OutcomePass {
			t.Errorf("%q: %+v %v", text, got, err)
		}
		s.cleanup()
	}
}

func TestLocalRound1SkippedRootDoesNotReusePriorHarmony(t *testing.T) {
	s, n, r := localEvidenceFixture(t, EvidenceKnown, localKind)
	defer s.cleanup()
	n.end.Scalar.Value = testEvidence(localKnownText("none"))
	parent := localNode(1, 0, "harmony")
	prior := localNode(3, 1, "prior-root")
	skipped := localNode(4, 1, "skipped-root")
	skipped.header.Declaration = testEvidence(Evidence[DeclarationID]{State: EvidenceUnknown, Reason: "skipped-declaration"})
	parent.children = []*factNode{prior, skipped, n}
	parent.end.ChildrenComplete = testEvidence(Evidence[bool]{State: EvidenceUnknown, Reason: "skipped-child", Cause: &DecisionRef{Rule: RuleKey{"input-gap", 1}, Subject: NodeRef{"doc", 4}}})
	s.nodes[1] = parent
	s.nodes[3] = prior
	s.nodes[4] = skipped
	r.b.Roles[localAdvisoryRole("root")] = []SchemaUse{{Declaration: prior.header.Declaration.Value}}
	if err := localAdvisoryNode(r.ctx, s, RuleKey{"local-test", 1}, n, r); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, row := range s.contextResults {
		if row.Property == "local.advisory-harmony-none-root" {
			count++
			if row.Layout.Applicability.State != EvidenceUnknown || row.Layout.Applicability.Cause == nil {
				t.Fatalf("stale association: %+v", row)
			}
		}
	}
	if count != 1 {
		t.Fatalf("rows %d", count)
	}
}
func TestLocalRound1WrongKnownDivisionsIsUnavailable(t *testing.T) {
	s, n, r := localEvidenceFixture(t, EvidenceKnown, localAdvisoryRole("duration"))
	defer s.cleanup()
	n.end.Scalar.Value = testEvidence(localKnownText("not-decimal"))
	if err := localAdvisoryNode(r.ctx, s, RuleKey{"local-test", 1}, n, r); err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, row := range s.contextResults {
		if row.Property == "local.advisory-integral-divisions" {
			count++
			if row.Layout.Applicability.State != EvidenceUnknown || row.Value.State != EvidenceUnknown {
				t.Fatalf("known false: %+v", row)
			}
		}
	}
	if count != 1 {
		t.Fatalf("rows %d", count)
	}
}

func TestLocalRound1FontTemporaryHeldUntilOwnedProjection(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	s.documentID = "doc"
	raw := localKnownText("music, serif")
	value, release, err := localFontCategoriesOwned(context.Background(), s, raw)
	if err != nil {
		t.Fatal(err)
	}
	held, _ := s.account.Usage(BudgetBufferedBytes)
	if held <= 0 || release == nil {
		t.Fatal("builder was not admitted")
	}
	n := localNode(1, 0, "font")
	if err := localPublish(context.Background(), s, RuleKey{"local-test", 1}, n, ContextFontFamilyCategories, raw, value, Evidence[bool]{State: EvidenceKnown, Value: true}, NodeRef{}, ContextUnitNone); err != nil {
		release()
		t.Fatal(err)
	}
	combined, _ := s.account.Usage(BudgetBufferedBytes)
	release()
	retained, _ := s.account.Usage(BudgetBufferedBytes)
	if retained <= 0 || combined-retained != held {
		t.Fatalf("held %d combined %d retained %d", held, combined, retained)
	}
	value.Value.Text = "changed"
	if s.contextResults[0].Value.Value.Text != "music,text" {
		t.Fatal("projection is not owned")
	}
}
func TestLocalRound1FontAdmissionBoundaryAndCancellation(t *testing.T) {
	for _, limit := range []int64{265, 266} {
		s := rationalSession(t, Budgets{MaxBufferedBytes: limit})
		_, release, err := localFontCategoriesOwned(context.Background(), s, localKnownText("music, serif"))
		if limit == 265 && err == nil {
			t.Fatal("under boundary admitted")
		}
		if limit == 266 && err != nil {
			t.Fatal(err)
		}
		if release != nil {
			release()
		}
		used, _ := s.account.Usage(BudgetBufferedBytes)
		if used != 0 {
			t.Fatalf("leak %d", used)
		}
		s.cleanup()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	_, release, err := localFontCategoriesOwned(ctx, s, localKnownText("music, serif"))
	if err == nil || release != nil {
		t.Fatalf("cancelled construction %v", err)
	}
}

func TestLocalRound1PriorInterruptionNeverPublishes(t *testing.T) {
	for _, family := range []string{"structural", "advisory", "glyph", "printing"} {
		s, n, r := localEvidenceFixture(t, EvidenceKnown, localFrameNote)
		sentinel := context.Canceled
		r.err = sentinel
		var err error
		switch family {
		case "structural":
			err = localStructuralNode(r.ctx, s, RuleKey{"local-test", 1}, n, r)
		case "advisory":
			err = localAdvisoryNode(r.ctx, s, RuleKey{"local-test", 1}, n, r)
		case "glyph":
			err = localGlyphFacts(r, RuleKey{"local-test", 1}, n)
		case "printing":
			err = localPrintNode(r.ctx, s, RuleKey{"local-test", 1}, n, r)
		}
		if err != sentinel || len(s.contextResults) != 0 {
			t.Errorf("%s error %v rows %d", family, err, len(s.contextResults))
		}
		s.cleanup()
	}
}
func TestLocalRound1FontFailedPublicationReleasesTemporary(t *testing.T) {
	s := rationalSession(t, Budgets{MaxBufferedBytes: 266})
	defer s.cleanup()
	s.documentID = "doc"
	raw := localKnownText("music, serif")
	value, release, err := localFontCategoriesOwned(context.Background(), s, raw)
	if err != nil {
		t.Fatal(err)
	}
	n := localNode(1, 0, "font")
	err = localPublish(context.Background(), s, RuleKey{"local-test", 1}, n, ContextFontFamilyCategories, raw, value, Evidence[bool]{State: EvidenceKnown, Value: true}, NodeRef{}, ContextUnitNone)
	release()
	used, _ := s.account.Usage(BudgetBufferedBytes)
	if err == nil || used != 0 || len(s.contextResults) != 0 {
		t.Fatalf("err %v used %d rows %d", err, used, len(s.contextResults))
	}
}

func TestLocalRound1StaffLineContractsRemainDistinct(t *testing.T) {
	for _, attribute := range []bool{false, true} {
		s, n, r := localEvidenceFixture(t, EvidenceKnown, localLine)
		defer s.cleanup()
		n.end.Scalar.Value = testEvidence(Evidence[Atomic]{State: EvidenceUnknown, Reason: "must-not-read-attribute-owner-scalar"})
		want := "one-is-bottom-line-unbounded-by-line-count"
		if attribute {
			r.b.Roles = map[contextRole][]SchemaUse{}
			r.b.Attributes[localLineNumber] = map[ComponentID]AttributeSelector{n.header.Type.Value: localAttr("line")}
			localSetAttr(n, "line", Atomic{Kind: AtomicInteger, Integer: localOne})
			want = "one-is-bottom-line"
		} else {
			n.end.Scalar.Value = testEvidence(Evidence[Atomic]{State: EvidenceKnown, Value: Atomic{Kind: AtomicInteger, Integer: localOne}})
		}
		if err := localStructuralNode(r.ctx, s, RuleKey{"local-test", 1}, n, r); err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, row := range s.contextResults {
			if row.Property == ContextStaffLineBasis {
				count++
				if row.Value.State != EvidenceKnown || row.Value.Value.Text != want {
					t.Fatalf("attribute%t %+v", attribute, row)
				}
			}
		}
		if count != 1 {
			t.Fatalf("rows %d", count)
		}
	}
}
