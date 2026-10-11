package xsd

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
	"reflect"
	"sort"
	"strconv"
	"strings"
)

// CompileLimits bounds schema snapshot and structural-index construction. Zero
// selects the finite default; positive values may lower, but not raise, it.
// Bytes is a conservative, architecture-independent logical retention measure,
// not an allocator/RSS measurement. Records counts expanded objects and index
// entries; edges counts pointer links, collection slots and indexed use slots.
// LiteralBytes applies to every string, including names, forms and map keys.
// Compilation checks these bounds before allocating the corresponding storage.
type CompileLimits struct {
	MaxBytes, MaxRecords, MaxEdges, MaxLiteralBytes int64
}

var defaultCompileLimits = CompileLimits{64 << 20, 65536, 65536, 1 << 20}

// Compiled owns an immutable snapshot of a bounded schema metadata graph.
// Compilation does not validate unreachable schema semantics. Concurrent readers
// may share it; mutable assessment caches belong to individual runs.
type Compiled struct {
	data                                 SchemaData
	digest                               string
	logicalBytes                         int64
	compileRecordCount, compileEdgeCount int64
	roots                                map[QName]Use
	simpleTypes                          map[*SimpleSchema]TypeID
	complexTypes                         map[*ComplexSchema]TypeID
	builtinTypes                         map[string]TypeID
	namedTypes                           map[QName]TypeID
	particleUses                         map[particleUseKey][]Use
	attributeUses                        map[attributeUseKey]Use
	uses                                 []Use
	components                           []TypeID
	records                              map[graphPointer]uint64
}

// Use identifies one exact schema use, separately retaining the original use
// and its referenced declaration. It contains no mutable public metadata.
type Use struct {
	schema                *Compiled
	original, declaration *ElementSchema
	attribute             *AttributeSchema
	ownerPath, path       string
}

// Declaration returns the expanded declaration name, or the zero name.
func (u Use) Declaration() QName {
	if u.attribute != nil {
		return u.attribute.Name
	}
	if u.declaration != nil {
		return u.declaration.Name
	}
	return QName{}
}

// TypeName returns the named declared type; anonymous types have a zero name.
func (u Use) TypeName() QName {
	if u.attribute != nil {
		return u.attribute.Type.Name
	}
	if u.declaration != nil {
		return u.declaration.Type.Name
	}
	return QName{}
}

// OwnerOccurrence returns the exact assembly-neutral owning component path.
func (u Use) OwnerOccurrence() string { return u.ownerPath }

// Occurrence returns the exact assembly-neutral structural use path.
func (u Use) Occurrence() string { return u.path }

// OwnerPath is an alias for OwnerOccurrence.
func (u Use) OwnerPath() string { return u.ownerPath }

// Path is an alias for Occurrence.
func (u Use) Path() string { return u.path }

// TypeID identifies a named, anonymous or built-in type without exposing DTOs.
type TypeID struct {
	schema  *Compiled
	simple  *SimpleSchema
	complex *ComplexSchema
	builtin string
	name    QName
	path    string
}

// Name returns the named type's expanded name; anonymous types have a zero name.
func (t TypeID) Name() QName { return t.name }

// Occurrence returns the assembly-neutral structural component path.
func (t TypeID) Occurrence() string { return t.path }

type particleUseKey struct {
	owner    TypeID
	particle *ParticleSchema
}
type attributeUseKey struct {
	owner TypeID
	index int
}
type graphPointer struct {
	typ reflect.Type
	ptr uintptr
}

// Digest returns the canonical SHA-256 graph digest, including nil, empty,
// sharing, cycles, all metadata fields and sorted map keys, but no addresses.
func (c *Compiled) Digest() string {
	if c == nil {
		return ""
	}
	return c.digest
}

// LogicalBytes returns the bounded logical snapshot and structural-index size.
func (c *Compiled) LogicalBytes() int64 {
	if c == nil {
		return 0
	}
	return c.logicalBytes
}

// Root returns a global nonnil declaration use by exact expanded name.
func (c *Compiled) Root(name QName) (Use, bool) {
	if c == nil {
		return Use{}, false
	}
	u, ok := c.roots[name]
	return u, ok
}

