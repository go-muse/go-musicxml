package musicxml

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"sort"
	"strconv"
	"strings"

	"github.com/go-muse/go-musicxml/validation"
)

const sourceDocumentID = "source"
const sourceXMLNSNamespace = "http://www.w3.org/2000/xmlns/"

// sourceFactSink is synchronous: each method copies everything it retains.
// The production implementation is a validation.Session.
type sourceFactSink interface {
	Open(context.Context, validation.NodeHeader) error
	Attribute(context.Context, validation.AttributeFact) error
	CompleteAttributes(context.Context, validation.NodeID, validation.Evidence[bool]) error
	Close(context.Context, validation.NodeEnd) error
}

type sourceNamespaceUndo struct {
	prefix, previous string
	existed          bool
}
type sourceObserverFrame struct {
	id          validation.NodeID
	path        string
	pathOmitted bool
	names       map[validation.QName]uint64
	text        sourceScalarBuffer
	undo        []sourceNamespaceUndo
	retained    int64
}

// sourceScalarBuffer uses an explicitly sized backing array. strings.Builder's
// implicit growth would allocate capacity that a length-only charge never saw.
type sourceScalarBuffer struct{ data []byte }

func (b *sourceScalarBuffer) Len() int       { return len(b.data) }
func (b *sourceScalarBuffer) Cap() int       { return cap(b.data) }
func (b *sourceScalarBuffer) String() string { return string(b.data) }

// sourceFactObserver owns only the open-element scalar/scope stack. The session
// owns the retained document facts and all node, attribute and depth charges.
// Neither schema defaults nor model-derived values are source evidence.
type sourceFactObserver struct {
	ctx                     context.Context
	sink                    sourceFactSink
	account                 *validation.Account
	documentID, entry       string
	stack                   []*sourceObserverFrame
	bindings                map[string]string
	next                    validation.NodeID
	retained                int64
	dtdUnavailable          bool
	last                    validation.Location
	lastRetained            int64
	origins                 *sourceOriginObserver
	captureAttributeLexical bool
	sourceDeclarations      bool
	sourceDTDIDs            bool
}

func newSourceFactObserver(ctx context.Context, sink sourceFactSink, account *validation.Account) (*sourceFactObserver, error) {
	return newSourceFactObserverWithIdentity(ctx, sink, account, sourceDocumentID, "")
}

func newSourceFactObserverWithIdentity(ctx context.Context, sink sourceFactSink, account *validation.Account, documentID, entry string) (*sourceFactObserver, error) {
	const fixed = 256
	if err := account.Charge(validation.BudgetBufferedBytes, fixed); err != nil {
		return nil, err
	}
	return &sourceFactObserver{ctx: ctx, sink: sink, account: account, documentID: documentID, entry: entry, bindings: map[string]string{"xml": "http://www.w3.org/XML/1998/namespace"}, retained: fixed}, nil
}

func (o *sourceFactObserver) observe(token xml.Token, meta sourceXMLTokenMeta) error {
	return o.observeSource(token, meta, nil)
}

