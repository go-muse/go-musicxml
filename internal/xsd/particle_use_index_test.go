package xsd

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// Both derived types reuse one base grammar, while unrelated owners contain
// homonymous declarations that must never influence inherited-use selection.
func inheritedUseFixture(unrelated int) SchemaData {
	item := QName{Local: "item"}
	base := &ComplexSchema{Form: ComplexDirect, Particle: &ParticleSchema{
		Kind: ParticleElement, Occurrence: Occurrence{Min: 1, Max: 1},
		Element: &ElementSchema{Reference: item},
	}}
	data := SchemaData{
		Types:    map[QName]*TypeSchema{{Local: "Base"}: {Complex: base}},
		Elements: map[QName]*ElementSchema{item: {Name: item, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "integer"}}}},
	}
	for _, name := range []string{"DerivedA", "DerivedB"} {
		typeName := QName{Local: name}
		data.Types[typeName] = &TypeSchema{Complex: &ComplexSchema{
			Form: ComplexComplexContentExtension, Base: &TypeRef{Name: QName{Local: "Base"}},
			Particle: &ParticleSchema{Kind: ParticleElement, Occurrence: Occurrence{Min: 1, Max: 1},
				Element: &ElementSchema{Name: QName{Local: "tail"}, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}}},
		}}
		data.Elements[typeName] = &ElementSchema{Name: typeName, Type: TypeRef{Name: typeName}}
	}
	for i := 0; i < unrelated; i++ {
		data.Types[QName{Local: fmt.Sprintf("Unused%04d", i)}] = &TypeSchema{Complex: &ComplexSchema{
			Form: ComplexDirect, Particle: &ParticleSchema{Kind: ParticleElement, Occurrence: Occurrence{Min: 1, Max: 1},
				Element: &ElementSchema{Name: item, Type: TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}}}},
		}}
	}
	return data
}

func bindInheritedUse(t *testing.T, c *Compiled, typeName string, limit int64) (work, peak int64, err error) {
	t.Helper()
	m := &boundaryMeter{workLimit: limit}
	r, err := NewRun(c, m)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := r.Close(); closeErr != nil || m.buffer != 0 {
			t.Errorf("close=%v retained=%d", closeErr, m.buffer)
		}
	}()
	name := QName{Local: typeName}
	u, _ := c.Root(name)
	b, err := r.Bind(context.Background(), u, Subject{
		ID: 1, Name: name, Path: "/root", TextKnown: true, AttributesComplete: true, ChildrenComplete: true,
		Children: []Child{{ID: 2, Name: QName{Local: "item"}, Path: "/root/item"}, {ID: 3, Name: QName{Local: "tail"}, Path: "/root/tail"}},
	})
	if err != nil {
		return m.work, m.peak, err
	}
	w := r.Witnesses(b)
	content := r.Results(b).Content
	if len(w) != 2 || !content.Valid || !content.Complete {
		t.Fatalf("content=%+v witnesses=%+v", content, w)
	}
	baseOwner := c.namedTypes[QName{Local: "Base"}]
	derivedOwner := c.namedTypes[name]
	baseParticle := baseOwner.complex.Particle
	baseUse := c.particleUses[particleUseKey{baseOwner, baseParticle}][0]
	if w[0].Use != baseUse || w[0].Use.OwnerOccurrence() != baseOwner.Occurrence() ||
		w[1].Use.OwnerOccurrence() != derivedOwner.Occurrence() || w[0].Use.Occurrence() == w[1].Use.Occurrence() {
		t.Fatalf("wrong exact owner/use: %+v", w)
	}
	global, _ := c.Root(QName{Local: "item"})
	if w[0].ResolvedDeclaration != global || w[0].Use == global || w[0].Use.original.Reference != (QName{Local: "item"}) {
		t.Fatalf("reference use was replaced by global declaration: %+v", w[0])
	}
	return m.work, m.peak, nil
}

func TestInheritedParticleLookupWorkIgnoresUnrelatedUses(t *testing.T) {
	var baselineWork, baselinePeak int64
	for _, unrelated := range []int{0, 1, 64, 512} {
		c, err := Compile(inheritedUseFixture(unrelated), CompileLimits{})
		if err != nil {
			t.Fatal(err)
		}
		work, peak, err := bindInheritedUse(t, c, "DerivedA", 0)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("unrelated uses=%d work=%d peak=%d", unrelated, work, peak)
		if unrelated == 0 {
			baselineWork, baselinePeak = work, peak
		} else if work != baselineWork || peak != baselinePeak {
			t.Errorf("unreached uses changed binding resources: work=%d want=%d peak=%d want=%d", work, baselineWork, peak, baselinePeak)
		}
	}
}

func TestInheritedParticleLookupRepeatableExactBudget(t *testing.T) {
	c, err := Compile(inheritedUseFixture(64), CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"DerivedA", "DerivedB"} {
		work, peak, err := bindInheritedUse(t, c, name, 0)
		if err != nil {
			t.Fatal(err)
		}
		for range 32 {
			gotWork, gotPeak, err := bindInheritedUse(t, c, name, work)
			if err != nil || gotWork != work || gotPeak != peak {
				t.Fatalf("nonrepeatable work: got=%d/%d want=%d/%d err=%v", gotWork, gotPeak, work, peak, err)
			}
		}
		if _, _, err := bindInheritedUse(t, c, name, work-1); !errors.Is(err, errMeterStop) {
			t.Fatalf("below exact budget: %v", err)
		}
		if _, _, err := bindInheritedUse(t, c, name, work+1); err != nil {
			t.Fatalf("above exact budget: %v", err)
		}
	}
}