// Roots returns an independent sorted copy of global nonnil declaration names.
func (c *Compiled) Roots() []QName {
	if c == nil {
		return nil
	}
	names := make([]QName, 0, len(c.roots))
	for name := range c.roots {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return lessSchemaName(names[i], names[j]) })
	return names
}

// Uses returns an independent copy of the immutable structural use handles.
func (c *Compiled) Uses() []Use {
	if c == nil {
		return nil
	}
	return append([]Use(nil), c.uses...)
}

// Components returns an independent copy of the immutable type handles.
func (c *Compiled) Components() []TypeID {
	if c == nil {
		return nil
	}
	return append([]TypeID(nil), c.components...)
}

// Compile snapshots and indexes metadata without eagerly assessing its semantic
// validity. The caller must not mutate data concurrently with this call; it may
// freely mutate its input after the call returns.
func Compile(data SchemaData, limits CompileLimits) (*Compiled, error) {
	b, err := newCompileBudget(limits)
	if err != nil {
		return nil, err
	}
	if err = b.audit(reflect.ValueOf(data)); err != nil {
		return nil, err
	}
	owned, digest, records := copySchemaGraph(data)
	c := &Compiled{data: owned, digest: digest, records: records, roots: make(map[QName]Use), simpleTypes: make(map[*SimpleSchema]TypeID), complexTypes: make(map[*ComplexSchema]TypeID), builtinTypes: make(map[string]TypeID), namedTypes: make(map[QName]TypeID), particleUses: make(map[particleUseKey][]Use), attributeUses: make(map[attributeUseKey]Use)}
	if err = c.index(b); err != nil {
		return nil, err
	}
	c.logicalBytes = b.bytes
	c.compileRecordCount, c.compileEdgeCount = b.records, b.edges
	return c, nil
}

type compileBudget struct {
	limits                CompileLimits
	bytes, records, edges int64
}

func newCompileBudget(in CompileLimits) (*compileBudget, error) {
	values := []*int64{&in.MaxBytes, &in.MaxRecords, &in.MaxEdges, &in.MaxLiteralBytes}
	defaults := []int64{defaultCompileLimits.MaxBytes, defaultCompileLimits.MaxRecords, defaultCompileLimits.MaxEdges, defaultCompileLimits.MaxLiteralBytes}
	for i, value := range values {
		if *value < 0 || *value > defaults[i] {
			return nil, fmt.Errorf("xsd: compile limit %d is outside [0,%d]", i, defaults[i])
		}
		if *value == 0 {
			*value = defaults[i]
		}
	}
	return &compileBudget{limits: in}, nil
}
func (b *compileBudget) add(bytes, records, edges int64) error {
	if bytes < 0 || bytes > b.limits.MaxBytes-b.bytes {
		return fmt.Errorf("xsd: compile byte limit exceeded")
	}
	if records < 0 || records > b.limits.MaxRecords-b.records {
		return fmt.Errorf("xsd: compile record limit exceeded")
	}
	if edges < 0 || edges > b.limits.MaxEdges-b.edges {
		return fmt.Errorf("xsd: compile edge limit exceeded")
	}
	b.bytes += bytes
	b.records += records
	b.edges += edges
	return nil
}
func (b *compileBudget) collection(length int, bytesPerSlot int64) error {
	n := int64(length)
	if n > b.limits.MaxEdges-b.edges || n > (b.limits.MaxBytes-b.bytes)/bytesPerSlot {
		return fmt.Errorf("xsd: compile collection limit exceeded")
	}
	return b.add(n*bytesPerSlot, 0, n)
}
func pointerKey(v reflect.Value) graphPointer { return graphPointer{v.Type(), v.Pointer()} }

