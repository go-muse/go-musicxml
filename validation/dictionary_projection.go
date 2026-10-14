package validation

import (
	"context"
	"reflect"
)

// Dictionary properties identify independently assessed exact relations and
// narrow rendering facts. They do not imply full MusicXML or IPA conformance.
const (
	ContextDictionaryCanonical ContextProperty = "dictionary.canonical-name"
	ContextDictionarySounds    ContextProperty = "dictionary.standard-sound"
	ContextDictionaryIPA       ContextProperty = "dictionary.ipa-positive"
	ContextDictionaryRendering ContextProperty = "dictionary.rendering"
	ContextDictionaryAdvice    ContextProperty = "dictionary.advice"
)

// DictionarySpan retains a byte-indexed part of the exact assessed UTF-8 input.
// Unknown and explicitly empty spans are retained. RowID identifies a reviewed
// positive row; its absence never proves that the encoding is forbidden.
type DictionarySpan struct {
	Start, End                  uint64
	Text                        string
	Result                      Evidence[bool]
	RowID, Category, Equivalent string
}

// DictionaryContextResult is caller-owned evidence for one scalar subject and
// relation. Attribute is nil for element content. ComponentVerified means the
// pinned component was verified on this run's account; SourceKind distinguishes
// an original component from a reviewed derived bundle. DataSHA256 and Policy
// identify the reviewed transform and semantic policy, never caller assertions.
// Result records membership only. WholeSequence stays unavailable for partial
// IPA evidence, including attested examples. Rendering facts use Role, Input,
// Effective and Applicability without asserting mandatory range or intent rules.
type DictionaryContextResult struct {
	Manifest           DictionaryManifestID
	Relation           DictionaryRelation
	Attribute          *AttributeSelector
	Input              Evidence[Atomic]
	Result             Evidence[bool]
	Codepoint          Evidence[uint32]
	WholeSequence      Evidence[bool]
	Spans              []DictionarySpan
	ComponentVerified  bool
	SourceKind         DictionarySourceKind
	DataSHA256, Policy string
	Role               string
	Effective          Evidence[Atomic]
	Applicability      Evidence[bool]
}

func dictionaryPropertyValid(p ContextProperty) bool {
	switch p {
	case ContextDictionaryCanonical, ContextDictionarySounds, ContextDictionaryIPA, ContextDictionaryRendering, ContextDictionaryAdvice:
		return true
	}
	return false
}

func cloneDictionaryContext(ctx context.Context, s *Session, p *DictionaryContextResult) (*DictionaryContextResult, error) {
	if p == nil {
		return nil, nil
	}
	// Dictionary scalar values live inside the compound payload. Outer Value is
	// absent, so its generic admission does not cover defaults or derived text.
	if err := s.step(ctx, 1); err != nil {
		return nil, err
	}
	for _, value := range []Atomic{p.Input.Value, p.Effective.Value} {
		size := value.Integer.ByteSize()
		for _, n := range []int64{value.Decimal.ByteSize(), int64(len(value.Text)), int64(len(value.Name.Space)), int64(len(value.Name.Local))} {
			var err error
			size, err = rationalCheckedAdd(BudgetScalarBytes, size, n)
			if err != nil {
				return nil, err
			}
		}
		if err := s.account.Check(BudgetScalarBytes, size); err != nil {
			return nil, err
		}
	}
	for _, span := range p.Spans {
		if err := s.step(ctx, 1); err != nil {
			return nil, err
		}
		if err := s.account.Check(BudgetScalarBytes, int64(len(span.Text))); err != nil {
			return nil, err
		}
		if err := s.account.Check(BudgetScalarBytes, int64(len(span.Equivalent))); err != nil {
			return nil, err
		}
	}
	size, err := resourceMeasure(ctx, s.account, reflect.ValueOf(*p))
	if err != nil {
		return nil, err
	}
	if _, err = AllocationSize(size); err != nil {
		return nil, err
	}
	if err = s.retain(size); err != nil {
		return nil, err
	}
	q := stateClone(*p)
	return &q, nil
}