// The scanner is borrowed only for this call, never retained in observer state.
func (o *sourceFactObserver) observeSource(token xml.Token, meta sourceXMLTokenMeta, scanner *sourceXMLTokenReader) error {
	if err := o.ctx.Err(); err != nil {
		return err
	}
	switch value := token.(type) {
	case xml.StartElement:
		return o.open(value, meta, scanner)
	case xml.CharData:
		if len(o.stack) == 0 {
			return nil
		}
		frame := o.stack[len(o.stack)-1]
		size := int64(frame.text.Len()) + int64(len(value))
		if err := o.account.Check(validation.BudgetScalarBytes, size); err != nil {
			return err
		}
		if size > int64(frame.text.Cap()) {
			capacity := max(size, 2*int64(frame.text.Cap()), 32)
			capacity = min(capacity, o.account.Limits().MaxScalarBytes)
			count, err := validation.AllocationSize(capacity)
			if err != nil {
				return err
			}
			if err := o.retain(frame, capacity); err != nil {
				return err
			}
			next := make([]byte, frame.text.Len(), count)
			copy(next, frame.text.data)
			old := int64(frame.text.Cap())
			frame.text.data = next
			if err := o.account.Release(validation.BudgetBufferedBytes, old); err != nil {
				return err
			}
			frame.retained -= old
			o.retained -= old
		}
		frame.text.data = append(frame.text.data, value...)
	case xml.EndElement:
		if len(o.stack) == 0 {
			return &validation.SessionError{Operation: "source close", Reason: "no open source occurrence"}
		}
		frame := o.stack[len(o.stack)-1]
		complete := validation.Evidence[bool]{State: validation.EvidenceKnown, Value: true}
		attrs := complete
		if o.dtdUnavailable {
			attrs = validation.Evidence[bool]{State: validation.EvidenceUnsupported, Reason: "dtd-infoset-unavailable"}
		}
		// The immutable scalar string is an additional copy during the synchronous
		// sink call; retain it before conversion, then release with this frame.
		if err := o.retain(frame, int64(frame.text.Len())); err != nil {
			return err
		}
		err := o.sink.Close(o.ctx, validation.NodeEnd{ID: frame.id, Scalar: sourceLexeme(frame.text.String()), ChildrenComplete: complete, AttributesComplete: attrs, Order: validation.Evidence[validation.OrderKind]{State: validation.EvidenceKnown, Value: validation.OrderSource}})
		if err != nil {
			return err
		}
		o.pop()
	}
	return nil
}