// audit is iterative and does not sort or copy caller collections until all
// their lengths have been admitted. Cyclic pointers are visited exactly once.
func (b *compileBudget) audit(root reflect.Value) error {
	stack := []reflect.Value{root}
	seen := make(map[graphPointer]bool)
	for len(stack) != 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch v.Kind() {
		case reflect.Pointer:
			if err := b.add(16, 0, 1); err != nil {
				return err
			}
			if v.IsNil() || seen[pointerKey(v)] {
				continue
			}
			if err := b.add(64, 1, 0); err != nil {
				return err
			}
			seen[pointerKey(v)] = true
			stack = append(stack, v.Elem())
		case reflect.Struct:
			if err := b.add(32+32*int64(v.NumField()), 1, 0); err != nil {
				return err
			}
			for i := v.NumField() - 1; i >= 0; i-- {
				stack = append(stack, v.Field(i))
			}
		case reflect.Map:
			if err := b.add(64, 1, 0); err != nil {
				return err
			}
			if err := b.collection(v.Len(), 128); err != nil {
				return err
			}
			it := v.MapRange()
			for it.Next() {
				stack = append(stack, it.Key(), it.Value())
			}
		case reflect.Slice:
			if err := b.add(64, 1, 0); err != nil {
				return err
			}
			if err := b.collection(v.Len(), 64); err != nil {
				return err
			}
			for i := v.Len() - 1; i >= 0; i-- {
				stack = append(stack, v.Index(i))
			}
		case reflect.String:
			if int64(v.Len()) > b.limits.MaxLiteralBytes {
				return fmt.Errorf("xsd: compile literal limit exceeded")
			}
			if err := b.add(32+int64(v.Len()), 0, 0); err != nil {
				return err
			}
		case reflect.Bool, reflect.Uint8, reflect.Uint64:
			if err := b.add(8, 0, 0); err != nil {
				return err
			}
		default:
			return fmt.Errorf("xsd: unsupported schema metadata kind %s", v.Kind())
		}
	}
	return nil
}

type schemaCopyTask struct {
	source, target              reflect.Value
	mapTarget, mapKey, mapValue reflect.Value
}
type schemaPointerFixup struct {
	key    graphPointer
	target reflect.Value
}

func hashNumber(h hash.Hash, n uint64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], n)
	_, _ = h.Write(buf[:])
}
func hashText(h hash.Hash, text string) {
	hashNumber(h, uint64(len(text)))
	_, _ = h.Write([]byte(text))
}
func lessSchemaName(a, b QName) bool {
	return a.Space < b.Space || a.Space == b.Space && a.Local < b.Local
}
func sortedSchemaMapKeys(v reflect.Value) []reflect.Value {
	keys := v.MapKeys()
	sort.Slice(keys, func(i, j int) bool { return lessSchemaName(keys[i].Interface().(QName), keys[j].Interface().(QName)) })
	return keys
}

// walkSchema visits pointer targets once, iteratively. The second argument says
// whether a value is an embedded field/slot rather than a pointer allocation.
func walkSchema(root reflect.Value, visit func(reflect.Value, bool)) {
	type task struct {
		value    reflect.Value
		embedded bool
	}
	stack := []task{{root, false}}
	seen := map[graphPointer]bool{}
	for len(stack) != 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		v := current.value
		visit(v, current.embedded)
		switch v.Kind() {
		case reflect.Pointer:
			if v.IsNil() || seen[pointerKey(v)] {
				continue
			}
			seen[pointerKey(v)] = true
			stack = append(stack, task{v.Elem(), false})
		case reflect.Struct:
			for i := v.NumField() - 1; i >= 0; i-- {
				stack = append(stack, task{v.Field(i), true})
			}
		case reflect.Map:
			keys := sortedSchemaMapKeys(v)
			for i := len(keys) - 1; i >= 0; i-- {
				stack = append(stack, task{v.MapIndex(keys[i]), false}, task{keys[i], false})
			}
		case reflect.Slice:
			for i := v.Len() - 1; i >= 0; i-- {
				stack = append(stack, task{v.Index(i), true})
			}
		}
	}
}

