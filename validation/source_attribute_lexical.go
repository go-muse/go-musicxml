package validation

import (
	"context"
	"strings"
	"unicode/utf8"
)

const sourceAttributeLexicalEntryBytes int64 = 256
const sourceAttributeLexicalHeaderBytes int64 = 512

// validateSourceResourceProfile elects only available provenance-aware resource
// plans. The lookup is immutable catalog state, never inferred from a caller's
// editable profile or the presence of source-origin facts in a report.
func validateSourceResourceProfile(c *CompiledCatalog, profile Profile, closure map[RuleKey]bool, supports map[RuleKey]RuleSupport) error {
	for key := range closure {
		p := c.plans[c.rules[key].Predicate]
		if p.context == nil || !p.context.sourceInfoset || supports[key].State != ImplementationAvailable {
			continue
		}
		assembly := p.context.Schema.assembly
		provider := c.sourceOrigins[profile.ID][assembly]
		if profile.Target != TargetSource || provider == (RuleKey{}) {
			return contract("source resource profile", "matching available source-origin provider required")
		}
		if c.sourceAttributeLexical == nil {
			c.sourceAttributeLexical = make(map[ProfileID]map[Assembly]RuleKey)
		}
		if c.sourceAttributeLexical[profile.ID] == nil {
			c.sourceAttributeLexical[profile.ID] = make(map[Assembly]RuleKey)
		}
		c.sourceAttributeLexical[profile.ID][assembly] = provider
	}
	return nil
}

// SourceAttributeLexicalProvider identifies the exact selected source-origin
// provider authorized to admit original attribute lexical evidence for a source
// resource composition. Legacy source-origin profiles do not enable this input.
func (e *Engine) SourceAttributeLexicalProvider(profile ProfileID, assembly Assembly) (RuleKey, bool) {
	if e == nil || e.catalog == nil {
		return RuleKey{}, false
	}
	key := e.catalog.sourceAttributeLexical[profile][assembly]
	if key == (RuleKey{}) {
		return RuleKey{}, false
	}
	return RuleKey{ID: RuleID(strings.Clone(string(key.ID))), Revision: key.Revision}, true
}

func (s *Session) sourceAttributeLexicalKey() RuleKey {
	if s == nil || s.catalog == nil || s.schema == nil || s.schema.active == nil {
		return RuleKey{}
	}
	return s.catalog.sourceAttributeLexical[s.profile.ID][s.schema.active.assembly]
}

// SourceAttributeLexical admits one independently owned original XML-normalized
// attribute spelling, before DTD tokenized normalization. Call it after the
// matching Attribute and SourceOrigin events while the exact owner remains open.
// The provider must be elected by SourceAttributeLexicalProvider. Known requires
// a specified physical attribute and genuinely retained original spelling;
// replaced general-entity spelling that was not retained is Unknown. Defaults
// and entity-generated attributes require Absent. Missing input never permits
// reconstructing Raw from the effective AttributeFact lexeme. This adds no
// public context row and does not change the effective attribute value. Every
// rejection is terminal; Finish still releases all owned live gauges.
func (s *Session) SourceAttributeLexical(ctx context.Context, provider RuleKey, subject NodeRef, attribute QName, raw Evidence[string]) (err error) {
	if err := s.event(ctx, "SourceAttributeLexical"); err != nil {
		return err
	}
	if s.profile.Target != TargetSource || provider == (RuleKey{}) || provider != s.sourceAttributeLexicalKey() {
		return s.fail(contract("source attribute lexical provider", "selected provenance-aware source resource provider required"))
	}
	if len(s.stack) == 0 || s.sourceOriginSeal != nil || subject != (NodeRef{s.documentID, s.stack[len(s.stack)-1].header.ID}) {
		return s.fail(&SessionError{Operation: "SourceAttributeLexical", Reason: "currently open exact subject required"})
	}
	// Validate bounded metadata before calling shared logical-byte helpers: those
	// helpers may sum native ints and assume admitted identity-sized fields.
	if !validQName(attribute) {
		return s.fail(contract("source attribute lexical", "exact expanded attribute name required"))
	}
	if err := validateEvidence(raw, "source attribute lexical"); err != nil {
		return s.fail(err)
	}
	if err := s.account.Check(BudgetScalarBytes, int64(len(raw.Value))); err != nil {
		return s.fail(err)
	}
	work, err := rationalCheckedMul(BudgetRuleSteps, int64(len(raw.Value)), 2)
	if err != nil {
		return s.fail(err)
	}
	work, err = rationalSum(BudgetRuleSteps, 1, int64(len(attribute.Space)), int64(len(attribute.Local)), work, evidenceLogicalBytes(raw))
	if err != nil {
		return s.fail(err)
	}
	if err := s.step(ctx, work); err != nil {
		return s.fail(err)
	}
	if raw.State == EvidenceKnown {
		if err := sourceAttributeLexicalUTF8(ctx, s, raw.Value); err != nil {
			return s.fail(err)
		}
	}
	slot := sourceOriginSubject{Node: subject.Node, Attribute: attribute}
	origin := s.sourceOriginSeen[slot]
	if origin == nil || origin.Attribute == nil {
		return s.fail(&SessionError{Operation: "SourceAttributeLexical", Reason: "admitted exact attribute origin required"})
	}
	if _, found := s.sourceAttributeLexical[slot]; found {
		return s.fail(&SessionError{Operation: "SourceAttributeLexical", Reason: "duplicate attribute lexical evidence"})
	}
	physical := origin.Specified.State == EvidenceKnown && origin.Specified.Value
	synthetic := origin.Specified.State == EvidenceKnown && !origin.Specified.Value
	if raw.State == EvidenceKnown && !physical || raw.State == EvidenceAbsent && !synthetic || synthetic && raw.State != EvidenceAbsent {
		return s.fail(contract("source attribute lexical", "raw evidence contradicts physical specifiedness"))
	}
	// Both map growth and the detached evidence/string are admitted before any
	// allocation. Keys borrow only the separately owned origin projection.
	copyBytes, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(raw.Value)), 2)
	if err != nil {
		return s.fail(err)
	}
	cost, err := rationalSum(BudgetBufferedBytes, sourceAttributeLexicalEntryBytes, copyBytes, 16, evidenceLogicalBytes(raw))
	if err != nil {
		return s.fail(err)
	}
	if s.sourceAttributeLexical == nil {
		cost, err = rationalCheckedAdd(BudgetBufferedBytes, cost, sourceAttributeLexicalHeaderBytes)
		if err != nil {
			return s.fail(err)
		}
	}
	before := s.retained
	defer func() {
		if err != nil {
			if delta := s.retained - before; delta > 0 {
				_ = s.account.Release(BudgetBufferedBytes, delta)
				s.retained = before
			}
			s.fail(err)
		}
	}()
	if err = s.retain(cost); err != nil {
		return err
	}
	owned := cloneEvidence(raw)
	owned.Value, err = resourceCopyString(ctx, raw.Value)
	if err != nil {
		return err
	}
	if err = s.step(ctx, 0); err != nil {
		return err
	}
	slot.Attribute = *origin.Attribute
	if s.sourceAttributeLexical == nil {
		s.sourceAttributeLexical = make(map[sourceOriginSubject]Evidence[string])
	}
	s.sourceAttributeLexical[slot] = owned
	return nil
}

