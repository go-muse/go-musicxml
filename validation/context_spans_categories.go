package validation

import "context"

// Unavailable category evidence is not the principal-note category. Prefer its
// original cause; hand-built admitted endpoints may carry that cause in complete.
func spanCategoryAvailability(e *spanEndpoint) Evidence[bool] {
	if e.categoryKnown {
		return Evidence[bool]{State: EvidenceKnown, Value: true}
	}
	for _, v := range []Evidence[bool]{e.categoryAvailable, e.complete} {
		if v.State != EvidenceKnown && v.State != EvidenceAbsent && v.State != "" {
			return v
		}
	}
	return stateUnknown[bool]("span-note-category-unavailable")
}

const (
	spanCategoryDomain uint8 = 1 << iota
	spanCategoryNumber
	spanCategoryVoice
)

type spanBeamCategoryGapKey struct {
	identity spanAssociationKey
	known    uint8
}

// Retain each established identity axis independently. Missing category (or a
// second missing axis) can interfere only with compatible beam runs.
func spanBeamCategoryGap(ctx context.Context, s *Session, e *spanEndpoint) (spanBeamCategoryGapKey, Evidence[bool], error) {
	k := spanBeamCategoryGapKey{identity: spanAssociationKey{family: SpanBeam, identity: SpanIdentityBeamLevel}}
	v := timelineMergeAvailability(spanCategoryAvailability(e), e.complete)
	p := &e.payload
	if p.Domain.State == EvidenceKnown {
		k.known |= spanCategoryDomain
		k.identity.domain = p.Domain.Value
	} else if p.Domain.State != EvidenceAbsent {
		v = timelineMergeAvailability(v, stateUnavailable[NodeRef, bool](p.Domain))
	}
	number, ok, err := spanAtomicKey(ctx, s, p.Number)
	if err != nil {
		return k, v, err
	}
	if ok {
		k.known |= spanCategoryNumber
		k.identity.number = number
	} else if p.Number.State != EvidenceKnown && p.Number.State != EvidenceAbsent {
		v = timelineMergeAvailability(v, stateUnavailable[Atomic, bool](p.Number))
	}
	switch p.Voice.State {
	case EvidenceKnown:
		k.known |= spanCategoryVoice
		k.identity.voice = p.Voice.Value
		k.identity.voiceSpecified = true
	case EvidenceAbsent:
		k.known |= spanCategoryVoice
	default:
		v = timelineMergeAvailability(v, stateUnavailable[string, bool](p.Voice))
	}
	if err := s.step(ctx, 1+int64(len(k.identity.domain.DocumentID)+len(k.identity.number)+len(k.identity.voice))); err != nil {
		return k, v, err
	}
	return k, v, nil
}

func spanBeamCategoryRunGap(ctx context.Context, s *Session, k spanAssociationKey, gaps map[spanBeamCategoryGapKey]Evidence[bool]) (Evidence[bool], error) {
	available := Evidence[bool]{State: EvidenceKnown, Value: true}
	if len(gaps) == 0 || k.family != SpanBeam {
		return available, nil
	}
	k.grace, k.cue = false, false
	for known := uint8(0); known < 8; known++ {
		if err := s.step(ctx, 1+int64(len(k.domain.DocumentID)+len(k.number)+len(k.voice))); err != nil {
			return Evidence[bool]{}, err
		}
		candidate := k
		if known&spanCategoryDomain == 0 {
			candidate.domain = NodeRef{}
		}
		if known&spanCategoryNumber == 0 {
			candidate.number = ""
		}
		if known&spanCategoryVoice == 0 {
			candidate.voice = ""
			candidate.voiceSpecified = false
		}
		if gap, ok := gaps[spanBeamCategoryGapKey{identity: candidate, known: known}]; ok {
			available = timelineMergeAvailability(available, gap)
		}
	}
	return available, nil
}
