package validation

import "context"

// ContainerContextRole identifies one exact container-assembly element role.
type ContainerContextRole string

// Container roles are selected by exact declarations in the sealed assembly.
const (
	ContainerRoot      ContainerContextRole = "container"
	ContainerRootfiles ContainerContextRole = "rootfiles"
	ContainerRootfile  ContainerContextRole = "rootfile"
)

// ContainerContextResult retains one source occurrence without interpreting a
// package graph. Subject and its original location are in the enclosing result.
// Container identifies the document root; Parent is zero only on that root.
// Content proves this occurrence's complete, grammar-valid ordered child-role
// inventory. A graph must consume both container and rootfiles Content before
// asserting a complete rootfile inventory. Malformed, unbound or skipped children
// preserve causal uncertainty; a known subset never proves complete inventory.
// Ordinal is the one-based rootfile position within Parent, known only when that
// rootfiles inventory is complete. Non-rootfile attributes and Ordinal are absent.
// FullPathRaw and MediaTypeRaw retain original XML-normalized lexemes. FullPath
// and MediaType are assessed xs:token values, including whitespace collapse.
// With NewSourceContainerMetadataContextPlan, Raw is physical source evidence:
// DTD-defaulted and entity-generated attributes are absent, and unretained
// general-entity replacement spellings are unknown. The effective token fields
// still include admitted DTD-supplied values.
// Omitted optional media-type is absent, distinct from known empty; no MIME
// default or classification is applied. Omitted required full-path is invalid
// effective evidence, with its XSD prerequisite cause, while its raw is absent.
type ContainerContextResult struct {
	Role                      ContainerContextRole
	Container, Parent         NodeRef
	Ordinal                   Evidence[uint64]
	FullPath, MediaType       Evidence[Atomic]
	FullPathRaw, MediaTypeRaw Evidence[string]
	Content                   Evidence[bool]
}

const (
	containerRoot      contextRole = "container.root"
	containerRootfiles contextRole = "container.rootfiles"
	containerRootfile  contextRole = "container.rootfile"
	containerFullPath  contextRole = "container.full-path"
	containerMediaType contextRole = "container.media-type"
)

func compileContainerMetadataBindings(schema *CompiledSchema) (*contextBindings, error) {
	b, err := compileContextBindingSpecs(schema, []contextBindingSpec{
		{Role: containerRootfiles, Owner: "container", Name: "rootfiles", Type: "rootfiles", Count: 1},
		{Role: containerRootfile, Owner: "rootfiles", Name: "rootfile", Type: "rootfile", Count: 1},
		{Role: containerFullPath, Owner: "rootfile", Name: "full-path", Type: "xs:token", Attribute: true, Count: 1},
		{Role: containerMediaType, Owner: "rootfile", Name: "media-type", Type: "xs:token", Attribute: true, Count: 1},
	})
	if err != nil {
		return nil, err
	}
	root, ok := schema.Root(QName{Local: "container"})
	if !ok {
		return nil, contract("container root", "exact global container declaration required")
	}
	for _, u := range schema.Uses() {
		if u.Declaration != root {
			continue
		}
		if u.Attribute || u.OwnerTypeKnown || !u.DeclaredTypeKnown || u.DeclaredType != contextAssemblyType(AssemblyContainer, "container") {
			return nil, contract("container root", "exact declared container root type required")
		}
		b.Roles[containerRoot] = []SchemaUse{u}
		b.Elements[containerRoot] = map[DeclarationID]struct{}{root: {}}
		b.ElementRoles[root] = []contextRole{containerRoot}
		b.bytes += 1536 + contextDeclarationBytes(root) + contextComponentBytes(u.DeclaredType) + int64(len(containerRoot))
		b.bindingDigest = b.hash()
		return b, nil
	}
	return nil, contract("container root", "exact global declaration use required")
}

// containerContent additionally requires every child to have the expected exact
// declaration. Grammar evidence alone cannot turn skipped bindings into absence.
func containerContent(r *contextReader, n *factNode, childRole contextRole) Evidence[bool] {
	v := r.complete(n)
	if v.State != EvidenceKnown || !v.Value {
		return v
	}
	if n.end.Order.Value != OrderSource {
		return stateUnknown[bool]("container-source-order-unavailable")
	}
	for _, ch := range n.children {
		if !r.step(1) {
			return stateUnknown[bool]("container-context-interrupted")
		}
		if ch.header.Declaration.State != EvidenceKnown {
			if ch.header.Declaration.State == EvidenceAbsent {
				return stateUnknown[bool]("container-child-binding-unavailable")
			}
			return stateUnavailable[DeclarationID, bool](*ch.header.Declaration)
		}
		if ch.header.Parent != n.header.ID || childRole == "" || !r.matches(ch, childRole) {
			return stateUnknown[bool]("container-child-role-unavailable")
		}
	}
	return v
}

