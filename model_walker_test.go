package musicxml

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

type modelRecordingSink struct {
	headers         []validation.NodeHeader
	attributes      []validation.AttributeFact
	ends            map[validation.NodeID]validation.NodeEnd
	representations map[validation.NodeID]validation.ModelRepresentationFact
	account         *validation.Account
	cancel          context.CancelFunc
	cancelAt        int
}

func (s *modelRecordingSink) Open(_ context.Context, v validation.NodeHeader) error {
	if s.account != nil {
		if err := s.account.Charge(validation.BudgetDepth, 1); err != nil {
			return err
		}
		if err := s.account.Charge(validation.BudgetNodes, 1); err != nil {
			_ = s.account.Release(validation.BudgetDepth, 1)
			return err
		}
	}
	s.headers = append(s.headers, v)
	if len(s.headers) == s.cancelAt && s.cancel != nil {
		s.cancel()
	}
	return nil
}
func (s *modelRecordingSink) Attribute(_ context.Context, v validation.AttributeFact) error {
	s.attributes = append(s.attributes, v)
	return nil
}
func (s *modelRecordingSink) CompleteAttributes(context.Context, validation.NodeID, validation.Evidence[bool]) error {
	return nil
}
func (s *modelRecordingSink) ModelRepresentation(_ context.Context, v validation.ModelRepresentationFact) error {
	if s.representations == nil {
		s.representations = map[validation.NodeID]validation.ModelRepresentationFact{}
	}
	v.Problems = append([]validation.ModelProblem(nil), v.Problems...)
	s.representations[v.Owner] = v
	return nil
}
func (s *modelRecordingSink) Close(_ context.Context, v validation.NodeEnd) error {
	if s.ends == nil {
		s.ends = map[validation.NodeID]validation.NodeEnd{}
	}
	s.ends[v.ID] = v
	if s.account != nil {
		return s.account.Release(validation.BudgetDepth, 1)
	}
	return nil
}
func (s *modelRecordingSink) named(name string) []validation.NodeHeader {
	var out []validation.NodeHeader
	for _, h := range s.headers {
		if h.Name.Local == name {
			out = append(out, h)
		}
	}
	return out
}
func recordModel(t *testing.T, doc Document) (*modelRecordingSink, modelWalkStatus) {
	t.Helper()
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	s := &modelRecordingSink{account: a}
	status, err := walkModel(context.Background(), doc, s, a, "current", modelWalkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	assertWalkerReleased(t, a)
	return s, status
}
func assertWalkerReleased(t *testing.T, a *validation.Account) {
	t.Helper()
	n, err := a.Usage(validation.BudgetBufferedBytes)
	if err != nil || n != 0 {
		t.Fatalf("walker leaked %d bytes: %v", n, err)
	}
}

type modelUnsupportedDocument struct{}

func (*modelUnsupportedDocument) isDocument() {}
func TestDirectModelWalkerRootsNilAndTransportName(t *testing.T) {
	for _, doc := range []Document{(*ScorePartwise)(nil), (*ScoreTimewise)(nil), (*OpusDocument)(nil), nil} {
		a, _ := validation.NewBudgetAccount(validation.Budgets{})
		_, err := walkModel(context.Background(), doc, &modelRecordingSink{}, a, "nil", modelWalkOptions{})
		if !errors.Is(err, ErrNilDocument) {
			t.Errorf("got %v", err)
		}
	}
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	_, err := walkModel(context.Background(), &modelUnsupportedDocument{}, &modelRecordingSink{}, a, "unsupported", modelWalkOptions{})
	if !errors.Is(err, ErrUnsupportedDocument) {
		t.Fatal(err)
	}
	bad := xml.Name{Space: "\xff", Local: "\x00"}
	for _, doc := range []Document{&ScorePartwise{XMLName: bad}, &ScoreTimewise{XMLName: bad}, &OpusDocument{XMLName: bad}} {
		s, status := recordModel(t, doc)
		if status.HasProblems {
			t.Error("transport XMLName became a defect")
		}
		if s.headers[0].Name.Local != documentRootName(doc).Local {
			t.Error("wrong fixed root")
		}
		for _, h := range s.headers {
			if h.Location.Position != nil || h.Location.Target != validation.TargetModel {
				t.Error("fabricated source location")
			}
		}
	}
}
func TestDirectModelWalkerPresenceOrderAndExactValues(t *testing.T) {
	score := NewScorePartwise(PartDefinition{ID: "P1", Name: "Piano"})
	measure := NewScorePartwiseMeasure("1")
	note := NewRestNote(1)
	note.Rest = nil
	note.Pitch = &Pitch{Step: StepC, Octave: 0}
	zero := Semitones(0)
	note.Pitch.Alter = &zero
	measure.AddNote(note)
	measure.AddBackup(&Backup{Duration: 1})
	measure.AddNote(NewRestNote(1))
	score.Part[0].AddMeasure(measure)
	s, status := recordModel(t, score)
	if status.HasProblems {
		t.Fatal("unexpected representation defect")
	}
	octave := s.named("octave")
	if len(octave) != 1 {
		t.Fatal("mandatory zero omitted")
	}
	value := s.ends[octave[0].ID].Scalar
	if value.Value.State != validation.EvidenceKnown || value.Value.Value.Integer.String() != "0" || value.Lexeme.State != validation.EvidenceUnsupported {
		t.Fatal(value)
	}
	if len(s.named("alter")) != 1 {
		t.Fatal("present optional zero omitted")
	}
	note.Pitch.Alter = nil
	s, _ = recordModel(t, score)
	if len(s.named("alter")) != 0 {
		t.Fatal("nil optional represented")
	}
	var names []string
	measureID := s.named("measure")[0].ID
	for _, h := range s.headers {
		if h.Parent == measureID {
			names = append(names, h.Name.Local)
		}
	}
	if strings.Join(names, ",") != "note,backup,note" {
		t.Fatal(names)
	}
	if s.ends[measureID].Order.Value != validation.OrderModelContent {
		t.Error("wrong Content order")
	}
	pitchID := s.named("pitch")[0].ID
	if s.ends[pitchID].Scalar.Value.State != validation.EvidenceAbsent {
		t.Error("element-only scalar not absent")
	}
	if !strings.Contains(s.named("step")[0].Location.Path, "Part[0].Measure[0].Content[0].Note.Pitch.Step") {
		t.Fatal(s.named("step")[0].Location.Path)
	}
}
func TestDirectModelWalkerHolesCyclesAndSharedPointers(t *testing.T) {
	shared := &OpusDocument{}
	root := &OpusDocument{Content: OpusDocumentContents{{Opus: shared}, {}, {Opus: shared}, {Opus: shared, Score: &OpusScore{}}, {Score: &OpusScore{Href: "later"}}}}
	s, status := recordModel(t, root)
	if !status.HasProblems || status.HasCycle {
		t.Fatal(status)
	}
	if len(s.named("opus")) != 3 || len(s.named("score")) != 1 {
		t.Fatal("invalid wrapper fabricated children or shared occurrence lost")
	}
	if s.ends[1].Order.State != validation.EvidenceUnsupported || s.ends[1].ChildrenComplete.State != validation.EvidenceUnsupported {
		t.Error("middle hole compacted")
	}
	if len(s.representations[1].Problems) != 1 {
		t.Fatal("choice defect summaries not bounded")
	}
	root.Content = []OpusDocumentContent{{Opus: root}, {Score: &OpusScore{Href: "later"}}}
	s, status = recordModel(t, root)
	if !status.HasCycle || !status.HasProblems || len(s.named("score")) != 1 {
		t.Fatal("cycle did not preserve later sibling", status)
	}
	a, b := &OpusDocument{}, &OpusDocument{}
	a.Content = []OpusDocumentContent{{Opus: b}}
	b.Content = []OpusDocumentContent{{Opus: a}, {Score: &OpusScore{}}}
	s, status = recordModel(t, a)
	if !status.HasCycle || len(s.named("opus")) != 2 {
		t.Fatal("two-node cycle", status)
	}
}
func TestDirectModelWalkerScopedInvalidValues(t *testing.T) {
	for _, bad := range []string{"\xff", "\x01"} {
		root := &OpusDocument{Title: &bad, Content: OpusDocumentContents{{Score: &OpusScore{Href: bad}}, {Score: &OpusScore{Href: "later"}}}}
		s, status := recordModel(t, root)
		if !status.HasProblems || len(s.named("score")) != 2 {
			t.Fatal(status)
		}
		title := s.named("title")[0]
		if s.ends[title.ID].Scalar.Value.State != validation.EvidenceUnsupported {
			t.Error("invalid string claimed known")
		}
		if s.ends[1].ChildrenComplete.State != validation.EvidenceKnown {
			t.Error("scalar defect invalidated siblings")
		}
	}
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		number := Tenths(bad)
		duration := PositiveDivisions(bad)
		score := &ScorePartwise{Part: []ScorePartwisePart{{Measure: []ScorePartwisePartMeasure{{Number: "1", Width: &number, Content: ScorePartwisePartMeasureContents{{Note: &Note{Duration: &duration}}}}}}}}
		s, status := recordModel(t, score)
		if !status.HasProblems {
			t.Fatal("nonfinite escaped")
		}
		if s.ends[s.named("duration")[0].ID].Scalar.Value.State != validation.EvidenceUnsupported {
			t.Error("nonfinite element known")
		}
		found := false
		for _, a := range s.attributes {
			if a.Name.Local == "width" {
				found = true
				if a.Scalar.Value.State != validation.EvidenceUnsupported {
					t.Error("nonfinite attribute known")
				}
			}
		}
		if !found {
			t.Fatal("missing width")
		}
	}
}
func TestDirectModelWalkerBudgetsCancellationAndDepth(t *testing.T) {
	chain := func(depth int) *OpusDocument {
		root := &OpusDocument{}
		next := root
		for i := 1; i < depth; i++ {
			child := &OpusDocument{}
			next.Content = []OpusDocumentContent{{Opus: child}}
			next = child
		}
		return root
	}
	for _, depth := range []int{8, 9, 4096, 4097} {
		limit := int64(8)
		if depth > 9 {
			limit = 4096
		}
		a, _ := validation.NewBudgetAccount(validation.Budgets{MaxDepth: limit})
		s := &modelRecordingSink{account: a}
		status, err := walkModel(context.Background(), chain(depth), s, a, "depth", modelWalkOptions{})
		if int64(depth) > limit {
			var resource *validation.ResourceError
			if !errors.As(err, &resource) || resource.Kind != validation.BudgetDepth || !status.DepthExceeded {
				t.Fatalf("%d: %v %+v", depth, err, status)
			}
		} else if err != nil {
			t.Fatalf("%d: %v", depth, err)
		}
		assertWalkerReleased(t, a)
	}
	for _, budget := range []validation.Budgets{{MaxEvents: 10}, {MaxBufferedBytes: 128}, {MaxScalarBytes: 1}, {MaxNodes: 2}} {
		a, _ := validation.NewBudgetAccount(budget)
		s := &modelRecordingSink{account: a}
		title := "long"
		_, err := walkModel(context.Background(), &OpusDocument{Title: &title, Content: OpusDocumentContents{{Score: &OpusScore{}}}}, s, a, "bounded", modelWalkOptions{})
		if !errors.Is(err, validation.ErrResourceLimit) {
			t.Fatalf("%+v: %v", budget, err)
		}
		assertWalkerReleased(t, a)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	s := &modelRecordingSink{account: a, cancel: cancel, cancelAt: 4}
	_, err := walkModel(ctx, chain(9), s, a, "cancel", modelWalkOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	assertWalkerReleased(t, a)
}
func TestDirectModelWalkerLegacyPathsAndMetadataMismatch(t *testing.T) {
	kind := "simple"
	root := &OpusDocument{Content: OpusDocumentContents{{Score: &OpusScore{Type: &kind}}, {Score: &OpusScore{}}}}
	a, _ := validation.NewBudgetAccount(validation.Budgets{})
	s := &modelRecordingSink{}
	_, err := walkModel(context.Background(), root, s, a, "legacy", modelWalkOptions{LegacyPaths: true})
	if err != nil {
		t.Fatal(err)
	}
	if s.named("score")[1].Location.Path != "/opus/score[2]" {
		t.Fatal(s.named("score"))
	}
	found := false
	for _, attribute := range s.attributes {
		if attribute.Name.Local == "type" {
			found = true
			if attribute.Location.Path != "/opus/score/@{http://www.w3.org/1999/xlink}type" {
				t.Error(attribute.Location.Path)
			}
		}
	}
	if !found {
		t.Fatal("missing namespaced attribute")
	}
	w := modelWalker{ctx: context.Background(), account: a}
	_, err = w.field(reflect.ValueOf(Pitch{}), modelField{Name: "Wrong", Index: []int{0}})
	if !errors.Is(err, validation.ErrIncompleteAssessment) {
		t.Fatal(err)
	}
}

func TestDirectModelWalkerCorpusIsReadOnly(t *testing.T) {
	entries, err := os.ReadDir(musicXMLTestSuiteDirectory)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		// This upstream malformed numeric fixture cannot produce a current model.
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".xml") || entry.Name() == "99d-AccordionInvalid.xml" {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(musicXMLTestSuiteDirectory, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			doc, err := Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			before, err := Decode(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			recordModel(t, doc)
			if !reflect.DeepEqual(doc, before) {
				t.Fatal("walk mutated input")
			}
		})
	}
}
func TestDirectModelWalkerNoTransportOrDynamicCallbacks(t *testing.T) {
	banned := map[string]bool{"Encode": true, "Decode": true, "Marshal": true, "Unmarshal": true, "MarshalXML": true, "UnmarshalXML": true, "MarshalText": true, "UnmarshalText": true, "Method": true, "MethodByName": true, "Call": true, "CallSlice": true, "Tag": true, "checkDocumentText": true, "checkDocumentNesting": true, "parseValidationDocument": true}
	for _, path := range []string{"model_walker.go", "model_scalar.go", "model_metadata.go"} {
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, i := range f.Imports {
			if i.Path.Value == `"encoding/xml"` {
				t.Fatal("XML transport import in direct walker")
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				var name string
				switch f := call.Fun.(type) {
				case *ast.Ident:
					name = f.Name
				case *ast.SelectorExpr:
					name = f.Sel.Name
				}
				if banned[name] {
					t.Errorf("%s calls forbidden %s", path, name)
				}
			}
			return true
		})
	}
}
func TestDirectModelWalkerWideHolesRemainBounded(t *testing.T) {
	root := &OpusDocument{Content: make(OpusDocumentContents, 10000)}
	a, _ := validation.NewBudgetAccount(validation.Budgets{MaxBufferedBytes: 16000})
	s := &modelRecordingSink{}
	status, err := walkModel(context.Background(), root, s, a, "wide", modelWalkOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !status.HasProblems || len(s.representations[1].Problems) != 1 {
		t.Fatal("unbounded repeated problem storage")
	}
	assertWalkerReleased(t, a)
}
func TestDirectModelScalarLimitsAndRepresentationProblems(t *testing.T) {
	for _, value := range []string{"\xff", "\x00", "\x01"} {
		a, _ := validation.NewBudgetAccount(validation.Budgets{})
		result, err := readModelScalar(context.Background(), a, reflect.ValueOf(value), modelString)
		if err != nil || result.problem == "" || result.scalar.Value.State != validation.EvidenceUnsupported {
			t.Fatalf("%q: %+v %v", value, result, err)
		}
	}
	a, _ := validation.NewBudgetAccount(validation.Budgets{MaxScalarBytes: 1})
	result, err := readModelScalar(context.Background(), a, reflect.ValueOf(float64(0)), modelDecimal)
	if err != nil {
		t.Fatal("zero scalar should fit one byte", err)
	}
	_ = a.Release(validation.BudgetBufferedBytes, result.retained)
	result, err = readModelScalar(context.Background(), a, reflect.ValueOf(math.SmallestNonzeroFloat64), modelDecimal)
	if !errors.Is(err, validation.ErrResourceLimit) {
		t.Fatal(err)
	}
	assertWalkerReleased(t, a)
	a, _ = validation.NewBudgetAccount(validation.Budgets{MaxBufferedBytes: 100})
	_, err = readModelScalar(context.Background(), a, reflect.ValueOf(float64(.1)), modelDecimal)
	if !errors.Is(err, validation.ErrResourceLimit) {
		t.Fatal(err)
	}
	assertWalkerReleased(t, a)
}

func TestDirectModelWalkerRejectsIncompleteDescriptor(t *testing.T) {
	descriptor := *modelDescriptors["Pitch"]
	descriptor.Elements = descriptor.Elements[:1]
	account, _ := validation.NewBudgetAccount(validation.Budgets{})
	walker := modelWalker{ctx: context.Background(), sink: &modelRecordingSink{}, account: account, documentID: "broken-map"}
	err := walker.push(reflect.ValueOf(Pitch{}), &descriptor, 0, modelName{Local: "pitch"}, "Pitch", "Pitch", nil)
	if !errors.Is(err, validation.ErrIncompleteAssessment) {
		t.Fatalf("incomplete descriptor accepted: %v", err)
	}
	_ = account.Release(validation.BudgetBufferedBytes, walker.retained)
}

func TestDirectModelWalkerParallelReadOnly(t *testing.T) {
	shared := &OpusDocument{}
	root := &OpusDocument{Content: OpusDocumentContents{{Opus: shared}, {Opus: shared}}}
	for i := 0; i < 16; i++ {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			t.Parallel()
			sink, status := recordModel(t, root)
			if status.HasProblems || len(sink.named("opus")) != 3 {
				t.Fatal("shared current graph changed")
			}
		})
	}
}
func TestDirectModelWalkerScalarLeafDepthBoundary(t *testing.T) {
	for _, depth := range []int{15, 16} {
		root := &OpusDocument{}
		next := root
		for i := 1; i < depth; i++ {
			child := &OpusDocument{}
			next.Content = OpusDocumentContents{{Opus: child}}
			next = child
		}
		title := "leaf"
		next.Title = &title
		account, _ := validation.NewBudgetAccount(validation.Budgets{MaxDepth: 16})
		sink := &modelRecordingSink{account: account}
		status, err := walkModel(context.Background(), root, sink, account, "leaf-depth", modelWalkOptions{})
		if depth == 15 && err != nil {
			t.Fatal(err)
		}
		if depth == 16 && (!status.DepthExceeded || !errors.Is(err, validation.ErrResourceLimit)) {
			t.Fatalf("scalar leaf did not count as node depth: %+v %v", status, err)
		}
		assertWalkerReleased(t, account)
	}
}

