package xsd

import "testing"

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
