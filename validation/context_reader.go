package validation

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"github.com/go-muse/go-musicxml/internal/xsd"
)

// contextReader borrows admitted schema facts. It owns only its first error;
// callers propagate err at every provider boundary, never substitute absence.
type contextReader struct {
	ctx             context.Context
	s               *Session
	b               *contextBindings
	err             error
	attributeCache  map[xsd.Binding]*contextAttributeTable
	attributeIntern map[contextAttributeInternKey]*contextAttributeTable
	elementCache    map[DeclarationID][]contextRole
	ownerCache      map[ComponentID]*contextAttributeTable
}

func (r *contextReader) step(n int64) bool {
	if r.err == nil {
		r.err = r.s.step(r.ctx, n)
	}
	return r.err == nil
}
func (r *contextReader) sameDeclaration(a, b DeclarationID) bool {
	if !r.step(contextDeclarationBytes(a) + contextDeclarationBytes(b)) {
		return false
	}
	return a == b
}
func (r *contextReader) matches(n *factNode, role contextRole) bool {
	if n == nil || r.b == nil || !r.step(1) || n.header.Declaration.State != EvidenceKnown {
		return false
	}
	if r.b.Elements != nil {
		if !r.step(int64(len(role)) + contextDeclarationBytes(n.header.Declaration.Value)) {
			return false
		}
		_, ok := r.b.Elements[role][n.header.Declaration.Value]
		return ok
	}
	for _, u := range r.b.Roles[role] {
		if !u.Attribute && r.sameDeclaration(n.header.Declaration.Value, u.Declaration) {
			return true
		}
	}
	return false
}

// roles returns a borrowed immutable set of exact element roles. The lookup
// and returned key traversal are charged once; callers must propagate err even
// when the result is empty. Unavailable declaration evidence proves no exclusion.
func (r *contextReader) roles(n *factNode) []contextRole {
	if !r.step(1) || n == nil || r.b == nil || n.header.Declaration.State != EvidenceKnown {
		return nil
	}
	d := n.header.Declaration.Value
	if !r.step(contextDeclarationBytes(d)) {
		return nil
	}
	roles := r.b.ElementRoles[d]
	if r.b.ElementRoles == nil {
		var ready bool
		roles, ready = r.elementCache[d]
		if !ready {
			// Manually assembled private test bindings lack the compiled index.
			// Keep the compatibility fallback bounded and cached by declaration.
			var scratch int64
			defer func() {
				if scratch != 0 {
					_ = r.s.account.Release(BudgetBufferedBytes, scratch)
				}
			}()
			if r.b.Elements != nil {
				for role, declarations := range r.b.Elements {
					if !r.step(1 + int64(len(role)) + contextDeclarationBytes(d)) {
						return nil
					}
					if _, ok := declarations[d]; ok {
						if r.err = r.s.account.Charge(BudgetBufferedBytes, 64); r.err != nil {
							return nil
						}
						scratch += 64
						roles = append(roles, role)
					}
				}
			} else {
				for role, uses := range r.b.Roles {
					if !r.step(1 + int64(len(role))) {
						return nil
					}
					for _, u := range uses {
						if !r.step(1) {
							return nil
						}
						if !u.Attribute && r.sameDeclaration(d, u.Declaration) {
							if r.err = r.s.account.Charge(BudgetBufferedBytes, 64); r.err != nil {
								return nil
							}
							scratch += 64
							roles = append(roles, role)
							break
						}
					}
				}
			}
			if !contextSortRoles(r, roles, func(v contextRole) contextRole { return v }) {
				return nil
			}
			if !r.step(1) {
				return nil
			}
			if r.err = r.s.retain(256 + contextDeclarationBytes(d)); r.err != nil {
				return nil
			}
			if r.elementCache == nil {
				r.elementCache = make(map[DeclarationID][]contextRole)
			}
			r.elementCache[d] = roles
			// Transfer the already admitted backing storage to the cache without
			// charging it twice or releasing the borrowed immutable result.
			r.s.retained += scratch
			scratch = 0
		}
	}
	for _, role := range roles {
		if !r.step(1 + int64(len(role))) {
			return nil
		}
	}
	return roles
}