func TestDirectModelWalkerLegacyRepresentationKeepsGoPath(t *testing.T) {
	bad := "a\x00b"
	for _, tt := range []struct {
		name               string
		doc                Document
		goPath, legacyPath string
	}{
		{"element", &ScorePartwise{MovementTitle: &bad}, "ScorePartwise.MovementTitle", "/score-partwise/movement-title"},
		{"attribute", &ScoreTimewise{Version: &bad}, "ScoreTimewise.Version", "/score-timewise/@version"},
		{"choice", &OpusDocument{Content: OpusDocumentContents{{Score: &OpusScore{Href: bad}}}}, "OpusDocument.Content[0].Score.Href", "/opus/score/@{http://www.w3.org/1999/xlink}href"},
		{"named string", &ScorePartwise{Part: []ScorePartwisePart{{Measure: []ScorePartwisePartMeasure{{Text: Ptr(MeasureText(bad))}}}}}, "ScorePartwise.Part[0].Measure[0].Text", "/score-partwise/part/measure/@text"},
		{"simple content", &ScorePartwise{Credit: []Credit{{Content: CreditContents{{CreditWords: &FormattedTextID{Value: bad}}}}}}, "ScorePartwise.Credit[0].Content[0].CreditWords.Value", "/score-partwise/credit/credit-words"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a, _ := validation.NewBudgetAccount(validation.Budgets{})
			sink := &modelRecordingSink{}
			status, err := walkModel(context.Background(), tt.doc, sink, a, "legacy-representation", modelWalkOptions{LegacyPaths: true})
			if err != nil {
				t.Fatal(err)
			}
			if !status.HasProblems {
				t.Fatal("representation defect lost")
			}
			found := false
			for _, fact := range sink.representations {
				for _, problem := range fact.Problems {
					if problem.Code == "model-invalid-xml-character" {
						found = true
						if !strings.Contains(problem.Actual, tt.goPath) {
							t.Errorf("actual %q lacks Go field path %q", problem.Actual, tt.goPath)
						}
						if problem.Location.Path != tt.legacyPath {
							t.Errorf("legacy path changed: %s", problem.Location.Path)
						}
					}
				}
			}
			if !found {
				t.Fatal("problem missing")
			}
			assertWalkerReleased(t, a)
		})
	}
}

