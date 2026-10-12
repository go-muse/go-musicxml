package musicxml

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-muse/go-musicxml/validation"
)

// The sink is synchronous and copies every retained fact. Session owns actual
// occurrence/depth/event charges; this walker owns traversal work and scratch.
type modelFactSink interface {
	Open(context.Context, validation.NodeHeader) error
	Attribute(context.Context, validation.AttributeFact) error
	CompleteAttributes(context.Context, validation.NodeID, validation.Evidence[bool]) error
	ModelRepresentation(context.Context, validation.ModelRepresentationFact) error
	Close(context.Context, validation.NodeEnd) error
}

type modelWalkOptions struct{ LegacyPaths bool }
type modelWalkStatus struct{ HasProblems, HasCycle, DepthExceeded bool }
type modelPointer struct {
	typ     reflect.Type
	address uintptr
}
type modelWalkFrame struct {
	id           validation.NodeID
	value        reflect.Value
	descriptor   *modelDescriptor
	kind         modelScalarKind
	path         string
	goPath       string
	pointer      modelPointer
	field, index int
	names        map[modelName]uint64
	problems     []validation.ModelProblem
	hole         bool
	retained     int64
}
type modelWalker struct {
	ctx            context.Context
	sink           modelFactSink
	account        *validation.Account
	documentID     string
	options        modelWalkOptions
	status         modelWalkStatus
	stack          []*modelWalkFrame
	active         map[modelPointer]bool
	activeCapacity int
	next           validation.NodeID
	retained       int64
}

func walkModel(ctx context.Context, document Document, sink modelFactSink, account *validation.Account, documentID string, options modelWalkOptions) (modelWalkStatus, error) {
	var status modelWalkStatus
	if ctx == nil {
		return status, &validation.ConfigurationError{Field: "context", Reason: "must not be nil"}
	}
	if err := ctx.Err(); err != nil {
		return status, err
	}
	if sink == nil {
		return status, modelMetadataError("missing fact sink")
	}
	if err := account.Charge(validation.BudgetEvents, 0); err != nil {
		return status, err
	}
	var name string
	switch document.(type) {
	case *ScorePartwise:
		name = "ScorePartwise"
	case *ScoreTimewise:
		name = "ScoreTimewise"
	case *OpusDocument:
		name = "OpusDocument"
	case nil:
		return status, ErrNilDocument
	default:
		return status, ErrUnsupportedDocument
	}
	value := reflect.ValueOf(document)
	if value.IsNil() {
		return status, ErrNilDocument
	}
	descriptor := modelDescriptors[name]
	if descriptor == nil || descriptor.Root.Local == "" {
		return status, modelMetadataError("missing concrete root")
	}
	w := modelWalker{ctx: ctx, sink: sink, account: account, documentID: documentID, options: options}
	if err := w.retain(nil, 256); err != nil {
		return status, err
	}
	defer func() { _ = account.Release(validation.BudgetBufferedBytes, w.retained) }()
	path := name
	if options.LegacyPaths {
		path = "/" + descriptor.Root.Local
	}
	err := w.push(value, descriptor, 0, descriptor.Root, path, name, nil)
	if err == nil {
		err = w.run()
	}
	return w.status, err
}

func (w *modelWalker) step() error {
	if err := w.ctx.Err(); err != nil {
		return err
	}
	return w.account.Charge(validation.BudgetEvents, 1)
}
func (w *modelWalker) retain(frame *modelWalkFrame, amount int64) error {
	if _, err := validation.AllocationSize(amount); err != nil {
		return err
	}
	if err := w.account.Charge(validation.BudgetBufferedBytes, amount); err != nil {
		return err
	}
	w.retained += amount
	if frame != nil {
		frame.retained += amount
	}
	return nil
}
func (w *modelWalker) release(frame *modelWalkFrame, amount int64) {
	_ = w.account.Release(validation.BudgetBufferedBytes, amount)
	w.retained -= amount
	if frame != nil {
		frame.retained -= amount
	}
}
func (w *modelWalker) location(path string) validation.Location {
	return validation.Location{Target: validation.TargetModel, DocumentID: w.documentID, Path: path}
}
func modelQName(name modelName) validation.QName {
	return validation.QName{Space: name.Space, Local: name.Local}
}
func modelComplete() validation.Evidence[bool] {
	return validation.Evidence[bool]{State: validation.EvidenceKnown, Value: true}
}