// attributeRoles returns the immutable union of exact public owner slots and
// selected uses actually assessed for this binding. It is equivalent to probing
// attributeOwner for every compiled role, including inherited and incomplete
// enumeration cases, but never infers a use from another node of the same type.
func (r *contextReader) attributeRoles(n *factNode) []contextRole {
	if !r.step(1) || n == nil || r.b == nil {
		return nil
	}
	table := r.attributeTable(n)
	if r.err != nil || table == nil {
		return nil
	}
	for _, role := range table.roles {
		if !r.step(1 + int64(len(role))) {
			return nil
		}
	}
	return table.roles
}

type contextAttributeInternKey struct {
	actual   xsd.TypeID
	assembly Assembly
	schema   string
	digest   [32]byte
}

// attributeTable preserves per-binding discovery. ActualType alone is not a
// safe key: Run.Attributes omits missing required slots and omits known absences
// when the input axis is incomplete. Only fully enumerated, exactly equal tables
// may borrow one compact immutable table. A fingerprint collision cannot merge
// tables or introduce an unbounded collision-bucket scan.
func (r *contextReader) attributeTable(n *factNode) *contextAttributeTable {
	// A run binding identifies one immutable final assessed subject, including
	// its actual type and public owner evidence. Check that compact identity
	// before repeating any owner-key hashing on cached negative lookups.
	var binding xsd.Binding
	hasBinding := false
	if r.s.schema != nil && r.s.schema.run != nil && r.s.schema.active != nil {
		if !r.step(1) {
			return nil
		}
		bound, ok := r.s.schema.nodes[n.header.ID]
		if ok && bound.available {
			binding, hasBinding = bound.binding, true
			if table, ready := r.attributeCache[binding]; ready {
				return table
			}
		}
	}
	var direct *contextAttributeTable
	if n.header.Type.State == EvidenceKnown {
		if !r.step(contextComponentBytes(n.header.Type.Value)) {
			return nil
		}
		direct = r.ownerTable(n.header.Type.Value)
		if r.err != nil {
			return nil
		}
	}
	if !hasBinding {
		return direct
	}

	var selected []contextAttributeRole
	var scratch int64
	defer func() {
		if scratch != 0 {
			_ = r.s.account.Release(BudgetBufferedBytes, scratch)
		}
	}()
	appendSelected := func(v contextAttributeRole) bool {
		// Doubling covers both backing arrays during geometric growth.
		if r.err = r.s.account.Charge(BudgetBufferedBytes, 256); r.err != nil {
			return false
		}
		scratch += 256
		selected = append(selected, v)
		return true
	}
	for attr := range r.s.schema.run.Attributes(binding) {
		d := r.s.schema.active.declaration(attr.Use)
		if !r.step(contextDeclarationBytes(d)) {
			return nil
		}
		for _, v := range r.b.AttributeRoles[d] {
			if !r.step(1 + int64(len(v.role))) {
				return nil
			}
			if direct != nil {
				if _, exists := direct.selectors[v.role]; exists {
					continue
				}
			}
			exists := false
			for _, previous := range selected {
				if !r.step(1 + int64(len(previous.role)+len(v.role))) {
					return nil
				}
				if previous.role == v.role {
					exists = true
					break
				}
			}
			if !exists && !appendSelected(v) {
				return nil
			}
		}
	}
	table := direct
	var internKey contextAttributeInternKey
	intern := false
	fresh := len(selected) != 0
	if fresh {
		if direct != nil {
			for _, role := range direct.roles {
				if !r.step(1 + int64(len(role))) {
					return nil
				}
				if !appendSelected(contextAttributeRole{role, direct.selectors[role]}) {
					return nil
				}
			}
		}
		if !contextSortRoles(r, selected, func(v contextAttributeRole) contextRole { return v.role }) {
			return nil
		}
		if r.err != nil {
			return nil
		}
		digest, ok := r.attributeFingerprint(selected)
		if !ok {
			return nil
		}
		actual, known := r.s.schema.run.ActualType(binding)
		internKey = contextAttributeInternKey{actual, r.s.schema.active.assembly, r.s.schema.active.Digest(), digest}
		if !r.step(32 + contextComponentBytes(r.s.schema.active.component(actual)) + int64(len(internKey.assembly)+len(internKey.schema))) {
			return nil
		}
		if known {
			intern = true
			if previous := r.attributeIntern[internKey]; previous != nil {
				equal := len(previous.roles) == len(selected)
				for _, v := range selected {
					a, exists := previous.selectors[v.role]
					if !r.step(1 + int64(len(v.role)) + contextAttributeSelectorBytes(a) + contextAttributeSelectorBytes(v.selector)) {
						return nil
					}
					equal = equal && exists && a == v.selector
				}
				if equal {
					table, fresh = previous, false
				} else {
					intern = false
				}
			}
		}
	}
	retained := int64(128)
	if r.attributeCache == nil {
		retained += 128
	}
	if fresh {
		retained += 512 + int64(len(selected))*256
		if intern {
			retained += 384
			if r.attributeIntern == nil {
				retained += 128
			}
		}
	}
	// Admit the final map/slice construction before any persistent allocation.
	// Enumeration and fingerprint work do not pay for these additional writes.
	if fresh {
		for _, v := range selected {
			if !r.step(1 + int64(len(v.role))) {
				return nil
			}
		}
	}
	if !r.step(1) {
		return nil
	}
	if r.err = r.s.retain(retained); r.err != nil {
		return nil
	}
	if fresh {
		table = &contextAttributeTable{selectors: make(map[contextRole]AttributeSelector, len(selected)), roles: make([]contextRole, len(selected))}
		for i, v := range selected {
			table.selectors[v.role] = v.selector
			table.roles[i] = v.role
		}
		if intern {
			if r.attributeIntern == nil {
				r.attributeIntern = make(map[contextAttributeInternKey]*contextAttributeTable)
			}
			r.attributeIntern[internKey] = table
		}
	}
	if r.attributeCache == nil {
		r.attributeCache = make(map[xsd.Binding]*contextAttributeTable)
	}
	r.attributeCache[binding] = table
	return table
}

