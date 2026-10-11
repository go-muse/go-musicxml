package xsd

import (
	"context"
	"errors"
	"math"
	"unsafe"
)

// SubjectID identifies a source occurrence; zero denotes no containing owner.
type SubjectID uint64

// Subject contains XML-neutral borrowed facts. The caller keeps their storage
// unchanged until Close. Incomplete axes never imply an empty value or absence.
type Subject struct {
	ID, Parent                           SubjectID
	Name                                 QName
	Path                                 string
	Namespaces                           map[string]string
	Attributes                           []Attribute
	Text                                 string
	TextKnown                            bool
	Children                             []Child
	AttributesComplete, ChildrenComplete bool
}
type Child struct {
	ID   SubjectID
	Name QName
	Path string
}

// Detail preserves the original diagnostic plus exact source-slot identity.
type Detail struct {
	Issue
	Subject         SubjectID
	Attribute       QName
	RelatedSubjects []SubjectID
}
type Limitation struct {
	Reason string
	Cause  error
}
type Result struct {
	Applicable, Valid, Complete bool
	Issues                      []Detail
	Limitations                 []Limitation
}
type NodeResults struct{ Binding, Declaration, Attributes, Scalar, Content Result }
type ScalarResult struct {
	Known, Complete, Defaulted       bool
	Lexeme, EffectiveLexeme, Builtin string
	Value                            SimpleValue
}
type AttributeResult struct {
	Name                                           QName
	Use                                            Use
	Present, TypeValid, DeclarationValid, Complete bool
	Scalar                                         ScalarResult
}
type WitnessMode string

const (
	WitnessDeclared WitnessMode = "declared"
	WitnessSkip     WitnessMode = "skip"
	WitnessLax      WitnessMode = "unsupported-lax"
	WitnessStrict   WitnessMode = "unsupported-strict"
)

type Witness struct {
	Index                    int
	Child                    SubjectID
	Use, ResolvedDeclaration Use
	Mode                     WitnessMode
}
type Binding struct {
	run   *Run
	state *boundState
}
type Completion struct {
	TypeValid, DeclarationValid, Complete, IdentityClosed bool
	run                                                   *Run
	subject                                               SubjectID
}

// IdentityResults is cumulative within a run. Issues retains emission order:
// later calls append newly proved findings without moving the preview prefix.
// Valid becomes false before any allocation needed to publish its detail.
type IdentityResults struct {
	BySubject       map[SubjectID]Result
	Issues          []Detail
	Valid, Complete bool
}

// Meter is a fixed execution-control interface, never a predicate callback.
type Meter interface {
	Step(context.Context, int64) error
	Reserve(int64) error
	Release(int64) error
	Scalar(int64) error
	Identity(int64) error
}

type boundState struct {
	attributeEpoch                               uint64
	attributeKnown                               bool
	defaulted                                    bool
	run                                          *Run
	subject                                      Subject
	node                                         *Node
	use                                          Use
	actual, declared                             *TypeRef
	results                                      NodeResults
	scalar                                       ScalarResult
	attributes                                   []AttributeResult
	matches                                      []ElementMatch
	witnesses                                    []Witness
	typeValid, declarationValid, identityAllowed bool
	effectiveText                                string
	family                                       string
	attribute                                    QName
	issueSubject                                 SubjectID
	scalarAssessment                             SimpleAssessment
	scalarAssessed                               bool
	done                                         bool
	completion                                   Completion
}

// Run contains only run-local state. Compiled schema handles remain immutable.
type Run struct {
	schema            *Compiled
	meter             Meter
	context           Context
	nodes             map[SubjectID]*Node
	states            map[SubjectID]*boundState
	retained          int64
	scratchRetained   int64
	closed, finalized bool
	current           context.Context
	identityResults   map[SubjectID]Result
	identityIssues    []Detail
	identitySeen      map[identityDiagnostic]bool
	identityValid     bool
	identityOutput    *IdentityResults
}

