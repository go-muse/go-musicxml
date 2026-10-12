package validation

type layoutFormatField struct {
	Value  Evidence[Atomic]
	Origin NodeRef
}

var layoutFormatNames = [17]string{"font-family", "font-style", "font-weight", "font-size", "justify", "halign", "valign", "enclosure", "language", "line-height", "letter-spacing", "dir", "color", "rotation", "underline", "overline", "line-through"}

type layoutFormat [17]layoutFormatField
type layoutPublisherKey struct {
	Name, Number               string
	NamePresent, NumberPresent bool
}
type layoutPublisher struct {
	Node         *factNode
	Name, Number Evidence[Atomic]
	Values       [5]Evidence[Atomic]
}
type layoutFormatting struct {
	r          *contextReader
	key        RuleKey
	formats    map[NodeID]*layoutFormat
	previous   map[NodeID]*factNode
	publishers map[layoutPublisherKey][]layoutPublisher
	uncertain  []layoutPublisher
	closed     Evidence[bool]
	defaults   *factNode
}

func newLayoutFormatting(r *contextReader, key RuleKey) (*layoutFormatting, error) {
	if !r.step(1) {
		return nil, r.err
	}
	if err := r.s.retain(2048); err != nil {
		return nil, err
	}
	f := &layoutFormatting{r: r, key: key, formats: make(map[NodeID]*layoutFormat), previous: make(map[NodeID]*factNode), publishers: make(map[layoutPublisherKey][]layoutPublisher), closed: Evidence[bool]{State: EvidenceKnown, Value: true}}
	for _, id := range r.s.sequence {
		if !r.step(1) {
			return nil, r.err
		}
		n := r.s.nodes[id]
		if n == nil {
			continue
		}
		if n.header.Parent == 0 {
			f.closed = timelineMergeAvailability(f.closed, r.enumerated(n))
		}
		if r.matches(n, layoutRole("defaults")) {
			if f.defaults != nil {
				f.closed = stateUnknown[bool]("multiple-defaults-publishers")
			} else {
				f.defaults = n
			}
			for _, ch := range n.children {
				if !r.step(1) {
					return nil, r.err
				}
				if ch.header.Declaration.State != EvidenceKnown {
					f.closed = stateUnavailable[DeclarationID, bool](ch.header.Declaration)
				}
			}
			f.closed = timelineMergeAvailability(f.closed, r.enumerated(n))
		}
		font, lang := r.matches(n, layoutRole("lyric-font")), r.matches(n, layoutRole("lyric-language"))
		if !font && !lang {
			continue
		}
		p := layoutPublisher{Node: n, Name: layoutAttribute(r, n, "name"), Number: layoutAttribute(r, n, "number")}
		for i := range p.Values {
			p.Values[i] = Evidence[Atomic]{State: EvidenceAbsent}
		}
		if font {
			for i := 0; i < 4; i++ {
				p.Values[i] = layoutAttribute(r, n, layoutFormatNames[i])
			}
		}
		if lang {
			p.Values[4] = layoutLanguage(r, n)
		}
		if r.err != nil {
			return nil, r.err
		}
		if err := r.s.retain(3072 + int64(len(p.Name.Value.Text)+len(p.Number.Value.Text))); err != nil {
			return nil, err
		}
		k, known := layoutPublisherKeyFor(p.Name, p.Number)
		if known {
			f.publishers[k] = append(f.publishers[k], p)
		} else {
			f.uncertain = append(f.uncertain, p)
		}
	}
	return f, r.err
}
func layoutPublisherKeyFor(name, number Evidence[Atomic]) (layoutPublisherKey, bool) {
	if (name.State != EvidenceKnown && name.State != EvidenceAbsent) || (number.State != EvidenceKnown && number.State != EvidenceAbsent) {
		return layoutPublisherKey{}, false
	}
	return layoutPublisherKey{Name: name.Value.Text, Number: number.Value.Text, NamePresent: name.State == EvidenceKnown, NumberPresent: number.State == EvidenceKnown}, true
}
func layoutAttribute(r *contextReader, n *factNode, name string) Evidence[Atomic] {
	role := layoutRole("@" + name)
	if name == "smufl" && r.attributeOwner(n, layoutRole("elision.@smufl")) {
		role = layoutRole("elision.@smufl")
	}
	if !r.attributeOwner(n, role) {
		return Evidence[Atomic]{State: EvidenceAbsent}
	}
	return localRawAttribute(r, n, role)
}
func layoutLanguage(r *contextReader, n *factNode) Evidence[Atomic] {
	for _, owner := range []string{"formatted-text", "formatted-text-id", "text-element-data", "lyric-language", "accidental-text"} {
		role := layoutRole("lang." + owner)
		if r.attributeOwner(n, role) {
			return localRawAttribute(r, n, role)
		}
	}
	return Evidence[Atomic]{State: EvidenceAbsent}
}
func (f *layoutFormatting) lyricValue(lyric *factNode, component int) layoutFormatField {
	r := f.r
	name, number := layoutAttribute(r, lyric, "name"), layoutAttribute(r, lyric, "number")
	result := layoutFormatField{Value: Evidence[Atomic]{State: EvidenceAbsent}}
	consume := func(p layoutPublisher) {
		if !r.step(1 + int64(len(p.Name.Value.Text)+len(p.Number.Value.Text)+len(name.Value.Text)+len(number.Value.Text))) {
			return
		}
		match := layoutSelectorMatch(p.Name, p.Number, name, number)
		if match.State == EvidenceKnown && !match.Value {
			return
		}
		value := p.Values[component]
		if value.State == EvidenceAbsent {
			return
		}
		if match.State != EvidenceKnown {
			value = stateUnavailable[bool, Atomic](match)
		}
		if !r.step(1 + atomicLogicalBytes(result.Value.Value) + atomicLogicalBytes(value.Value)) {
			return
		}
		result.Value = layoutConsensusPair(result.Value, value)
		if result.Origin.Node == 0 {
			result.Origin = NodeRef{r.s.documentID, p.Node.header.ID}
		}
	}
	keys := [4]layoutPublisherKey{{}, {Name: name.Value.Text, NamePresent: true}, {Number: number.Value.Text, NumberPresent: true}, {Name: name.Value.Text, Number: number.Value.Text, NamePresent: true, NumberPresent: true}}
	for _, k := range keys {
		if k.NamePresent && name.State != EvidenceKnown {
			continue
		}
		if k.NumberPresent && number.State != EvidenceKnown {
			continue
		}
		if !r.step(1 + int64(len(k.Name)+len(k.Number))) {
			break
		}
		for _, p := range f.publishers[k] {
			consume(p)
		}
	}
	for _, p := range f.uncertain {
		consume(p)
	}
	if name.State != EvidenceKnown && name.State != EvidenceAbsent || number.State != EvidenceKnown && number.State != EvidenceAbsent {
		result.Value = stateUnknown[Atomic]("lyric-selector-unavailable")
	}
	if f.closed.State != EvidenceKnown || !f.closed.Value {
		result.Value = stateUnavailable[bool, Atomic](f.closed)
		if result.Value.State == EvidenceKnown {
			result.Value = stateUnknown[Atomic]("lyric-publishers-unclosed")
		}
	}
	return result
}
func (f *layoutFormatting) resolve(n *factNode) error {
	r := f.r
	parent := r.s.nodes[n.header.Parent]
	isDirection := r.matches(parent, layoutRole("direction-type"))
	isCredit := r.matches(parent, layoutRole("credit")) && (r.matches(n, layoutRole("credit-words")) || r.matches(n, layoutRole("credit-symbol")))
	hasFont := r.attributeOwner(n, layoutRole("@font-family"))
	hasStyle := hasFont || r.attributeOwner(n, layoutRole("@justify")) || r.attributeOwner(n, layoutRole("@valign"))
	if !hasStyle {
		return r.err
	}
	if err := r.s.retain(8192); err != nil {
		return err
	}
	style := new(layoutFormat)
	ref := NodeRef{r.s.documentID, n.header.ID}
	previous := (*factNode)(nil)
	if isDirection || isCredit {
		previous = f.previous[parent.header.ID]
	}
	crossWrapper := false
	if isDirection && previous == nil {
		direction := r.s.nodes[parent.header.Parent]
		if direction != nil && r.matches(direction, layoutRole("direction")) {
			if old := f.previous[direction.header.ID]; old != nil && old.header.Parent != parent.header.ID {
				crossWrapper = true
				previous = old
			}
		}
	}
	lyric := (*factNode)(nil)
	for a := parent; a != nil; a = r.s.nodes[a.header.Parent] {
		if !r.step(1) {
			return r.err
		}
		if r.matches(a, layoutRole("lyric")) {
			lyric = a
			break
		}
		if r.matches(a, layoutRole("direction")) || r.matches(a, layoutRole("credit")) {
			break
		}
	}
	for i, name := range layoutFormatNames {
		role := layoutRole("@" + name)
		if name == "smufl" && r.attributeOwner(n, layoutRole("elision.@smufl")) {
			role = layoutRole("elision.@smufl")
		}
		owns := r.attributeOwner(n, role)
		if name == "language" {
			owns = true
		}
		if !owns {
			(*style)[i].Value = Evidence[Atomic]{State: EvidenceAbsent}
			continue
		}
		raw := layoutAttribute(r, n, name)
		if name == "language" {
			raw = layoutLanguage(r, n)
		}
		value := layoutFormatField{Value: raw, Origin: ref}
		if raw.State == EvidenceAbsent && previous != nil {
			if prior := f.formats[previous.header.ID]; prior != nil {
				inherited := (*prior)[i]
				if inherited.Value.State != EvidenceAbsent {
					value = inherited
					if crossWrapper {
						value.Value = stateUnknown[Atomic]("direction-cross-wrapper-format-precedence")
					}
				}
			}
		}
		if value.Value.State == EvidenceAbsent && parent != nil && i < 4 && (r.matches(n, layoutRole("per-minute")) && r.matches(parent, layoutRole("metronome"))) {
			if inherited := f.formats[parent.header.ID]; inherited != nil {
				value = (*inherited)[i]
			}
		}
		if value.Value.State == EvidenceAbsent && (i < 4 || name == "color") {
			for a := parent; a != nil; a = r.s.nodes[a.header.Parent] {
				if !r.step(1) {
					return r.err
				}
				if r.matches(a, layoutRole("harmony")) {
					if inherited := f.formats[a.header.ID]; inherited != nil {
						value = (*inherited)[i]
					}
					break
				}
				if r.matches(a, layoutRole("direction")) || r.matches(a, layoutRole("lyric")) {
					break
				}
			}
		}
		if value.Value.State == EvidenceAbsent && name == "language" {
			for a := parent; a != nil; a = r.s.nodes[a.header.Parent] {
				if !r.step(1) {
					return r.err
				}
				lang := layoutLanguage(r, a)
				if lang.State != EvidenceAbsent {
					value = layoutFormatField{Value: lang, Origin: NodeRef{r.s.documentID, a.header.ID}}
					break
				}
			}
		}
		if value.Value.State == EvidenceAbsent && lyric != nil && (i < 4 || name == "language") {
			component := i
			if name == "language" {
				component = 4
			}
			value = f.lyricValue(lyric, component)
		}
		if value.Value.State == EvidenceAbsent && i < 4 && f.defaults != nil && lyric == nil {
			font, e := r.child(f.defaults, layoutRole("word-font"))
			if e.State == EvidenceKnown {
				value = layoutFormatField{Value: layoutAttribute(r, font, name), Origin: NodeRef{r.s.documentID, font.header.ID}}
			} else if e.State != EvidenceAbsent {
				value.Value = stateUnavailable[bool, Atomic](e)
			}
		}
		if value.Value.State == EvidenceAbsent {
			fallback := ""
			switch name {
			case "justify":
				if isDirection {
					fallback = "left"
				}
				if r.matches(n, layoutRole("lyric")) {
					fallback = "center"
				}
			case "halign":
				value = (*style)[4]
			case "valign":
				if r.matches(n, layoutRole("lyric")) {
					fallback = "baseline"
				}
			case "enclosure":
				if r.matches(n, layoutRole("rehearsal")) {
					fallback = "square"
				} else if isDirection || isCredit || r.matches(n, layoutRole("display-text")) || r.matches(n, layoutRole("accidental-text")) {
					fallback = "none"
				}
			case "language":
				if r.matches(n, layoutRole("words")) || r.matches(n, layoutRole("rehearsal")) || r.matches(n, layoutRole("display-text")) || r.matches(n, layoutRole("accidental-text")) {
					fallback = "it"
				}
			case "line-height", "letter-spacing":
				fallback = "normal"
			case "dir":
				fallback = "ltr"
			}
			if fallback != "" {
				value.Value = layoutDefault(value.Value, fallback)
			}
			if value.Value.State == EvidenceAbsent {
				value.Value = stateUnknown[Atomic]("application-format-choice")
			}
		}
		if value.Origin.Node == 0 {
			value.Origin = ref
		}
		(*style)[i] = value
		if r.err != nil {
			return r.err
		}
		provenance := ContextExplicit
		if raw.State == EvidenceAbsent {
			provenance = ContextDocumentedDefault
			if value.Origin != ref {
				provenance = ContextInherited
			}
			if value.Value.State == EvidenceUnknown {
				provenance = ContextApplicationChoice
			}
		}
		payload := &LayoutContextResult{Raw: raw, Applicability: Evidence[bool]{State: EvidenceKnown, Value: true}, Scope: ref, Origin: value.Origin, Provenance: provenance}
		if name == "rotation" {
			payload.Basis = "positive-clockwise-about-alignment-point"
		}
		if name == "font-size" && value.Value.State == EvidenceKnown && value.Value.Value.Kind == AtomicDecimal {
			payload.Unit = ContextUnitPoints
		}
		if name == "line-height" && value.Value.State == EvidenceKnown && value.Value.Value.Kind == AtomicDecimal {
			payload.Unit = ContextUnitPercent
		}
		if err := r.s.publishContextResult(r.ctx, ContextResult{Provider: f.key, Subject: ref, Property: ContextProperty("layout." + name), Value: value.Value, Layout: payload}); err != nil {
			return err
		}
	}
	f.formats[n.header.ID] = style
	if isDirection || isCredit {
		f.previous[parent.header.ID] = n
	}
	if isDirection {
		f.previous[parent.header.Parent] = n
	}
	return r.err
}