// copySchemaGraph first identifies interior pointer targets, then allocates
// their containing storage before fixing links. In particular, a *TypeRef may
// point at an attribute's embedded Type and a *string at a QName field. Those
// aliases must survive just like separately allocated shared pointer objects.
func copySchemaGraph(data SchemaData) (SchemaData, string, map[graphPointer]uint64) {
	source := reflect.ValueOf(data)
	pointerTargets := map[graphPointer]bool{}
	walkSchema(source, func(v reflect.Value, _ bool) {
		if v.Kind() == reflect.Pointer && !v.IsNil() {
			pointerTargets[pointerKey(v)] = true
		}
	})
	interior := map[graphPointer]bool{}
	walkSchema(source, func(v reflect.Value, embedded bool) {
		if embedded && v.CanAddr() {
			key := pointerKey(v.Addr())
			if pointerTargets[key] {
				interior[key] = true
			}
		}
	})
	var output SchemaData
	targets := make(map[graphPointer]reflect.Value)
	records := make(map[graphPointer]uint64)
	register := func(key graphPointer, target reflect.Value) {
		if _, ok := targets[key]; ok {
			return
		}
		targets[key] = target
		records[pointerKey(target)] = uint64(len(targets))
	}
	var fixups []schemaPointerFixup
	stack := []schemaCopyTask{{source: source, target: reflect.ValueOf(&output).Elem()}}
	for len(stack) != 0 {
		task := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if task.mapTarget.IsValid() {
			task.mapTarget.SetMapIndex(task.mapKey, task.mapValue)
			continue
		}
		v, dst := task.source, task.target
		if v.CanAddr() {
			key := pointerKey(v.Addr())
			if pointerTargets[key] {
				register(key, dst.Addr())
			}
		}
		switch v.Kind() {
		case reflect.Pointer:
			if v.IsNil() {
				continue
			}
			key := pointerKey(v)
			if prior, ok := targets[key]; ok {
				dst.Set(prior)
				continue
			}
			if interior[key] {
				fixups = append(fixups, schemaPointerFixup{key, dst})
				continue
			}
			value := reflect.New(v.Type().Elem())
			register(key, value)
			dst.Set(value)
			stack = append(stack, schemaCopyTask{source: v.Elem(), target: value.Elem()})
		case reflect.Struct:
			for i := v.NumField() - 1; i >= 0; i-- {
				stack = append(stack, schemaCopyTask{source: v.Field(i), target: dst.Field(i)})
			}
		case reflect.Map:
			if v.IsNil() {
				continue
			}
			dst.Set(reflect.MakeMapWithSize(v.Type(), v.Len()))
			keys := sortedSchemaMapKeys(v)
			for i := len(keys) - 1; i >= 0; i-- {
				key, value := reflect.New(v.Type().Key()).Elem(), reflect.New(v.Type().Elem()).Elem()
				stack = append(stack, schemaCopyTask{mapTarget: dst, mapKey: key, mapValue: value}, schemaCopyTask{source: v.MapIndex(keys[i]), target: value}, schemaCopyTask{source: keys[i], target: key})
			}
		case reflect.Slice:
			if v.IsNil() {
				continue
			}
			dst.Set(reflect.MakeSlice(v.Type(), v.Len(), v.Len()))
			for i := v.Len() - 1; i >= 0; i-- {
				stack = append(stack, schemaCopyTask{source: v.Index(i), target: dst.Index(i)})
			}
		case reflect.String:
			dst.SetString(strings.Clone(v.String()))
		case reflect.Bool:
			dst.SetBool(v.Bool())
		case reflect.Uint8, reflect.Uint64:
			dst.SetUint(v.Uint())
		}
	}
	for _, fixup := range fixups {
		fixup.target.Set(targets[fixup.key])
	}
	return output, schemaGraphDigest(output, records), records
}