func (w *modelWalker) push(value reflect.Value, descriptor *modelDescriptor, kind modelScalarKind, name modelName, path, goPath string, parent *modelWalkFrame) error {
	if err := w.step(); err != nil {
		return err
	}
	var pointer modelPointer
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return modelMetadataError("nil represented occurrence")
		}
		pointer = modelPointer{value.Type(), value.Pointer()}
		if w.active[pointer] {
			if parent == nil {
				return modelMetadataError("cyclic root")
			}
			w.status.HasCycle = true
			parent.hole = true
			return w.problem(parent, validation.ModelAxisChildren, "model-cycle", modelName{}, path, goPath, "acyclic active path", "pointer already active")
		}
		value = value.Elem()
	}
	if descriptor != nil {
		if value.Kind() != reflect.Struct {
			return modelMetadataError("complex representation is not a struct")
		}
		mapped := len(descriptor.Attributes) + len(descriptor.Elements)
		if descriptor.Scalar != nil {
			mapped++
		}
		if value.NumField() != descriptor.GoFields || mapped != descriptor.MappedFields {
			return modelMetadataError("incomplete complex field map")
		}
		if name := value.Type().Name(); name != "" && name != descriptor.Name {
			return modelMetadataError("complex type identity mismatch")
		}
	} else if kind == 0 {
		return modelMetadataError("missing occurrence descriptor")
	}
	if len(w.stack) == cap(w.stack) {
		if int64(cap(w.stack)) > math.MaxInt64/16 {
			return &validation.ResourceError{Kind: validation.BudgetBufferedBytes, Limit: math.MaxInt64}
		}
		capacity := max(8, 2*int64(cap(w.stack)))
		size, err := validation.AllocationSize(capacity * 8)
		if err != nil {
			return err
		}
		if err = w.retain(nil, int64(size)); err != nil {
			return err
		}
		next := make([]*modelWalkFrame, len(w.stack), size/8)
		if err = w.account.Charge(validation.BudgetEvents, int64(len(w.stack))); err != nil {
			return err
		}
		copy(next, w.stack)
		old := int64(cap(w.stack)) * 8
		w.stack = next
		w.release(nil, old)
	}
	if pointer.address != 0 && len(w.active) == w.activeCapacity {
		if int64(w.activeCapacity) > math.MaxInt64/256 {
			return &validation.ResourceError{Kind: validation.BudgetBufferedBytes, Limit: math.MaxInt64}
		}
		capacity := max(8, 2*int64(w.activeCapacity))
		size, err := validation.AllocationSize(capacity)
		if err != nil {
			return err
		}
		if err = w.retain(nil, capacity*128); err != nil {
			return err
		}
		next := make(map[modelPointer]bool, size)
		for key := range w.active {
			if err = w.step(); err != nil {
				return err
			}
			next[key] = true
		}
		old := int64(w.activeCapacity) * 128
		w.active = next
		w.activeCapacity = size
		w.release(nil, old)
	}
	frameBytes := int64(512 + len(path))
	if w.options.LegacyPaths {
		frameBytes += int64(len(goPath))
	}
	if err := w.retain(nil, frameBytes); err != nil {
		return err
	}
	w.next++
	frame := &modelWalkFrame{id: w.next, value: value, descriptor: descriptor, kind: kind, path: strings.Clone(path), pointer: pointer, retained: frameBytes}
	frame.goPath = frame.path
	if w.options.LegacyPaths {
		frame.goPath = strings.Clone(goPath)
	}
	w.stack = append(w.stack, frame)
	if pointer.address != 0 {
		w.active[pointer] = true
	}
	var parentID validation.NodeID
	if parent != nil {
		parentID = parent.id
	}
	header := validation.NodeHeader{ID: frame.id, Parent: parentID, Name: modelQName(name), Location: w.location(frame.path), Type: validation.Evidence[validation.ComponentID]{State: validation.EvidenceUnsupported, Reason: "model-schema-witness-required"}, Declaration: validation.Evidence[validation.DeclarationID]{State: validation.EvidenceUnsupported, Reason: "model-schema-witness-required"}}
	if err := w.sink.Open(w.ctx, header); err != nil {
		var resource *validation.ResourceError
		if errors.As(err, &resource) && resource.Kind == validation.BudgetDepth {
			w.status.DepthExceeded = true
		}
		return err
	}
	if descriptor != nil {
		for _, field := range descriptor.Attributes {
			if err := w.attribute(frame, field); err != nil {
				return err
			}
		}
	}
	return w.sink.CompleteAttributes(w.ctx, frame.id, modelComplete())
}