// ownerTable's compatibility path is only for private hand-built bindings.
// Production bindings construct and account this index once at compilation.
func (r *contextReader) ownerTable(owner ComponentID) *contextAttributeTable {
	if r.b.OwnerTables != nil {
		return r.b.OwnerTables[owner]
	}
	if table, ready := r.ownerCache[owner]; ready {
		return table
	}
	count := 0
	for role, owners := range r.b.Attributes {
		if !r.step(1 + int64(len(role)) + contextComponentBytes(owner)) {
			return nil
		}
		if _, ok := owners[owner]; ok {
			count++
		}
	}
	if !r.step(1) {
		return nil
	}
	scratch := 512 + int64(count)*256
	if r.err = r.s.account.Charge(BudgetBufferedBytes, scratch); r.err != nil {
		return nil
	}
	defer func() {
		if scratch != 0 {
			_ = r.s.account.Release(BudgetBufferedBytes, scratch)
		}
	}()
	table := &contextAttributeTable{selectors: make(map[contextRole]AttributeSelector, count), roles: make([]contextRole, 0, count)}
	for role, owners := range r.b.Attributes {
		if !r.step(1 + int64(len(role)) + contextComponentBytes(owner)) {
			return nil
		}
		if a, ok := owners[owner]; ok {
			table.selectors[role] = a
			table.roles = append(table.roles, role)
		}
	}
	if !contextSortRoles(r, table.roles, func(v contextRole) contextRole { return v }) {
		return nil
	}
	if !r.step(1) {
		return nil
	}
	if r.ownerCache == nil {
		r.ownerCache = make(map[ComponentID]*contextAttributeTable)
	}
	r.ownerCache[owner] = table
	// All fallible work is complete; the cache now owns the admitted storage.
	r.s.retained += scratch
	scratch = 0
	return table
}