// Canonical graph IDs, including embedded pointer targets, encode sharing and
// cycles without hashing addresses. All scalar fields are included in order.
func schemaGraphDigest(data SchemaData, records map[graphPointer]uint64) string {
	h := sha256.New()
	hashText(h, "xsd-schema-graph-v1")
	seen := map[graphPointer]bool{}
	stack := []reflect.Value{reflect.ValueOf(data)}
	for len(stack) != 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		hashNumber(h, uint64(v.Kind()))
		id := uint64(0)
		if v.CanAddr() {
			id = records[pointerKey(v.Addr())]
		}
		hashNumber(h, id)
		switch v.Kind() {
		case reflect.Pointer:
			if v.IsNil() {
				hashNumber(h, 0)
				continue
			}
			key := pointerKey(v)
			hashNumber(h, records[key])
			if seen[key] {
				hashNumber(h, 0)
				continue
			}
			seen[key] = true
			hashNumber(h, 1)
			stack = append(stack, v.Elem())
		case reflect.Struct:
			hashNumber(h, uint64(v.NumField()))
			for i := v.NumField() - 1; i >= 0; i-- {
				stack = append(stack, v.Field(i))
			}
		case reflect.Map:
			if v.IsNil() {
				hashNumber(h, 0)
				continue
			}
			hashNumber(h, 1)
			hashNumber(h, uint64(v.Len()))
			keys := sortedSchemaMapKeys(v)
			for i := len(keys) - 1; i >= 0; i-- {
				stack = append(stack, v.MapIndex(keys[i]), keys[i])
			}
		case reflect.Slice:
			if v.IsNil() {
				hashNumber(h, 0)
				continue
			}
			hashNumber(h, 1)
			hashNumber(h, uint64(v.Len()))
			for i := v.Len() - 1; i >= 0; i-- {
				stack = append(stack, v.Index(i))
			}
		case reflect.String:
			hashText(h, v.String())
		case reflect.Bool:
			if v.Bool() {
				hashNumber(h, 1)
			} else {
				hashNumber(h, 0)
			}
		case reflect.Uint8, reflect.Uint64:
			hashNumber(h, v.Uint())
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (c *Compiled) recordPath(value any) string {
	return "/records/" + strconv.FormatUint(c.records[pointerKey(reflect.ValueOf(value))], 10)
}
func (c *Compiled) addType(b *compileBudget, id TypeID) error {
	if err := b.add(192+int64(len(id.path)), 1, 0); err != nil {
		return err
	}
	c.components = append(c.components, id)
	if id.simple != nil {
		if _, ok := c.simpleTypes[id.simple]; !ok {
			c.simpleTypes[id.simple] = id
		}
	}
	if id.complex != nil {
		if _, ok := c.complexTypes[id.complex]; !ok {
			c.complexTypes[id.complex] = id
		}
	}
	return nil
}
func (c *Compiled) addUse(b *compileBudget, use Use) error {
	if err := b.add(192+int64(len(use.path))+int64(len(use.ownerPath)), 1, 1); err != nil {
		return err
	}
	c.uses = append(c.uses, use)
	return nil
}
func (c *Compiled) elementUse(element *ElementSchema, owner, path string) Use {
	u := Use{schema: c, original: element, declaration: element, ownerPath: owner, path: path}
	// Preserve an unresolved reference as metadata. Resolution is deliberately
	// only a single lookup, matching the kernel's lazy declaration resolution.
	if element != nil && element.Reference.Local != "" {
		u.declaration = c.data.Elements[element.Reference]
	}
	return u
}

func (c *Compiled) index(b *compileBudget) error {
	// Builtins are immutable vocabulary, indexed even when an instance's
	// xsi:type is the first occurrence of that name. No scalar is assessed.
	for _, name := range strings.Fields("anyType anySimpleType string normalizedString token language Name NCName NMTOKEN ID IDREF ENTITY NMTOKENS IDREFS ENTITIES boolean decimal float double integer long int short byte nonPositiveInteger negativeInteger nonNegativeInteger positiveInteger unsignedLong unsignedInt unsignedShort unsignedByte base64Binary hexBinary anyURI QName NOTATION date dateTime duration gDay gMonth gMonthDay gYear gYearMonth time") {
		id := TypeID{schema: c, builtin: name, name: QName{Space: XSDNamespace, Local: name}, path: "/builtins/" + name}
		if err := c.addType(b, id); err != nil {
			return err
		}
		c.builtinTypes[name] = id
	}

	// Named owners precede anonymous graph records; map ordinals are exact,
	// deterministic structural paths without unbounded QName/path expansion.
	for i, key := range sortedSchemaMapKeys(reflect.ValueOf(c.data.Types)) {
		name := key.Interface().(QName)
		typ := c.data.Types[name]
		if typ == nil {
			continue
		}
		id := TypeID{schema: c, simple: typ.Simple, complex: typ.Complex, name: name, path: "/types/" + strconv.Itoa(i)}
		if err := c.addType(b, id); err != nil {
			return err
		}
		c.namedTypes[name] = id
	}
	// Record IDs already encode the canonical graph traversal. Sorting those
	// bounded records avoids any dependence on pointer-map iteration order.
	type entry struct {
		key graphPointer
		id  uint64
	}
	entries := make([]entry, 0, len(c.records))
	for key, id := range c.records {
		entries = append(entries, entry{key, id})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].id < entries[j].id })
	// Walk the owned graph to recover typed objects, using only pointer equality.
	objects := map[graphPointer]reflect.Value{}
	stack := []reflect.Value{reflect.ValueOf(c.data)}
	for len(stack) != 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch v.Kind() {
		case reflect.Pointer:
			if v.IsNil() {
				continue
			}
			key := pointerKey(v)
			if _, ok := objects[key]; ok {
				continue
			}
			objects[key] = v
			stack = append(stack, v.Elem())
		case reflect.Struct:
			for i := v.NumField() - 1; i >= 0; i-- {
				stack = append(stack, v.Field(i))
			}
		case reflect.Map:
			it := v.MapRange()
			for it.Next() {
				stack = append(stack, it.Value())
			}
		case reflect.Slice:
			for i := v.Len() - 1; i >= 0; i-- {
				stack = append(stack, v.Index(i))
			}
		}
	}
	for _, entry := range entries {
		value := objects[entry.key].Interface()
		id := TypeID{schema: c, path: "/records/" + strconv.FormatUint(entry.id, 10)}
		switch value := value.(type) {
		case *SimpleSchema:
			if _, ok := c.simpleTypes[value]; ok {
				continue
			}
			id.simple = value
		case *ComplexSchema:
			if _, ok := c.complexTypes[value]; ok {
				continue
			}
			id.complex = value
		default:
			continue
		}
		if err := c.addType(b, id); err != nil {
			return err
		}
	}
	for i, key := range sortedSchemaMapKeys(reflect.ValueOf(c.data.Elements)) {
		name := key.Interface().(QName)
		element := c.data.Elements[name]
		if element == nil {
			continue
		}
		path := "/elements/" + strconv.Itoa(i)
		use := c.elementUse(element, path, path)
		if err := c.addUse(b, use); err != nil {
			return err
		}
		c.roots[name] = use
	}
	for _, owner := range c.components {
		if owner.complex == nil {
			continue
		}
		if err := c.indexComplex(b, owner); err != nil {
			return err
		}
	}
	return nil
}
func (c *Compiled) indexComplex(b *compileBudget, owner TypeID) error {
	for i := range owner.complex.Attributes {
		use := Use{schema: c, attribute: &owner.complex.Attributes[i], ownerPath: owner.path, path: owner.path + "/attributes/" + strconv.Itoa(i)}
		if err := c.addUse(b, use); err != nil {
			return err
		}
		c.attributeUses[attributeUseKey{owner, i}] = use
	}
	type task struct {
		particle *ParticleSchema
		path     string
	}
	stack := []task{{owner.complex.Particle, owner.path + "/particle"}}
	seen := make(map[*ParticleSchema]bool)
	for len(stack) != 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		p := current.particle
		if p == nil {
			continue
		}
		if err := b.add(64+int64(len(current.path)), 0, 1); err != nil {
			return err
		}
		if p.Element != nil {
			use := c.elementUse(p.Element, owner.path, current.path+"/element")
			if err := c.addUse(b, use); err != nil {
				return err
			}
			key := particleUseKey{owner, p}
			c.particleUses[key] = append(c.particleUses[key], use)
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		if err := b.collection(len(p.Children), 64); err != nil {
			return err
		}
		for i := len(p.Children) - 1; i >= 0; i-- {
			stack = append(stack, task{p.Children[i], owner.path + c.recordPath(p) + "/children/" + strconv.Itoa(i)})
		}
	}
	return nil
}