// field reads only generated indexes, checking the named endpoint before use.
// No XML tags, method sets, arbitrary struct enumeration or callbacks are read.
func (w *modelWalker) field(value reflect.Value, field modelField) (reflect.Value, error) {
	if err := w.step(); err != nil {
		return reflect.Value{}, err
	}
	for n, index := range field.Index {
		if err := w.step(); err != nil {
			return reflect.Value{}, err
		}
		if value.Kind() != reflect.Struct || index < 0 || index >= value.NumField() {
			return reflect.Value{}, modelMetadataError("field index mismatch")
		}
		sf := value.Type().Field(index)
		if n == len(field.Index)-1 {
			expected := field.Name
			if at := strings.LastIndexByte(expected, '.'); at >= 0 {
				expected = expected[at+1:]
			}
			if sf.Name != expected || sf.PkgPath != "" {
				return reflect.Value{}, modelMetadataError("field identity mismatch")
			}
		}
		value = value.Field(index)
	}
	if len(field.Index) == 0 {
		return reflect.Value{}, modelMetadataError("empty field index")
	}
	return value, nil
}

func (w *modelWalker) attribute(frame *modelWalkFrame, field modelField) error {
	value, err := w.field(frame.value, field)
	if err != nil {
		return err
	}
	if field.Optional {
		if value.Kind() != reflect.Pointer {
			return modelMetadataError("optional attribute is not pointer")
		}
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	scalar, err := readModelScalar(w.ctx, w.account, value, field.Kind)
	if err != nil {
		return err
	}
	defer w.account.Release(validation.BudgetBufferedBytes, scalar.retained)
	path, bytes, err := w.path(frame, field.Name, -1, "", field.XML, true)
	if err != nil {
		return err
	}
	defer w.release(nil, bytes)
	if scalar.problem != "" {
		goPath := path
		if w.options.LegacyPaths {
			var retained int64
			goPath, retained, err = w.goFieldPath(frame, field.Name, -1, "")
			if err != nil {
				return err
			}
			defer w.release(nil, retained)
		}
		if err = w.problem(frame, validation.ModelAxisAttribute, scalar.problem, field.XML, path, goPath, "representable current value", "unrepresentable current value"); err != nil {
			return err
		}
	}
	return w.sink.Attribute(w.ctx, validation.AttributeFact{Owner: frame.id, Name: modelQName(field.XML), Declaration: validation.Evidence[validation.DeclarationID]{State: validation.EvidenceUnsupported, Reason: "model-schema-witness-required"}, Presence: modelComplete(), Scalar: scalar.scalar, Location: w.location(path)})
}

func (w *modelWalker) run() error {
	for len(w.stack) > 0 {
		if err := w.step(); err != nil {
			return err
		}
		frame := w.stack[len(w.stack)-1]
		if frame.descriptor == nil || frame.field >= len(frame.descriptor.Elements) {
			if err := w.close(frame); err != nil {
				return err
			}
			continue
		}
		field := frame.descriptor.Elements[frame.field]
		value, err := w.field(frame.value, field)
		if err != nil {
			return err
		}
		index := -1
		if field.Repeated {
			if value.Kind() != reflect.Slice {
				return modelMetadataError("repeated field is not slice")
			}
			if frame.index >= value.Len() {
				frame.field++
				frame.index = 0
				continue
			}
			if err = w.step(); err != nil {
				return err
			}
			index = frame.index
			frame.index++
			value = value.Index(index)
		} else {
			frame.field++
		}
		variant := ""
		if len(field.Variants) > 0 {
			if value.Kind() != reflect.Struct {
				return modelMetadataError("choice entry is not struct")
			}
			count := 0
			var selected modelField
			var selectedValue reflect.Value
			for _, candidate := range field.Variants {
				candidateValue, e := w.field(value, candidate)
				if e != nil {
					return e
				}
				if candidateValue.Kind() != reflect.Pointer {
					return modelMetadataError("choice variant is not pointer")
				}
				if !candidateValue.IsNil() {
					count++
					selected = candidate
					selectedValue = candidateValue
				}
			}
			if count != 1 {
				path, bytes, e := w.path(frame, field.Name, index, "", modelName{}, false)
				if e != nil {
					return e
				}
				frame.hole = true
				goPath := path
				var goBytes int64
				if w.options.LegacyPaths {
					goPath, goBytes, e = w.goFieldPath(frame, field.Name, index, "")
					if e != nil {
						w.release(nil, bytes)
						return e
					}
				}
				e = w.problem(frame, validation.ModelAxisChildren, "model-choice-cardinality", modelName{}, path, goPath, "exactly one selected variant", strconv.Itoa(count)+" selected variants")
				w.release(nil, bytes+goBytes)
				if e != nil {
					return e
				}
				continue
			}
			variant = selected.Name
			selected.Name = field.Name
			field = selected
			value = selectedValue
		} else if field.Optional {
			if value.Kind() != reflect.Pointer {
				return modelMetadataError("optional element is not pointer")
			}
			if value.IsNil() {
				continue
			}
		}
		var descriptor *modelDescriptor
		if field.Child != "" {
			descriptor = modelDescriptors[field.Child]
			if descriptor == nil {
				return modelMetadataError("missing child descriptor")
			}
		}
		path, bytes, err := w.path(frame, field.Name, index, variant, field.XML, false)
		if err != nil {
			return err
		}
		goPath := path
		var goBytes int64
		if w.options.LegacyPaths {
			goPath, goBytes, err = w.goFieldPath(frame, field.Name, index, variant)
			if err != nil {
				w.release(nil, bytes)
				return err
			}
		}
		err = w.push(value, descriptor, field.Kind, field.XML, path, goPath, frame)
		w.release(nil, bytes+goBytes)
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *modelWalker) close(frame *modelWalkFrame) error {
	scalar := modelScalarResult{scalar: absentModelScalar()}
	var err error
	if frame.descriptor == nil {
		scalar, err = readModelScalar(w.ctx, w.account, frame.value, frame.kind)
	} else if field := frame.descriptor.Scalar; field != nil {
		var value reflect.Value
		value, err = w.field(frame.value, *field)
		if err == nil {
			scalar, err = readModelScalar(w.ctx, w.account, value, field.Kind)
		}
	}
	if err != nil {
		return err
	}
	defer w.account.Release(validation.BudgetBufferedBytes, scalar.retained)
	if scalar.problem != "" {
		path := frame.path
		if frame.descriptor != nil && !w.options.LegacyPaths {
			var retained int64
			path, retained, err = w.path(frame, frame.descriptor.Scalar.Name, -1, "", modelName{}, false)
			if err != nil {
				return err
			}
			defer w.release(nil, retained)
		}
		goPath := frame.goPath
		if w.options.LegacyPaths && frame.descriptor != nil {
			var retained int64
			goPath, retained, err = w.goFieldPath(frame, frame.descriptor.Scalar.Name, -1, "")
			if err != nil {
				return err
			}
			defer w.release(nil, retained)
		}
		if err = w.problem(frame, validation.ModelAxisScalar, scalar.problem, modelName{}, path, goPath, "representable current value", "unrepresentable current value"); err != nil {
			return err
		}
	}
	children := modelComplete()
	order := validation.Evidence[validation.OrderKind]{State: validation.EvidenceKnown, Value: validation.OrderModelFields}
	if frame.descriptor != nil && len(frame.descriptor.Elements) == 1 && len(frame.descriptor.Elements[0].Variants) > 0 {
		order.Value = validation.OrderModelContent
	}
	if frame.hole {
		children = validation.Evidence[bool]{State: validation.EvidenceUnsupported, Reason: "model-child-sequence-hole"}
		order = validation.Evidence[validation.OrderKind]{State: validation.EvidenceUnsupported, Reason: "model-child-sequence-hole"}
	}
	if err = w.sink.ModelRepresentation(w.ctx, validation.ModelRepresentationFact{Owner: frame.id, Complete: modelComplete(), Problems: frame.problems}); err != nil {
		return err
	}
	if err = w.sink.Close(w.ctx, validation.NodeEnd{ID: frame.id, Scalar: scalar.scalar, ChildrenComplete: children, AttributesComplete: modelComplete(), Order: order}); err != nil {
		return err
	}
	if frame.pointer.address != 0 {
		delete(w.active, frame.pointer)
	}
	w.stack[len(w.stack)-1] = nil
	w.stack = w.stack[:len(w.stack)-1]
	w.release(nil, frame.retained)
	return nil
}

// One representative per axis/code/attribute is enough for the once-per-node
// integrity decision; repeated bad choice entries never grow an unbounded list.
func (w *modelWalker) problem(frame *modelWalkFrame, axis validation.ModelAxis, code validation.Code, attribute modelName, path, goPath, expected, actual string) error {
	w.status.HasProblems = true
	for _, problem := range frame.problems {
		if err := w.step(); err != nil {
			return err
		}
		if problem.Axis == axis && problem.Code == code && problem.Attribute == modelQName(attribute) {
			return nil
		}
	}
	extra := 0
	if w.options.LegacyPaths {
		extra = len(goPath) + 4
	}
	if err := w.retain(frame, 512+int64(len(path)+len(expected)+len(actual)+extra)); err != nil {
		return err
	}
	if w.options.LegacyPaths {
		actual += " at " + goPath
	}
	// Explicit exact growth accounts the transient replacement before releasing
	// the previous backing array. Strings above are retained with the frame.
	count, err := validation.AllocationSize(int64(len(frame.problems)+1) * 256)
	if err != nil {
		return err
	}
	if err = w.retain(frame, int64(count)); err != nil {
		return err
	}
	next := make([]validation.ModelProblem, len(frame.problems)+1)
	copy(next, frame.problems)
	old := int64(len(frame.problems)) * 256
	next[len(frame.problems)] = validation.ModelProblem{Code: code, Axis: axis, Attribute: modelQName(attribute), Location: w.location(strings.Clone(path)), Expected: expected, Actual: actual}
	frame.problems = next
	w.release(frame, old)
	return nil
}

// Paths are display hints; NodeID is the occurrence identity. At large depth,
// anchor the remaining field/index path at the exact parent occurrence instead
// of allocating quadratic, potentially unbounded ancestor strings.
func (w *modelWalker) path(frame *modelWalkFrame, field string, index int, variant string, name modelName, attribute bool) (string, int64, error) {
	if err := w.step(); err != nil {
		return "", 0, err
	}
	// Four copies cover bounded intermediate concatenations as well as the
	// returned path during the synchronous sink call.
	maximum := 4 * int64(len(frame.path)+len(field)+len(variant)+len(name.Space)+len(name.Local)+128)
	if err := w.retain(nil, maximum); err != nil {
		return "", 0, err
	}
	prefix := frame.path
	if len(prefix) > 2048 {
		prefix = "node[" + strconv.FormatUint(uint64(frame.id), 10) + "]"
	}
	var result string
	if w.options.LegacyPaths && name.Local != "" {
		count := uint64(1)
		if !attribute {
			count = frame.names[name] + 1
			if count == 1 {
				if err := w.retain(frame, 128); err != nil {
					w.release(nil, maximum)
					return "", 0, err
				}
				if frame.names == nil {
					frame.names = make(map[modelName]uint64)
				}
			}
			frame.names[name] = count
		}
		label := name.Local
		if name.Space != "" {
			label = "{" + name.Space + "}" + label
		}
		if attribute {
			label = "@" + label
		}
		if count > 1 {
			label += "[" + strconv.FormatUint(count, 10) + "]"
		}
		result = prefix + "/" + label
	} else {
		result = prefix + "." + field
		if index >= 0 {
			result += "[" + strconv.Itoa(index) + "]"
		}
		if variant != "" {
			result += "." + variant
		}
	}
	return result, maximum, nil
}

// goFieldPath preserves representation diagnostics when the legacy facade uses
// XML-structural display locations. It shares the same bounded traversal and
// parent occurrence anchors; it never rereads the graph or invokes transport.
func (w *modelWalker) goFieldPath(frame *modelWalkFrame, field string, index int, variant string) (string, int64, error) {
	if err := w.step(); err != nil {
		return "", 0, err
	}
	maximum := 4 * int64(len(frame.goPath)+len(field)+len(variant)+128)
	if err := w.retain(nil, maximum); err != nil {
		return "", 0, err
	}
	prefix := frame.goPath
	if len(prefix) > 2048 {
		prefix = "node[" + strconv.FormatUint(uint64(frame.id), 10) + "]"
	}
	path := prefix + "." + field
	if index >= 0 {
		path += "[" + strconv.Itoa(index) + "]"
	}
	if variant != "" {
		path += "." + variant
	}
	return path, maximum, nil
}