// contextSortRoles is a cancellation-aware bottom-up merge sort for the
// compiled role vocabulary (at most 4096 roles). Its O(n log n) comparisons,
// copies and bounded scratch array are charged; a refused non-sticky charge
// immediately stops the loop instead of letting a library sort continue.
type contextSortableRole interface {
	contextRole | contextAttributeRole
}

func contextSortRoles[T contextSortableRole](r *contextReader, values []T, role func(T) contextRole) bool {
	if r.err != nil {
		return false
	}
	if len(values) > 4096 {
		r.err = contract("context role index", "finite role vocabulary exceeded")
		return false
	}
	if !r.step(1) {
		return false
	}
	if len(values) < 2 {
		return true
	}
	// Both uses have fixed elements no larger than contextAttributeRole. The
	// generous per-entry allowance also covers allocator rounding.
	size := int64(len(values)) * 256
	if r.err = r.s.account.Charge(BudgetBufferedBytes, size); r.err != nil {
		return false
	}
	defer r.s.account.Release(BudgetBufferedBytes, size)
	buffer := make([]T, len(values))
	src, dst := values, buffer
	inInput := true
	for width := 1; width < len(values); width *= 2 {
		if !r.step(1) {
			return false
		}
		for start := 0; start < len(values); start += 2 * width {
			if !r.step(1) {
				return false
			}
			middle, end := min(start+width, len(values)), min(start+2*width, len(values))
			i, j := start, middle
			for k := start; k < end; k++ {
				if !r.step(1) {
					return false
				}
				left := j == end
				if i < middle && j < end {
					a, b := role(src[i]), role(src[j])
					if !r.step(int64(len(a) + len(b))) {
						return false
					}
					left = a <= b
				}
				if i < middle && left {
					dst[k] = src[i]
					i++
				} else {
					dst[k] = src[j]
					j++
				}
			}
		}
		src, dst = dst, src
		inInput = !inInput
	}
	if !inInput {
		for i, v := range src {
			if !r.step(1) {
				return false
			}
			values[i] = v
		}
	}
	return true
}

func contextAttributeSelectorBytes(a AttributeSelector) int64 {
	return contextDeclarationBytes(a.Declaration) + int64(len(a.Name.Space)+len(a.Name.Local))
}
func (r *contextReader) attributeFingerprint(selected []contextAttributeRole) ([32]byte, bool) {
	h := sha256.New()
	var buffer [1024]byte
	for _, v := range selected {
		for _, value := range []string{string(v.role), v.selector.Name.Space, v.selector.Name.Local, string(v.selector.Declaration.Assembly), v.selector.Declaration.OwnerOccurrence, v.selector.Declaration.Occurrence} {
			if !r.step(8) {
				return [32]byte{}, false
			}
			binary.BigEndian.PutUint64(buffer[:8], uint64(len(value)))
			_, _ = h.Write(buffer[:8])
			for len(value) != 0 {
				count := min(len(value), len(buffer))
				if !r.step(int64(count)) {
					return [32]byte{}, false
				}
				copy(buffer[:count], value[:count])
				_, _ = h.Write(buffer[:count])
				value = value[count:]
			}
		}
	}
	var sum [32]byte
	h.Sum(sum[:0])
	return sum, true
}