func TestInheritedParticleLookupRetainsAmbiguity(t *testing.T) {
	data := inheritedUseFixture(0)
	base := data.Types[QName{Local: "Base"}].Complex
	data.Types[QName{Local: "OtherBase"}] = &TypeSchema{Complex: &ComplexSchema{Form: ComplexDirect, Particle: base.Particle}}
	c, err := Compile(data, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	r, err := NewRun(c, &boundaryMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	u, _ := c.Root(QName{Local: "DerivedA"})
	b, err := r.Bind(context.Background(), u, Subject{ID: 1, Name: QName{Local: "DerivedA"}, Path: "/root", TextKnown: true, AttributesComplete: true, ChildrenComplete: true,
		Children: []Child{{ID: 2, Name: QName{Local: "item"}}, {ID: 3, Name: QName{Local: "tail"}}}})
	if err != nil {
		t.Fatal(err)
	}
	w, content := r.Witnesses(b), r.Results(b).Content
	if len(w) != 1 || w[0].Child != 3 || !content.Valid || content.Complete ||
		len(content.Limitations) != 1 || content.Limitations[0].Reason != "exact-declaration-use-unavailable" {
		t.Fatalf("ambiguous inherited use guessed: witnesses=%+v content=%+v", w, content)
	}
	// An exact owner lookup still wins even when that pointer has another owner.
	ownedBase := c.namedTypes[QName{Local: "Base"}]
	s := &boundState{run: r, subject: Subject{Children: []Child{{ID: 4}}}, actual: &TypeRef{Name: QName{Local: "Base"}}, results: freshResults(),
		matches: []ElementMatch{{Particle: ownedBase.complex.Particle, Index: 0}}}
	r.buildWitnesses(s)
	if len(s.witnesses) != 1 || s.witnesses[0].Use.OwnerOccurrence() != ownedBase.Occurrence() {
		t.Fatalf("unambiguous owner use was lost: %+v", s.witnesses)
	}
}

func TestCompileInheritedParticleIndexAdmission(t *testing.T) {
	particle := &ParticleSchema{}
	c := &Compiled{}
	u := Use{schema: c, ownerPath: "/owner", path: "/owner/particle"}
	b, err := newCompileBudget(CompileLimits{MaxBytes: 351})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.indexInheritedParticleUse(b, particle, u); err == nil || c.inheritedParticleUses != nil || b.bytes != 0 {
		t.Fatalf("index allocated before admission: err=%v bytes=%d index=%+v", err, b.bytes, c.inheritedParticleUses)
	}
	b, err = newCompileBudget(CompileLimits{MaxBytes: 352, MaxRecords: 1, MaxEdges: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.indexInheritedParticleUse(b, particle, u); err != nil || c.inheritedParticleUses[particle] != u || b.bytes != 352 || b.records != 1 || b.edges != 1 {
		t.Fatalf("exact index admission: err=%v budget=%+v index=%+v", err, b, c.inheritedParticleUses)
	}
	if err := c.indexInheritedParticleUse(b, &ParticleSchema{}, u); err == nil || len(c.inheritedParticleUses) != 1 {
		t.Fatalf("new index entry escaped exhausted budget: err=%v entries=%d", err, len(c.inheritedParticleUses))
	}
	for _, path := range []string{"/owner/other-slot", "/other-owner/particle"} {
		other := Use{schema: c, ownerPath: "/other-owner", path: path}
		if err := c.indexInheritedParticleUse(b, particle, other); err != nil || c.inheritedParticleUses[particle] != (Use{}) || b.bytes != 352 {
			t.Fatalf("ambiguity grew storage or selected a use: err=%v budget=%+v index=%+v", err, b, c.inheritedParticleUses)
		}
	}
}

func TestCompileInheritedParticleIndexLimits(t *testing.T) {
	data := inheritedUseFixture(64)
	c, err := Compile(data, CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		n      int64
		limits func(int64) CompileLimits
	}{
		{"bytes", c.LogicalBytes(), func(n int64) CompileLimits { return CompileLimits{MaxBytes: n} }},
		{"records", c.compileRecordCount, func(n int64) CompileLimits { return CompileLimits{MaxRecords: n} }},
		{"edges", c.compileEdgeCount, func(n int64) CompileLimits { return CompileLimits{MaxEdges: n} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, delta := range []int64{-1, 0, 1} {
				_, err := Compile(data, test.limits(test.n+delta))
				if (err != nil) != (delta < 0) {
					t.Errorf("boundary %d%+d: %v", test.n, delta, err)
				}
			}
		})
	}
}

func TestInheritedParticleLookupChecksCancellationBeforeFallback(t *testing.T) {
	c, err := Compile(inheritedUseFixture(64), CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m := &scalarMeter{cancel: cancel, cancelAt: 2}
	r, err := NewRun(c, m)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	r.current = ctx
	p := c.namedTypes[QName{Local: "Base"}].complex.Particle
	s := &boundState{run: r, subject: Subject{Children: []Child{{ID: 2}}}, actual: &TypeRef{Name: QName{Local: "DerivedA"}}, results: freshResults(),
		matches: []ElementMatch{{Particle: p, Index: 0}}}
	func() { defer recoverRun(&err); r.buildWitnesses(s) }()
	if !errors.Is(err, context.Canceled) || m.calls != 2 || len(s.witnesses) != 0 || r.scratchRetained != 0 {
		t.Fatalf("fallback bypassed cancellation: err=%v calls=%d witnesses=%+v scratch=%d", err, m.calls, s.witnesses, r.scratchRetained)
	}
}