func NewRun(schema *Compiled, meter Meter) (out *Run, err error) {
	if schema == nil || schema.digest == "" || meter == nil {
		return nil, errors.New("xsd: compiled schema and meter required")
	}
	r := &Run{schema: schema, meter: meter, identityValid: true}
	defer func() {
		if err != nil {
			_ = r.Close()
		}
	}()
	defer recoverRun(&err)
	// Reserve all initial map and context allowances before allocating them.
	r.reserve(1024)
	r.nodes = make(map[SubjectID]*Node)
	r.states = make(map[SubjectID]*boundState)
	r.context = Context{Schema: &schema.data, Effective: make(map[*ComplexSchema]*EffectiveComplex), Identifiers: make(map[string]string), run: r}
	return r, nil
}

func (r *Run) Root(name QName) (Use, bool) {
	if r == nil {
		return Use{}, false
	}
	return r.schema.Root(name)
}
func (r *Run) check(ctx context.Context) error {
	if r == nil || r.closed || r.finalized {
		return errors.New("xsd: run is closed or finalized")
	}
	if ctx == nil {
		return errors.New("xsd: nil context")
	}
	return ctx.Err()
}
func (r *Run) reserve(n int64) {
	if n < 0 || n > math.MaxInt64-r.retained-r.scratchRetained {
		panic(runAbort{errors.New("xsd: negative retention")})
	}
	if err := r.meter.Reserve(n); err != nil {
		panic(runAbort{err})
	}
	r.retained += n
}
func (r *Run) step(n int64) {
	if err := r.current.Err(); err != nil {
		panic(runAbort{err})
	}
	if err := r.meter.Step(r.current, n); err != nil {
		panic(runAbort{err})
	}
}
func (r *Run) scalar(n int64) {
	if err := r.meter.Scalar(n); err != nil {
		panic(runAbort{err})
	}
}

type runAbort struct{ err error }

func recoverRun(err *error) {
	if p := recover(); p != nil {
		if stop, ok := p.(runAbort); ok {
			*err = stop.err
		} else {
			panic(p)
		}
	}
}
func freshResult() Result { return Result{Applicable: true, Valid: true, Complete: true} }
func freshResults() NodeResults {
	return NodeResults{freshResult(), freshResult(), freshResult(), freshResult(), freshResult()}
}
func (s *boundState) result() *Result {
	switch s.family {
	case "binding":
		return &s.results.Binding
	case "attributes":
		return &s.results.Attributes
	case "scalar":
		return &s.results.Scalar
	case "content":
		return &s.results.Content
	default:
		return &s.results.Declaration
	}
}
func (s *boundState) limit(family, reason string, cause error) {
	old := s.family
	s.family = family
	v := s.result()
	v.Complete = false

	v.Limitations = appendRetained(&s.run.context, v.Limitations, Limitation{reason, cause})
	s.family = old
}
func (s *boundState) detail(path, constraint, message string) {
	v := s.result()
	v.Valid = false
	s.run.reserve(int64(len(path)))
	s.run.reserve(int64(len(message)))
	d := Detail{Issue: Issue{Path: path, Constraint: constraint, Message: message}, Subject: s.subject.ID, Attribute: s.attribute}
	if d.Attribute == (QName{}) {
		for _, a := range s.subject.Attributes {
			s.run.step(1)
			if compareText(path, attributePath(s.subject.Path, a.Name, &s.run.context), &s.run.context) == 0 {
				d.Attribute = a.Name
				break
			}
		}
	}
	if s.issueSubject != 0 {
		d.Subject = s.issueSubject
	}
	v.Issues = appendRetained(&s.run.context, v.Issues, d)
}

