package xsd

import (
	"context"
	"errors"
)

// OpenElement is an element whose attributes are complete and whose children
// are still arriving. PushChild gives each child, as it opens, the witness
// that Bind gives it once the element closes: by prefix binding, a child's
// declaration depends only on the children up to and including it.
type OpenElement struct {
	id      SubjectID
	owner   TypeID
	matcher *prefixMatcher
	next    int
	closed  bool
}

// Open prepares the witnesses of an element's children before it closes. It
// returns nil, without error, when Bind could give the children other
// witnesses than their prefix does, or none: an incomplete attribute
// enumeration, any schema-instance attribute, an abstract, fixed or defaulted
// element declaration, or a type that is not a complex type with element
// content. The caller still binds the element with Bind once it closes, and
// must then find the same witnesses for the children it bound early. For a
// current model the order of the children is a fact of the closed element:
// Open assumes it will be known.
func (r *Run) Open(ctx context.Context, use Use, subject Subject) (open *OpenElement, err error) {
	if err = r.check(ctx); err != nil {
		return nil, err
	}
	if use.schema != r.schema || use.original == nil || subject.ID == 0 {
		return nil, errors.New("xsd: invalid schema use or subject")
	}
	r.current = ctx
	defer recoverRun(&err)
	defer r.releaseScratch()
	r.step(1)
	if !subject.AttributesComplete {
		return nil, nil
	}
	for _, a := range subject.Attributes {
		r.step(1)
		if !a.NamespaceDeclaration && a.Name.Space == XSINamespace {
			return nil, nil
		}
	}
	c := &r.context
	schema, found := c.ResolveElement(use.original)
	if !found || schema.Name != Name(subject.Name) || schema.Abstract || schema.Fixed != nil || schema.Default != nil {
		return nil, nil
	}
	simple, complex, builtin, found := c.ResolveType(&schema.Type)
	if !found || simple != nil || complex == nil || complex.Abstract || !c.ValidTypeMetadata(TypeIdentity{Simple: simple, Complex: complex, Builtin: builtin}) {
		return nil, nil
	}
	effective, ok := c.EffectiveComplex(complex)
	if !ok || effective.Simple != nil || effective.Particle == nil {
		return nil, nil
	}
	owner, _ := r.typeID(&schema.Type)
	matcher, ok := c.newPrefixMatcher(effective.Particle)
	if !ok {
		return nil, nil
	}
	matcher.retained = true
	// Children bound before this element hold its node as their parent's;
	// Bind reuses it.
	if r.nodes[subject.ID] == nil {
		r.reserveNode()
		r.nodes[subject.ID] = &Node{ID: subject.ID, Name: subject.Name}
	}
	r.reserve(openElementBytes)
	r.opened = true
	return &OpenElement{id: subject.ID, owner: owner, matcher: matcher}, nil
}

// openElementBytes covers an open element and its matcher.
const openElementBytes = 256

// PushChild returns the witness of the next child of an open element, or
// false when the child has none: its declared use is unavailable, or the
// element's content model rejects it or an earlier child, in which case no
// later child has one either.
func (r *Run) PushChild(ctx context.Context, open *OpenElement, child Child) (w Witness, ok bool, err error) {
	if err = r.check(ctx); err != nil {
		return
	}
	if open == nil || open.closed || child.ID == 0 || child.ID == open.id {
		return Witness{}, false, errors.New("xsd: invalid open element or child")
	}
	r.current = ctx
	defer recoverRun(&err)
	defer r.releaseScratch()
	r.step(1)
	index := open.next
	open.next++
	open.matcher.c = &r.context
	if open.matcher.stopped {
		return Witness{}, false, nil
	}
	r.context.scratch(nodeObjectBytes)
	match, bound := open.matcher.push(&Node{ID: child.ID, Name: child.Name})
	if !bound {
		return Witness{}, false, nil
	}
	if match.Particle.Kind == ParticleAny {
		mode := WitnessStrict
		if match.Particle.Any.ProcessContents == "skip" {
			mode = WitnessSkip
		} else if match.Particle.Any.ProcessContents == "lax" {
			mode = WitnessLax
		}
		return Witness{Index: index, Child: child.ID, Mode: mode}, true, nil
	}
	use := r.witnessUse(open.owner, match.Particle)
	if use.schema == nil {
		// Bind gives this child no witness but continues with the next.
		return Witness{}, false, nil
	}
	resolved := use
	if resolved.original.Reference.Local != "" {
		if target, ok := r.schema.Root(resolved.original.Reference); ok {
			resolved = target
		}
	}
	return Witness{Index: index, Child: child.ID, Use: use, ResolvedDeclaration: resolved, Mode: WitnessDeclared}, true, nil
}

// CloseOpen releases an open element's matcher. Its node stays for Bind.
func (r *Run) CloseOpen(open *OpenElement) error {
	if r == nil || r.closed || open == nil || open.closed {
		return errors.New("xsd: invalid open element close")
	}
	open.closed = true
	open.matcher.c = &r.context
	var err error
	func() {
		defer recoverRun(&err)
		open.matcher.dropKept()
	}()
	r.retained -= openElementBytes
	if releaseErr := r.meter.Release(openElementBytes); err == nil {
		err = releaseErr
	}
	return err
}