func containerRawAttribute(r *contextReader, n *factNode, role contextRole) Evidence[string] {
	a, ok := r.attributeSelector(n, role)
	if !ok {
		if n.header.Type.State != EvidenceKnown {
			return stateUnavailable[ComponentID, string](*n.header.Type)
		}
		return Evidence[string]{State: EvidenceUnsupported, Reason: "context-attribute-binding-unavailable"}
	}
	presence, fact, err := r.s.attributeEvidence(r.ctx, n, a)
	r.err = err
	if r.err != nil {
		return Evidence[string]{}
	}
	if r.sourceInfoset {
		_, raw, err := r.s.sourcePhysicalAttribute(r.ctx, n, fact, presence)
		r.err = err
		return raw
	}
	if presence.State == EvidenceAbsent || presence.State == EvidenceKnown && !presence.Value {
		return Evidence[string]{State: EvidenceAbsent}
	}
	if presence.State != EvidenceKnown {
		return stateUnavailable[bool, string](presence)
	}
	if fact == nil {
		return stateUnknown[string]("container-attribute-lexeme-unavailable")
	}
	return fact.Scalar.Lexeme
}

func evaluateContextContainerMetadata(ctx context.Context, s *Session, _ *factNode, key RuleKey, p *contextPlan, final bool) (Decision, bool, error) {
	if !final {
		return Decision{}, true, nil
	}
	r := &contextReader{ctx: ctx, s: s, b: p.Bindings, sourceInfoset: p.sourceInfoset}
	root := s.root
	if !r.matches(root, containerRoot) {
		return Decision{Outcome: OutcomePass}, false, r.err
	}
	container := NodeRef{s.documentID, root.header.ID}
	publish := func(n *factNode, role ContainerContextRole, content Evidence[bool], ordinal Evidence[uint64]) error {
		value := ContainerContextResult{Role: role, Container: container, Ordinal: ordinal, Content: content,
			FullPath: Evidence[Atomic]{State: EvidenceAbsent}, MediaType: Evidence[Atomic]{State: EvidenceAbsent}, FullPathRaw: Evidence[string]{State: EvidenceAbsent}, MediaTypeRaw: Evidence[string]{State: EvidenceAbsent}}
		if n.header.Parent != 0 {
			value.Parent = NodeRef{s.documentID, n.header.Parent}
		}
		if role == ContainerRootfile {
			value.FullPath, value.MediaType = r.attribute(n, containerFullPath), r.attribute(n, containerMediaType)
			value.FullPathRaw, value.MediaTypeRaw = containerRawAttribute(r, n, containerFullPath), containerRawAttribute(r, n, containerMediaType)
		}
		if r.err != nil {
			return r.err
		}
		return s.publishContextResult(ctx, ContextResult{Provider: key, Subject: NodeRef{s.documentID, n.header.ID}, Property: ContextPropertyContainerMetadata,
			Value: Evidence[Atomic]{State: EvidenceAbsent}, Origins: []Location{n.header.Location}, Container: &value})
	}
	if err := publish(root, ContainerRoot, containerContent(r, root, containerRootfiles), Evidence[uint64]{State: EvidenceAbsent}); err != nil {
		return Decision{}, false, err
	}
	for _, files := range root.children {
		if !r.step(1) {
			return Decision{}, false, r.err
		}
		if !r.matches(files, containerRootfiles) {
			continue
		}
		content := containerContent(r, files, containerRootfile)
		if err := publish(files, ContainerRootfiles, content, Evidence[uint64]{State: EvidenceAbsent}); err != nil {
			return Decision{}, false, err
		}
		for i, file := range files.children {
			if !r.step(1) {
				return Decision{}, false, r.err
			}
			if !r.matches(file, containerRootfile) {
				continue
			}
			ordinal := stateUnavailable[bool, uint64](content)
			if content.State == EvidenceKnown && content.Value {
				ordinal = Evidence[uint64]{State: EvidenceKnown, Value: uint64(i) + 1}
			}
			if err := publish(file, ContainerRootfile, containerContent(r, file, ""), ordinal); err != nil {
				return Decision{}, false, err
			}
		}
	}
	return Decision{Outcome: OutcomePass}, false, r.err
}

func cloneContainerContext(ctx context.Context, s *Session, p *ContainerContextResult) (*ContainerContextResult, error) {
	if p == nil {
		return nil, nil
	}
	if err := contextRetainProjection(ctx, s, *p); err != nil {
		return nil, err
	}
	q := stateClone(*p)
	return &q, nil
}