// Bind assesses only the subject's local XSD contracts and exact child grammar.
// It never recursively assesses descendants. Even a stopped operation returns
// its partial binding when available, preserving previously proved local issues.
func (r *Run) Bind(ctx context.Context, use Use, subject Subject) (bound Binding, err error) {
	if err = r.check(ctx); err != nil {
		return
	}
	if use.schema != r.schema || use.original == nil || subject.ID == 0 {
		return Binding{}, errors.New("xsd: invalid schema use or subject")
	}
	if r.states[subject.ID] != nil {
		return Binding{}, errors.New("xsd: duplicate bound subject")
	}
	r.current = ctx
	defer func() {
		if err != nil {
			r.context.IdentityIncomplete = true
			if r.owns(bound) {
				bound.state.abort()
			}
		}
	}()
	defer recoverRun(&err)
	defer r.releaseScratch()
	r.step(1)
	r.scalar(int64(len(subject.Text)))
	r.reserve(int64(unsafe.Sizeof(boundState{})) + int64(unsafe.Sizeof(Node{})) + 192)
	r.reserve(int64(len(subject.Text)))
	r.reserve(r.context.size(int64(len(subject.Children)), int64(unsafe.Sizeof((*Node)(nil)))))
	node := r.nodes[subject.ID]
	if node == nil {
		node = &Node{}
		r.nodes[subject.ID] = node
	}
	node.ID = subject.ID
	node.Name = subject.Name
	node.Namespaces = subject.Namespaces
	node.Attrs = subject.Attributes
	r.context.bytes(len(subject.Text))
	node.Text.WriteString(subject.Text)
	node.Children = make([]*Node, len(subject.Children))
	for i, ch := range subject.Children {
		r.step(1)
		if ch.ID == 0 || ch.ID == subject.ID {
			return Binding{}, errors.New("xsd: invalid child subject")
		}
		n := r.nodes[ch.ID]
		if n == nil {
			r.reserve(256)
			n = &Node{ID: ch.ID, Name: ch.Name}
			r.nodes[ch.ID] = n
		} else if n.Name != ch.Name {
			return Binding{}, errors.New("xsd: conflicting child name")
		}
		node.Children[i] = n
	}
	s := &boundState{run: r, subject: subject, node: node, use: use, results: freshResults(), family: "binding"}
	r.states[subject.ID] = s
	bound = Binding{r, s}
	r.context.local = s
	defer func() { r.context.local = nil }()
	for _, a := range subject.Attributes {
		r.step(1)
		r.scalar(int64(len(a.Value)))
	}
	if !subject.AttributesComplete {
		s.limit("attributes", "attribute-enumeration-incomplete", nil)
		r.context.IdentityIncomplete = true
	}
	if !subject.ChildrenComplete {
		s.limit("content", "child-enumeration-incomplete", nil)
		r.context.IdentityIncomplete = true
	}
	if !subject.TextKnown {
		s.limit("scalar", "scalar-lexeme-unavailable", nil)
		s.limit("declaration", "scalar-lexeme-unavailable", nil)
		s.limit("content", "scalar-lexeme-unavailable", nil)
		r.context.IdentityIncomplete = true
	}
	for _, a := range subject.Attributes {
		r.step(1)
		if !a.NamespaceDeclaration && a.Name.Space == XSINamespace && !a.ValueKnown && (a.Name.Local == "type" || a.Name.Local == "nil") {
			for _, family := range []string{"binding", "declaration", "attributes", "scalar", "content"} {
				s.limit(family, "schema-instance-value-unavailable", nil)
			}
			r.context.IdentityIncomplete = true
			return bound, nil
		}
	}
	r.context.ValidateElementOwned(node, use.original, subject.Path, r.nodes[subject.Parent])
	if _, ok := r.typeID(s.actual); !ok {
		s.limit("binding", "actual-type-binding-unavailable", nil)
	}
	r.buildWitnesses(s)
	return bound, nil
}
func (r *Run) ActualType(b Binding) (TypeID, bool) {
	if !r.owns(b) || b.state.actual == nil {
		return TypeID{}, false
	}
	return r.typeID(b.state.actual)
}
func (r *Run) typeID(ref *TypeRef) (TypeID, bool) {
	if ref == nil {
		return TypeID{}, false
	}
	if ref.InlineSimple != nil {
		v, ok := r.schema.simpleTypes[ref.InlineSimple]
		return v, ok
	}
	if ref.InlineComplex != nil {
		v, ok := r.schema.complexTypes[ref.InlineComplex]
		return v, ok
	}
	if ref.Name.Space == XSDNamespace {
		v, ok := r.schema.builtinTypes[ref.Name.Local]
		return v, ok
	}
	v, ok := r.schema.namedTypes[ref.Name]
	return v, ok
}
func (r *Run) owns(b Binding) bool {
	return r != nil && b.run == r && b.state != nil && r.states[b.state.subject.ID] == b.state
}
func (r *Run) Results(b Binding) NodeResults {
	if !r.owns(b) {
		return NodeResults{}
	}
	return b.state.results
}
func (r *Run) Witnesses(b Binding) []Witness {
	if !r.owns(b) {
		return nil
	}
	return b.state.witnesses
}
func (r *Run) Scalar(b Binding) ScalarResult {
	if !r.owns(b) {
		return ScalarResult{}
	}
	return b.state.scalar
}
func (r *Run) Attributes(b Binding) []AttributeResult {
	if !r.owns(b) {
		return nil
	}
	return b.state.attributes
}
func (r *Run) buildWitnesses(s *boundState) {
	owner, _ := r.typeID(s.actual)
	for _, m := range s.matches {
		r.step(1)
		if m.Particle != nil && m.Particle.Kind == ParticleAny {
			mode := WitnessStrict
			if m.Particle.Any.ProcessContents == "skip" {
				mode = WitnessSkip
			} else if m.Particle.Any.ProcessContents == "lax" {
				mode = WitnessLax
			}
			r.context.wildcard(m.Particle.Any.ProcessContents, "content")
			s.witnesses = appendRetained(&r.context, s.witnesses, Witness{Index: m.Index, Child: s.subject.Children[m.Index].ID, Mode: mode})
			continue
		}
		uses := r.schema.particleUses[particleUseKey{owner, m.Particle}]
		if len(uses) == 0 {
			for key, candidates := range r.schema.particleUses {
				r.step(1)
				if key.particle == m.Particle {
					r.context.scratch(r.context.size(int64(len(candidates)), 256))
					uses = append(uses, candidates...)
				}
			}
		}
		if len(uses) != 1 {
			s.limit("content", "exact-declaration-use-unavailable", nil)
			r.context.IdentityIncomplete = true
			continue
		}
		resolved := uses[0]
		if resolved.original.Reference.Local != "" {
			if target, ok := r.schema.Root(resolved.original.Reference); ok {
				resolved = target
			}
		}
		s.witnesses = appendRetained(&r.context, s.witnesses, Witness{Index: m.Index, Child: s.subject.Children[m.Index].ID, Use: uses[0], ResolvedDeclaration: resolved, Mode: WitnessDeclared})
	}
}
func (r *Run) Complete(ctx context.Context, b Binding, children []Completion) (out Completion, err error) {
	if err = r.check(ctx); err != nil {
		return
	}
	if !r.owns(b) || b.state.done {
		return out, errors.New("xsd: invalid or repeated node completion")
	}
	r.current = ctx
	defer func() {
		if err != nil {
			r.context.IdentityIncomplete = true
			b.state.abort()
			b.state.done = true
		}
	}()
	defer recoverRun(&err)
	defer r.releaseScratch()
	s := b.state
	r.context.local = s
	defer func() { r.context.local = nil }()
	var declared []Witness
	for _, w := range s.witnesses {
		r.step(1)
		if w.Mode == WitnessDeclared {
			declared = appendScratch(&r.context, declared, w)
		}
	}
	if len(children) != len(declared) {
		return out, errors.New("xsd: child completion count mismatch")
	}
	typeValid := s.typeValid
	complete := s.results.Binding.Complete && s.results.Declaration.Complete && s.results.Attributes.Complete && s.results.Scalar.Complete && s.results.Content.Complete
	for i, ch := range children {
		r.step(1)
		if ch.run != r || ch.subject != declared[i].Child {
			return out, errors.New("xsd: mismatched child completion")
		}
		typeValid = typeValid && ch.DeclarationValid
		complete = complete && ch.Complete
	}
	declarationValid := s.declarationValid && typeValid
	if typeValid && s.identityAllowed {
		s.family = "declaration"
		r.context.RecordElementIdentity(s.declared, s.actual, s.effectiveText, s.subject.Path, r.nodes[s.subject.Parent], declarationValid)
	}
	s.done = true
	s.completion = Completion{TypeValid: typeValid, DeclarationValid: declarationValid, Complete: complete, IdentityClosed: complete && !r.context.IdentityIncomplete, run: r, subject: s.subject.ID}
	return s.completion, nil
}