func (r *contextReader) enumerated(n *factNode) Evidence[bool] {
	if r.err != nil {
		return stateUnknown[bool]("context-interrupted")
	}
	old := stateReader{ctx: r.ctx, s: r.s}
	v := old.enumerated(n)
	r.err = old.err
	return v
}
func (r *contextReader) complete(n *factNode) Evidence[bool] {
	if r.err != nil {
		return stateUnknown[bool]("context-interrupted")
	}
	old := stateReader{ctx: r.ctx, s: r.s}
	v := old.complete(n)
	r.err = old.err
	return v
}
func (r *contextReader) scalar(n *factNode) Evidence[Atomic] {
	if n == nil {
		return Evidence[Atomic]{State: EvidenceAbsent}
	}
	if r.err != nil {
		return stateUnknown[Atomic]("context-interrupted")
	}
	v, e := r.s.localScalar(r.ctx, n, nil)
	r.err = e
	return v
}
func (r *contextReader) attribute(n *factNode, role contextRole) Evidence[Atomic] {
	if n == nil {
		return Evidence[Atomic]{State: EvidenceAbsent}
	}
	if r.err != nil {
		return stateUnknown[Atomic]("context-interrupted")
	}
	if r.b == nil {
		return Evidence[Atomic]{State: EvidenceUnsupported, Reason: "context-binding-unavailable"}
	}
	a, ok := r.attributeSelector(n, role)
	if r.err != nil {
		return stateUnknown[Atomic]("context-interrupted")
	}
	if !ok {
		if n.header.Type.State != EvidenceKnown {
			return stateUnavailable[ComponentID, Atomic](n.header.Type)
		}
		return Evidence[Atomic]{State: EvidenceUnsupported, Reason: "context-attribute-binding-unavailable"}
	}
	v, e := r.s.localScalar(r.ctx, n, &a)
	r.err = e
	if e == nil && v.State == EvidenceUnsupported && v.Reason == "exact-attribute-use-unavailable" {
		if invalid, ok := r.missingRequiredAttribute(n, a); ok {
			return invalid
		}
	}
	return v
}

// missingRequiredAttribute bridges the deliberate absence of a kernel
// AttributeResult for an omitted required slot. It proves only a selected use
// directly declared by the authoritative actual type, in the same assembly and
// schema graph. Equivalent compiled copies remain portable through catalog
// interning. Inherited omissions remain conservative rather than guessing
// whether a derived type retains or replaces its base attribute requirement.
func (r *contextReader) missingRequiredAttribute(n *factNode, a AttributeSelector) (Evidence[Atomic], bool) {
	if !r.step(contextDeclarationBytes(a.Declaration) + int64(len(a.Name.Space)+len(a.Name.Local)+len(r.b.requiredSchemaDigest))) {
		return Evidence[Atomic]{}, false
	}
	owner, required := r.b.RequiredAttributes[a]
	if !required || !n.closed || n.end.AttributesComplete.State != EvidenceKnown || !n.end.AttributesComplete.Value || r.s.schema == nil || r.s.schema.run == nil || r.s.schema.active == nil || r.s.schema.active.assembly != a.Declaration.Assembly || r.b.requiredSchemaDigest == "" || r.s.schema.active.Digest() != r.b.requiredSchemaDigest {
		return Evidence[Atomic]{}, false
	}
	bound, ok := r.s.schema.nodes[n.header.ID]
	if !ok || !bound.available {
		return Evidence[Atomic]{}, false
	}
	actual, known := r.s.schema.run.ActualType(bound.binding)
	if !known {
		return Evidence[Atomic]{}, false
	}
	actualOwner := r.s.schema.active.component(actual)
	if !r.step(contextComponentBytes(actualOwner)+contextComponentBytes(owner)) || actualOwner != owner {
		return Evidence[Atomic]{}, false
	}
	// Exact expanded-name absence is independent of declaration matching. A
	// present or uncertain same-named fact with the wrong selector cannot be
	// turned into a missing required attribute by attributeEvidence's fallback.
	for _, input := range n.attributes {
		if !r.step(1 + int64(len(input.Name.Space)+len(input.Name.Local))) {
			return Evidence[Atomic]{}, false
		}
		if input.Name == a.Name && input.Presence.State != EvidenceAbsent && (input.Presence.State != EvidenceKnown || input.Presence.Value) {
			return Evidence[Atomic]{}, false
		}
	}
	invalid, err := r.s.schemaInvalidValue(r.ctx, n, XSDCheckAttributes)
	r.err = err
	return invalid, err == nil && invalid.State == EvidenceInvalid && invalid.Cause != nil
}