// sourcePhysicalAttribute consumes an exact declaration-matched attribute from
// attributeEvidence (or the exact standard xsi hint slot). Origin lookup is a
// charged constant-time index over already owned publications, not a report scan.
func (s *Session) sourcePhysicalAttribute(ctx context.Context, n *factNode, attribute *AttributeFact, presence Evidence[bool]) (Evidence[bool], Evidence[string], error) {
	if presence.State == EvidenceAbsent || presence.State == EvidenceKnown && !presence.Value {
		return Evidence[bool]{State: EvidenceAbsent}, Evidence[string]{State: EvidenceAbsent}, nil
	}
	if presence.State != EvidenceKnown {
		return presence, stateUnavailable[bool, string](presence), nil
	}
	if attribute == nil {
		return stateUnknown[bool]("source-attribute-origin-unavailable"), stateUnknown[string]("source-attribute-origin-unavailable"), nil
	}
	if err := s.step(ctx, 1+int64(len(attribute.Name.Space))+int64(len(attribute.Name.Local))); err != nil {
		return Evidence[bool]{}, Evidence[string]{}, err
	}
	slot := sourceOriginSubject{Node: n.header.ID, Attribute: attribute.Name}
	origin := s.sourceOriginSeen[slot]
	if origin == nil {
		return stateUnknown[bool]("source-attribute-origin-unavailable"), stateUnknown[string]("source-attribute-origin-unavailable"), nil
	}
	if origin.Specified.State != EvidenceKnown {
		return origin.Specified, stateUnavailable[bool, string](origin.Specified), nil
	}
	if !origin.Specified.Value {
		return Evidence[bool]{State: EvidenceAbsent}, Evidence[string]{State: EvidenceAbsent}, nil
	}
	if raw, found := s.sourceAttributeLexical[slot]; found {
		return Evidence[bool]{State: EvidenceKnown, Value: true}, raw, nil
	}
	return Evidence[bool]{State: EvidenceKnown, Value: true}, stateUnknown[string]("source-attribute-lexical-unavailable"), nil
}

// Check UTF-8 in bounded blocks without splitting valid encodings. The maximum
// scalar limit is caller-configurable; admission alone is not cancellation.
func sourceAttributeLexicalUTF8(ctx context.Context, s *Session, value string) error {
	for start := 0; start < len(value); {
		if err := s.step(ctx, 0); err != nil {
			return err
		}
		end := start + min(resourceCopyChunk, len(value)-start)
		if end < len(value) {
			for i := 0; i < utf8.UTFMax-1 && !utf8.RuneStart(value[end]); i++ {
				end--
			}
		}
		if !utf8.ValidString(value[start:end]) {
			return contract("source attribute lexical", "invalid UTF-8")
		}
		start = end
	}
	return nil
}