// Identity reports currently proved identity facts; false completeness suppresses
// absence. It does not seal the run, so root-closed duplicates survive tail errors.
func (r *Run) Identity(ctx context.Context, inputComplete bool) (out IdentityResults, err error) {
	out.Valid = true
	if r != nil {
		out.Valid = r.identityValid
		out.BySubject = r.identityResults
		out.Issues = r.identityIssues
	}
	if err = r.check(ctx); err != nil {
		return
	}
	r.current = ctx
	defer func() {
		if err != nil {
			out.Complete = false
		}
	}()
	defer recoverRun(&err)
	defer r.releaseScratch()
	if r.identityResults == nil {
		r.reserve(256)
		r.identityResults = make(map[SubjectID]Result)
		r.identitySeen = make(map[identityDiagnostic]bool)
	}
	out = IdentityResults{BySubject: r.identityResults, Issues: r.identityIssues, Valid: r.identityValid, Complete: inputComplete && !r.context.IdentityIncomplete && r.context.AssessmentErr == nil}
	r.identityOutput = &out
	defer func() { r.identityOutput = nil }()
	old := r.context.IdentityIncomplete
	r.context.IdentityIncomplete = old || !inputComplete
	defer func() { r.context.IdentityIncomplete = old }()
	r.context.ValidateIdentityReferences()
	return out, nil
}
func (r *Run) IdentityPreview(ctx context.Context) (IdentityResults, error) {
	return r.Identity(ctx, false)
}