// attributeSelector uses compiled owner/type lookup on the common path. The
// uncommon inherited/actual-type path scans its authoritative attribute uses
// once, looking up exact declarations rather than every role candidate.
func (r *contextReader) attributeSelector(n *factNode, role contextRole) (AttributeSelector, bool) {
	if n == nil || r.b == nil || !r.step(1) {
		return AttributeSelector{}, false
	}
	if n.header.Type.State == EvidenceKnown {
		if !r.step(contextComponentBytes(n.header.Type.Value)) {
			return AttributeSelector{}, false
		}
		if a, ok := r.b.Attributes[role][n.header.Type.Value]; ok {
			return a, true
		}
	}
	if r.s.schema != nil && r.s.schema.run != nil {
		bound := r.s.schema.nodes[n.header.ID]
		if r.b.AttributeRoles != nil {
			if !r.step(1 + int64(len(role))) {
				return AttributeSelector{}, false
			}
			table := r.attributeTable(n)
			if table == nil || r.err != nil {
				return AttributeSelector{}, false
			}
			a, ok := table.selectors[role]
			return a, ok
		}
		for attr := range r.s.schema.run.Attributes(bound.binding) {
			d := r.s.schema.active.declaration(attr.Use)
			if !r.step(contextDeclarationBytes(d)) {
				return AttributeSelector{}, false
			}
			if a, ok := r.b.AttributeUses[role][d]; ok {
				return a, true
			}
		}
	}
	return AttributeSelector{}, false
}
func (r *contextReader) attributeOwner(n *factNode, role contextRole) bool {
	_, ok := r.attributeSelector(n, role)
	return ok
}

func (r *contextReader) child(n *factNode, role contextRole) (*factNode, Evidence[bool]) {
	complete := r.complete(n)
	if complete.State != EvidenceKnown || !complete.Value {
		return nil, complete
	}
	var found *factNode
	for _, ch := range n.children {
		if !r.step(1) {
			return nil, stateUnknown[bool]("context-interrupted")
		}
		if ch.header.Declaration.State != EvidenceKnown {
			return nil, stateUnavailable[DeclarationID, bool](ch.header.Declaration)
		}
		if r.matches(ch, role) {
			if found != nil {
				return nil, stateUnknown[bool]("context-multiple-children")
			}
			found = ch
		}
	}
	if found == nil {
		return nil, Evidence[bool]{State: EvidenceAbsent}
	}
	return found, complete
}
func (r *contextReader) childScalar(n *factNode, role contextRole) Evidence[Atomic] {
	ch, e := r.child(n, role)
	if e.State != EvidenceKnown {
		return stateUnavailable[bool, Atomic](e)
	}
	return r.scalar(ch)
}

// presence distinguishes encoded omission from an XSD-effective scalar default.
func (r *contextReader) presence(n *factNode, role contextRole) Evidence[bool] {
	if n == nil {
		return Evidence[bool]{State: EvidenceAbsent}
	}
	a, ok := r.attributeSelector(n, role)
	if r.err != nil {
		return stateUnknown[bool]("context-interrupted")
	}
	if !ok {
		if n.header.Type.State != EvidenceKnown {
			return stateUnavailable[ComponentID, bool](n.header.Type)
		}
		return Evidence[bool]{State: EvidenceUnsupported, Reason: "context-attribute-binding-unavailable"}
	}
	value, _, err := r.s.attributeEvidence(r.ctx, n, a)
	r.err = err
	return value
}