func TestDirectModelWalkerLegacyGoPathBoundsAndCleanup(t *testing.T) {
	root := &OpusDocument{}
	next := root
	for i := 1; i < 1100; i++ {
		child := &OpusDocument{}
		next.Content = OpusDocumentContents{{Opus: child}}
		next = child
	}
	bad := "\x00"
	next.Title = &bad
	account, _ := validation.NewBudgetAccount(validation.Budgets{MaxDepth: 2000})
	sink := &modelRecordingSink{account: account}
	status, err := walkModel(context.Background(), root, sink, account, "deep-legacy", modelWalkOptions{LegacyPaths: true})
	if err != nil {
		t.Fatal(err)
	}
	if !status.HasProblems {
		t.Fatal("missing deep defect")
	}
	for _, fact := range sink.representations {
		for _, problem := range fact.Problems {
			if len(problem.Actual) > 4096 || !strings.Contains(problem.Actual, "node[") || !strings.HasSuffix(problem.Actual, ".Title") {
				t.Fatalf("unbounded or untruthful current path: %q", problem.Actual)
			}
		}
	}
	assertWalkerReleased(t, account)
	for limit := int64(512); limit <= 16000; limit += 512 {
		account, _ := validation.NewBudgetAccount(validation.Budgets{MaxBufferedBytes: limit})
		sink := &modelRecordingSink{}
		_, err := walkModel(context.Background(), &OpusDocument{Content: OpusDocumentContents{{Score: &OpusScore{Href: bad}}}}, sink, account, "limited-legacy", modelWalkOptions{LegacyPaths: true})
		if err != nil && !errors.Is(err, validation.ErrResourceLimit) {
			t.Fatal(err)
		}
		assertWalkerReleased(t, account)
	}
}