func (r *Run) Finalize(ctx context.Context, inputComplete bool) (IdentityResults, error) {
	out, err := r.Identity(ctx, inputComplete)
	if r != nil {
		r.finalized = true
	}
	return out, err
}
func (r *Run) Close() error {
	if r == nil || r.closed {
		return errors.New("xsd: repeated or nil close")
	}
	r.closed = true
	err := r.meter.Release(r.retained + r.scratchRetained)
	r.scratchRetained = 0
	r.retained = 0
	r.nodes = nil
	r.states = nil
	r.context = Context{}
	r.identityResults = nil
	r.identityIssues = nil
	r.identitySeen = nil
	return err
}
func (c *Context) setFamily(name string) {
	if c.local != nil {
		c.local.family = name
	}
}
func (c *Context) step(n int64) {
	if c != nil && c.run != nil {
		c.run.step(n)
	}
}
func (c *Context) reserve(n int64) {
	if c != nil && c.run != nil {
		c.run.reserve(n)
	}
}
func (c *Context) observeScalar(assessment SimpleAssessment, valid bool) {
	if c.local == nil {
		return
	}
	s := c.local
	s.scalarAssessment = assessment
	s.scalarAssessed = valid
	if s.attribute == (QName{}) {
		s.scalar = ScalarResult{Known: valid, Complete: s.results.Scalar.Complete, Defaulted: s.defaulted, Lexeme: s.subject.Text, EffectiveLexeme: assessment.Lexical, Builtin: assessment.Builtin, Value: assessment.Value}
	}
}
func (c *Context) identityDetail(path, constraint, message string) {
	r := c.run
	if r == nil || r.identityOutput == nil {
		return
	}
	// Keep this scalar fact even when reserving diagnostic storage aborts.
	r.identityValid = false
	r.identityOutput.Valid = false
	if r.identitySeen[c.identityDiagnostic] {
		return
	}
	r.reserve(c.sum(int64(len(path)), int64(len(constraint)), int64(len(message))))
	// Distinct permanent map entries are admitted before either index mutates.
	r.reserve(64 + int64(unsafe.Sizeof(identityDiagnostic{})))
	id := c.identitySource.subject
	attribute := c.identitySource.attribute
	v, ok := r.identityResults[id]
	if !ok {
		r.reserve(64 + int64(unsafe.Sizeof(SubjectID(0))) + int64(unsafe.Sizeof(Result{})))
		v = freshResult()
	}
	v.Valid = false
	detail := Detail{Issue: Issue{path, constraint, message}, Subject: id, Attribute: attribute}
	v.Issues = appendRetained(c, v.Issues, detail)
	issues := appendRetained(c, r.identityIssues, detail)
	r.identityResults[id] = v
	r.identityIssues = issues
	r.identitySeen[c.identityDiagnostic] = true
	r.identityOutput.Issues = r.identityIssues
}