func (o *sourceFactObserver) open(value xml.StartElement, meta sourceXMLTokenMeta, scanner *sourceXMLTokenReader) error {
	if len(meta.Attributes) != len(value.Attr) {
		return &validation.SessionError{Operation: "source metadata", Reason: "attribute metadata does not match source token"}
	}
	name := validation.QName{Space: value.Name.Space, Local: value.Name.Local}
	if err := sourceNameCapacity(name); err != nil {
		return err
	}
	// Reject all fixed fact identity limitations before admitting this element.
	for i, attr := range value.Attr {
		if err := sourceNameCapacity(sourceAttributeName(attr, meta.Attributes[i].NamespaceDeclaration)); err != nil {
			return err
		}
	}
	var parent validation.NodeID
	var prefix string
	var omitted bool
	index := uint64(1)
	if len(o.stack) > 0 {
		frame := o.stack[len(o.stack)-1]
		parent, prefix, omitted = frame.id, frame.path, frame.pathOmitted
		index = frame.names[name] + 1
		if index == 1 {
			if err := o.retain(frame, 128+int64(len(name.Space))+int64(len(name.Local))); err != nil {
				return err
			}
			if frame.names == nil {
				frame.names = make(map[validation.QName]uint64)
			}
		}
		frame.names[name] = index
	}
	// Paths are human-readable hints, not occurrence identities. A long legal
	// name/nesting may exceed the fact vocabulary's display-path cap; omit the
	// path in that case rather than invent a truncated path or reject valid XML.
	labelBytes := len(name.Local)
	if name.Space != "" {
		labelBytes += len(name.Space) + 2
	}
	suffix := ""
	if index > 1 {
		suffix = "[" + strconv.FormatUint(index, 10) + "]"
	}
	pathBytes := len(prefix) + 1 + labelBytes + len(suffix)
	if omitted || pathBytes > 16384 {
		omitted = true
		pathBytes = 0
	}
	frameBytes := int64(192 + pathBytes)
	if err := o.account.Charge(validation.BudgetBufferedBytes, frameBytes); err != nil {
		return err
	}
	var path string
	if !omitted {
		path = prefix + "/" + sourceDisplayName(name) + suffix
	}
	frame := &sourceObserverFrame{path: path, pathOmitted: omitted, retained: frameBytes}
	o.retained += frameBytes
	o.next++
	frame.id = o.next
	if len(o.stack) == cap(o.stack) {
		capacity := max(1, 2*cap(o.stack))
		owned := int64(capacity) * 8
		if err := o.account.Charge(validation.BudgetBufferedBytes, owned); err != nil {
			return err
		}
		o.retained += owned
		next := make([]*sourceObserverFrame, len(o.stack), capacity)
		copy(next, o.stack)
		old := int64(cap(o.stack)) * 8
		o.stack = next
		if err := o.account.Release(validation.BudgetBufferedBytes, old); err != nil {
			return err
		}
		o.retained -= old
	}
	o.stack = append(o.stack, frame)
	for i, attr := range value.Attr {
		if err := o.ctx.Err(); err != nil {
			return err
		}
		if !meta.Attributes[i].NamespaceDeclaration {
			continue
		}
		nsPrefix := ""
		if attr.Name.Space != "" {
			nsPrefix = attr.Name.Local
		}
		if len(nsPrefix) > 1024 || len(attr.Value) > 4096 {
			return &validation.IncompleteError{Reason: "source-fact-identity-limit"}
		}
		if err := o.retain(frame, 80+int64(len(nsPrefix))+int64(len(attr.Value))); err != nil {
			return err
		}
		previous, existed := o.bindings[nsPrefix]
		if !existed && len(o.bindings) >= 65536 {
			return &validation.IncompleteError{Reason: "source-fact-identity-limit"}
		}
		if !existed {
			// Map bucket capacity survives delete at scope exit. A conservative
			// insertion allowance belongs to the observer until Close, not to the
			// frame. Repeated insertions may over-reserve, but never refund live
			// backing capacity or require a second unmetered seen-key map.
			const capacityAllowance = 128
			if err := o.account.Charge(validation.BudgetBufferedBytes, capacityAllowance); err != nil {
				return err
			}
			o.retained += capacityAllowance
		}
		ownedPrefix := strings.Clone(nsPrefix)
		frame.undo = append(frame.undo, sourceNamespaceUndo{prefix: ownedPrefix, previous: previous, existed: existed})
		o.bindings[ownedPrefix] = strings.Clone(attr.Value)
	}
	if len(o.bindings) > 65536 {
		return &validation.IncompleteError{Reason: "source-fact-identity-limit"}
	}
	if err := o.account.Charge(validation.BudgetRuleSteps, int64(len(value.Attr))+int64(len(o.bindings))*int64(1+bits.Len(uint(len(o.bindings))))); err != nil {
		return err
	}
	temporary := int64(len(o.bindings)) * 48
	if err := o.account.Charge(validation.BudgetBufferedBytes, temporary); err != nil {
		return err
	}
	defer o.account.Release(validation.BudgetBufferedBytes, temporary)
	bindings := make([]validation.Binding, 0, len(o.bindings))
	for prefix, uri := range o.bindings {
		bindings = append(bindings, validation.Binding{Prefix: prefix, URI: uri})
	}
	sort.Slice(bindings, func(i, j int) bool { return bindings[i].Prefix < bindings[j].Prefix })
	state, reason := validation.EvidenceUnknown, validation.Code("schema-binding-pending")
	if o.dtdUnavailable {
		state, reason = validation.EvidenceUnsupported, "dtd-infoset-unavailable"
	}
	location := o.location(path, meta.Span.Start)
	if meta.Origin != nil && !meta.Origin.PhysicalAvailable {
		location.Position = nil
	}
	if err := o.rememberLocation(location); err != nil {
		return err
	}
	if err := o.sink.Open(o.ctx, validation.NodeHeader{ID: frame.id, Parent: parent, Name: name, Type: validation.Evidence[validation.ComponentID]{State: state, Reason: reason}, Declaration: validation.Evidence[validation.DeclarationID]{State: state, Reason: reason}, Location: location, Bindings: bindings}); err != nil {
		return err
	}
	if parent == 0 && o.sourceDeclarations {
		if err := o.publishSourceEntityDeclarations(scanner); err != nil {
			return err
		}
	}
	if parent == 0 {
		if err := o.publishSourceDTDIDDomain(scanner); err != nil {
			return err
		}
	}
	if err := o.publishSourceOrigin(frame, nil, name, location, meta.Origin); err != nil {
		return err
	}
	for i, attr := range value.Attr {
		if err := o.attribute(frame, attr, meta.Attributes[i], state, reason); err != nil {
			return err
		}
	}
	complete := validation.Evidence[bool]{State: validation.EvidenceKnown, Value: true}
	if o.dtdUnavailable {
		complete = validation.Evidence[bool]{State: validation.EvidenceUnsupported, Reason: "dtd-infoset-unavailable"}
	}
	return o.sink.CompleteAttributes(o.ctx, frame.id, complete)
}

