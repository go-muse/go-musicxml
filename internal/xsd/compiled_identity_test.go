package xsd

import (
	"testing"
	"unsafe"
)

func TestCompiledCacheIdentityIsExactAndOwned(t *testing.T) {
	c, err := Compile(compilerFixture(), CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	other, err := Compile(compilerFixture(), CompileLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if (TypeID{}).CacheIdentity().BelongsTo(c) || (Use{}).CacheIdentity().BelongsTo(c) {
		t.Fatal("zero handle has cache authority")
	}
	seen := map[CacheIdentity]bool{}
	for _, typ := range c.Components() {
		id := typ.CacheIdentity()
		if !id.BelongsTo(c) || id.BelongsTo(other) || seen[id] {
			t.Fatal("type identity is missing, foreign or duplicated")
		}
		seen[id] = true
	}
	for _, use := range c.Uses() {
		id := use.CacheIdentity()
		if !id.BelongsTo(c) || id.BelongsTo(other) || seen[id] {
			t.Fatal("use identity is missing, foreign or collapsed")
		}
		seen[id] = true
	}
	root, _ := c.Root(QName{Local: "root"})
	again, _ := c.Root(QName{Local: "root"})
	if root.CacheIdentity() != again.CacheIdentity() {
		t.Fatal("root query recreated identity")
	}
	owner, _ := root.DeclaredType()
	particle := c.data.Types[QName{Local: "T"}].Complex.Particle
	a := c.particleUses[particleUseKey{owner: owner, particle: particle.Children[0]}][0]
	b := c.particleUses[particleUseKey{owner: owner, particle: particle.Children[1]}][0]
	if a.Declaration() != b.Declaration() || a.CacheIdentity() == b.CacheIdentity() || a.CacheIdentity() == root.CacheIdentity() {
		t.Fatal("same-name exact slots/root-child were merged")
	}
	if inherited := c.inheritedParticleUses[particle.Children[0]]; inherited.CacheIdentity() != a.CacheIdentity() {
		t.Fatal("inherited use lost original identity")
	}
	first, _ := c.typeID(&TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}})
	second, _ := c.typeID(&TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}})
	if first.CacheIdentity() != second.CacheIdentity() {
		t.Fatal("builtin query recreated identity")
	}
	foreign, _ := other.typeID(&TypeRef{Name: QName{Space: XSDNamespace, Local: "string"}})
	if first.CacheIdentity() == foreign.CacheIdentity() {
		t.Fatal("separate compiled owners share cache identity")
	}
}

// The shared identity record replaces inline path headers in copied handles.
const _ = uint64(compiledIdentityBytes) - uint64(unsafe.Sizeof(compiledIdentity{}))
const _ = 80 - uint64(unsafe.Sizeof(TypeID{}))
const _ = 120 - uint64(unsafe.Sizeof(Use{}))
const _ = 16 - uint64(unsafe.Sizeof(CacheIdentity{}))

func TestCompiledIdentityAdmissionAndOverflow(t *testing.T) {
	for _, limit := range []int64{63, 64} {
		c := &Compiled{}
		b, err := newCompileBudget(CompileLimits{MaxBytes: limit, MaxRecords: 1})
		if err != nil {
			t.Fatal(err)
		}
		identity, err := c.newIdentity(b, "owner", "path")
		if limit == 63 {
			if err == nil || identity != nil || c.nextIdentity != 0 || b.bytes != 0 || b.records != 0 {
				t.Fatalf("unadmitted identity published: %+v %v", identity, err)
			}
		} else if err != nil || identity == nil || identity.owner != c || identity.ordinal != 1 || identity.ownerPath != "owner" || identity.path != "path" || b.bytes != 64 || b.records != 1 {
			t.Fatalf("exact64-byte identity lease: %+v %v", identity, err)
		}
	}
	c := &Compiled{nextIdentity: ^uint64(0)}
	b, _ := newCompileBudget(CompileLimits{})
	if identity, err := c.newIdentity(b, "", ""); err == nil || identity != nil || b.bytes != 0 || c.nextIdentity != ^uint64(0) {
		t.Fatal("identity ordinal wrapped or charged after overflow")
	}
}

func TestCompiledIdentityIsAdmittedBeforeIndexPublication(t *testing.T) {
	for _, short := range []int64{0, 1} {
		c := &Compiled{simpleTypes: map[*SimpleSchema]TypeID{}, complexTypes: map[*ComplexSchema]TypeID{}}
		path := "/types/0"
		b, _ := newCompileBudget(CompileLimits{MaxBytes: 192 + int64(len(path)) + 64 - short})
		typ := TypeID{schema: c, simple: &SimpleSchema{}}
		err := c.addType(b, &typ, path)
		if short == 1 {
			if err == nil || typ.identity != nil || len(c.components) != 0 || len(c.simpleTypes) != 0 || c.nextIdentity != 0 {
				t.Fatal("unadmitted type identity published")
			}
		} else if err != nil || typ.identity == nil || len(c.components) != 1 || b.bytes != 192+int64(len(path))+64 || b.records != 2 {
			t.Fatalf("type index plus independent64-byte identity lease: %v %+v", err, b)
		}
		c = &Compiled{}
		owner, path := "/types/0", "/types/0/particle/element"
		b, _ = newCompileBudget(CompileLimits{MaxBytes: 288 + int64(len(owner)) + int64(len(path)) + 64 - short})
		use := Use{schema: c}
		err = c.addUse(b, &use, owner, path)
		if short == 1 {
			if err == nil || use.identity != nil || len(c.uses) != 0 || c.nextIdentity != 0 {
				t.Fatal("unadmitted use identity published")
			}
		} else if err != nil || use.identity == nil || len(c.uses) != 1 || b.bytes != 288+int64(len(owner))+int64(len(path))+64 || b.records != 2 {
			t.Fatalf("use index plus independent64-byte identity lease: %v %+v", err, b)
		}
	}
}