func (c *Context) attributeResult(schema AttributeSchema, value string, present, typeValid, declarationValid bool) {
	if c.local == nil {
		return
	}
	s := c.local
	use := s.run.attributeUse(s.actual, schema.Name)
	scalar := ScalarResult{Known: typeValid, Complete: s.attributeKnown && c.AssessmentVersion == s.attributeEpoch, Lexeme: value, EffectiveLexeme: s.scalarAssessment.Lexical, Builtin: s.scalarAssessment.Builtin, Value: s.scalarAssessment.Value}
	s.attributes = appendRetained(c, s.attributes, AttributeResult{Name: schema.Name, Use: use, Present: present, TypeValid: typeValid, DeclarationValid: declarationValid, Complete: scalar.Complete, Scalar: scalar})
}

// size validates arithmetic and host allocation width before any allocation.
func (c *Context) size(count, width int64) int64 {
	if count < 0 || width < 0 || (width != 0 && count > math.MaxInt64/width) {
		panic(runAbort{errors.New("xsd: allocation size overflow")})
	}
	n := count * width
	if uint64(n) > uint64(^uint(0)>>1) {
		panic(runAbort{errors.New("xsd: allocation exceeds host size")})
	}
	return n
}

// scratch accounts operation-local storage, released even if an operation aborts.
func (c *Context) scratch(n int64) {
	if c == nil || c.run == nil {
		return
	}
	r := c.run
	if n < 0 || n > math.MaxInt64-r.scratchRetained || n > math.MaxInt64-r.retained-r.scratchRetained {
		panic(runAbort{errors.New("xsd: temporary retention overflow")})
	}
	if err := r.meter.Reserve(n); err != nil {
		panic(runAbort{err})
	}
	r.scratchRetained += n
}
func (r *Run) releaseScratch() {
	n := r.scratchRetained
	if n == 0 {
		return
	}
	if err := r.meter.Release(n); err != nil {
		panic(runAbort{err})
	}
	r.scratchRetained = 0
}

// bytes bounds uninterruptible standard-library scans before entry. Checkpoints
// also make a large lexical operation cancellable before its library call.
func (c *Context) bytes(n int) {
	if c == nil || c.run == nil {
		return
	}
	for n > 0 {
		chunk := min(n, 256)
		c.step(int64(chunk))
		n -= chunk
	}
}
func (c *Context) identity(n int64) {
	if c != nil && c.run != nil {
		if err := c.run.meter.Identity(n); err != nil {
			panic(runAbort{err})
		}
	}
}

func (c *Context) sum(parts ...int64) int64 {
	var n int64
	for _, part := range parts {
		if part < 0 || part > math.MaxInt64-n {
			panic(runAbort{errors.New("xsd: allocation size overflow")})
		}
		n += part
	}
	return c.size(n, 1)
}

func (c *Context) scalar(n int64) {
	if c != nil && c.run != nil {
		c.run.scalar(n)
	}
}

// Child occurrence identity comes from the grammar position, never display paths.
func (c *Context) childIssue(node *Node, path string, index int, constraint, message string) {
	if c.local != nil {
		old := c.local.issueSubject
		c.local.issueSubject = node.Children[index].ID
		defer func() { c.local.issueSubject = old }()
	}
	c.AddIssue(indexedChildPath(path, node.Children, index, c), constraint, message)
}
func (c *Context) proveIdentityFailure() {
	if c.run != nil && c.run.identityOutput != nil {
		c.run.identityValid = false
		c.run.identityOutput.Valid = false
	}
}

// A stopped local operation cannot publish its untouched pass defaults as
// completed evidence. Proved failures and exact scalar facts remain available.
func (s *boundState) abort() {
	for _, result := range []*Result{&s.results.Binding, &s.results.Declaration, &s.results.Attributes, &s.results.Scalar, &s.results.Content} {
		result.Complete = false
	}
}