func (o *sourceFactObserver) attribute(frame *sourceObserverFrame, attr xml.Attr, meta sourceXMLAttribute, state validation.EvidenceState, reason validation.Code) error {
	name := sourceAttributeName(attr, meta.NamespaceDeclaration)
	if err := sourceNameCapacity(name); err != nil {
		return err
	}
	labelBytes := len(name.Local)
	if name.Space != "" {
		labelBytes += len(name.Space) + 2
	}
	pathBytes := len(frame.path) + 2 + labelBytes
	if frame.pathOmitted || pathBytes > 16384 {
		pathBytes = 0
	}
	temporary := int64(160 + pathBytes)
	if err := o.account.Charge(validation.BudgetBufferedBytes, temporary); err != nil {
		return err
	}
	defer o.account.Release(validation.BudgetBufferedBytes, temporary)
	path := ""
	if pathBytes != 0 {
		path = frame.path + "/@" + sourceDisplayName(name)
	}
	scalar := sourceLexeme(attr.Value)
	if o.dtdUnavailable && !meta.NamespaceDeclaration && !(o.sourceDTDIDs && meta.DTDID != nil) {
		scalar.Lexeme = validation.Evidence[string]{State: validation.EvidenceUnsupported, Reason: "dtd-infoset-unavailable"}
	}
	location := o.location(path, meta.NameSpan.Start)
	if meta.Origin != nil && !meta.Origin.PhysicalAvailable {
		location.Position = nil
	}
	if err := o.sink.Attribute(o.ctx, validation.AttributeFact{Owner: frame.id, Name: name, Declaration: validation.Evidence[validation.DeclarationID]{State: state, Reason: reason}, Presence: validation.Evidence[bool]{State: validation.EvidenceKnown, Value: true}, Scalar: scalar, Location: location}); err != nil {
		return err
	}
	if err := o.publishSourceOrigin(frame, &name, validation.QName{}, location, meta.Origin); err != nil {
		return err
	}
	if err := o.publishSourceDTDIDAttribute(frame, name, scalar.Lexeme.Value, location, meta); err != nil {
		return err
	}
	return o.publishSourceAttributeLexical(frame, name, meta)
}
func sourceDisplayName(name validation.QName) string {
	if name.Space == "" {
		return name.Local
	}
	return "{" + name.Space + "}" + name.Local
}

// Namespace-declaration origin comes from the lexical parser, not from a
// prefix-looking namespace URI. Canonical XMLNS names keep physical declarations
// observable without conflating ordinary attributes whose URI is "xmlns".
func sourceAttributeName(attr xml.Attr, declaration bool) validation.QName {
	if !declaration {
		return validation.QName{Space: attr.Name.Space, Local: attr.Name.Local}
	}
	local := attr.Name.Local
	if attr.Name.Space == "" {
		local = "xmlns"
	}
	return validation.QName{Space: sourceXMLNSNamespace, Local: local}
}
func sourceNameCapacity(name validation.QName) error {
	if len(name.Local) > 1024 || len(name.Space) > 4096 {
		return &validation.IncompleteError{Reason: "source-fact-identity-limit"}
	}
	return nil
}
func sourceLexeme(value string) validation.Scalar {
	return validation.Scalar{Lexeme: validation.Evidence[string]{State: validation.EvidenceKnown, Value: value}, Value: validation.Evidence[validation.Atomic]{State: validation.EvidenceUnknown, Reason: "schema-binding-pending"}}
}
func (o *sourceFactObserver) location(path string, position sourceXMLPosition) validation.Location {
	return validation.Location{Target: validation.TargetSource, DocumentID: o.documentID, Entry: o.entry, Path: path, Position: &validation.SourcePosition{ByteOffset: position.ByteOffset, Line: position.Line, Column: position.Column, CoordinateSpace: validation.CoordinateSpaceOriginalBytes}}
}

// The last source location outlives its closing frame when a mapper reports a
// scalar conversion error after consuming that end token. Own this small copied
// location separately instead of borrowing a released frame's path.
func (o *sourceFactObserver) rememberLocation(location validation.Location) error {
	size := int64(160 + len(location.Path))
	if err := o.account.Charge(validation.BudgetBufferedBytes, size); err != nil {
		return err
	}
	if err := o.account.Release(validation.BudgetBufferedBytes, o.lastRetained); err != nil {
		return err
	}
	o.retained += size - o.lastRetained
	o.lastRetained = size
	location.Path = strings.Clone(location.Path)
	if location.Position != nil {
		position := *location.Position
		location.Position = &position
	}
	o.last = location
	return nil
}
func (o *sourceFactObserver) retain(frame *sourceObserverFrame, size int64) error {
	if err := o.account.Charge(validation.BudgetBufferedBytes, size); err != nil {
		return err
	}
	frame.retained += size
	o.retained += size
	return nil
}
func (o *sourceFactObserver) pop() {
	frame := o.stack[len(o.stack)-1]
	for i := len(frame.undo) - 1; i >= 0; i-- {
		undo := frame.undo[i]
		if undo.existed {
			o.bindings[undo.prefix] = undo.previous
		} else {
			delete(o.bindings, undo.prefix)
		}
	}
	_ = o.account.Release(validation.BudgetBufferedBytes, frame.retained)
	o.retained -= frame.retained
	o.stack[len(o.stack)-1] = nil
	o.stack = o.stack[:len(o.stack)-1]
}
func (o *sourceFactObserver) close() error {
	if o == nil {
		return nil
	}
	if err := o.account.Release(validation.BudgetBufferedBytes, o.retained); err != nil {
		return fmt.Errorf("musicxml: release source observer: %w", err)
	}
	o.retained = 0
	o.stack = nil
	o.bindings = nil
	o.last = validation.Location{}
	o.lastRetained = 0
	o.origins = nil
	return nil
}

// observingXMLSource is the sole raw-prefix expansion boundary. Its independent
// parser stack survives model-conversion failure and owns the eventual EOF.
type observingXMLSource struct {
	ctx      context.Context
	decoder  *xml.Decoder
	lexical  *sourceXMLTokenReader
	observer *sourceFactObserver
	failure  error
	eof      bool
	started  bool
}

func (s *observingXMLSource) Token() (xml.Token, error) {
	if s.failure != nil {
		return nil, s.failure
	}
	if s.eof {
		return nil, io.EOF
	}
	if err := s.ctx.Err(); err != nil {
		s.failure = err
		return nil, err
	}
	token, err := s.decoder.Token()
	if err != nil {
		if err == io.EOF {
			s.eof = true
		} else {
			s.failure = err
		}
		return nil, err
	}
	if _, ok := token.(xml.StartElement); ok && !s.started {
		s.started = true
		if s.observer != nil {
			s.observer.dtdUnavailable = s.lexical.Summary().DTD.Unsupported
		}
	}
	if s.observer != nil {
		if err = s.observer.observeSource(token, s.lexical.Metadata(), s.lexical); err != nil {
			var unsupported *validation.UnsupportedRootError
			if errors.As(err, &unsupported) {
				err = errors.Join(err, &UnsupportedRootError{Name: xml.Name{Space: unsupported.Name.Space, Local: unsupported.Name.Local}})
			}
			s.failure = err
			return nil, err
		}
	}
	return token, nil
}
func (s *observingXMLSource) Skip() error {
	depth := 1
	for depth > 0 {
		token, err := s.Token()
		if err != nil {
			return err
		}
		switch token.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return nil
}
