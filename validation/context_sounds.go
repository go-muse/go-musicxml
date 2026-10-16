package validation

import "context"

// SoundMappingRole identifies the exact sounds-assembly element role.
type SoundMappingRole string

// Sound mapping roles are assembly-qualified by the sealed plan, never matched
// by local element name in an arbitrary score or foreign namespace.
const (
	SoundMappingSound    SoundMappingRole = "sound"
	SoundMappingAny      SoundMappingRole = "any"
	SoundMappingSolo     SoundMappingRole = "solo"
	SoundMappingEnsemble SoundMappingRole = "ensemble"
)

// SoundMappingDirection states which direction a primary preference describes.
type SoundMappingDirection string

// A sound's primary is a reverse preference; mapping children carry forward
// preferences. Neither direction chooses a unique mapping automatically.
const (
	SoundMappingToMusicXML   SoundMappingDirection = "library-to-musicxml"
	SoundMappingFromMusicXML SoundMappingDirection = "musicxml-to-library"
)

// SoundMappingContextResult is an owned projection of one exact occurrence.
// Sound identifies its owning sound element. SoundID is the assessed lexical ID;
// document identity validity remains an independent XSD report obligation.
// LibraryID preserves xs:string whitespace and is absent on the sound itself.
// Primary preserves known yes/no, absence and causal invalid/unavailable evidence;
// omission has no XSD or prose default. Explicitly selected DTD defaults remain
// effective supplied values. Number is xs:NMTOKEN and only applies to ensemble.
// Content describes the owning sound's grammar and child enumeration, independent
// of its fields. Alternatives remain separate results in original source order.
// No dictionary membership, cross-file association or application choice is inferred.
type SoundMappingContextResult struct {
	Role      SoundMappingRole
	Direction SoundMappingDirection
	Sound     NodeRef
	SoundID   Evidence[Atomic]
	LibraryID Evidence[Atomic]
	Primary   Evidence[Atomic]
	Number    Evidence[Atomic]
	Content   Evidence[bool]
}

const (
	soundsSound           contextRole = "sounds.sound"
	soundsAny             contextRole = "sounds.any"
	soundsSolo            contextRole = "sounds.solo"
	soundsEnsemble        contextRole = "sounds.ensemble"
	soundsID              contextRole = "sounds.id"
	soundsSoundPrimary    contextRole = "sounds.sound-primary"
	soundsAnyPrimary      contextRole = "sounds.any-primary"
	soundsSoloPrimary     contextRole = "sounds.solo-primary"
	soundsEnsemblePrimary contextRole = "sounds.ensemble-primary"
	soundsNumber          contextRole = "sounds.number"
)

func contextSoundsMappingBindingSpecs() []contextBindingSpec {
	return []contextBindingSpec{
		{Role: soundsSound, Owner: "sounds", Name: "sound", Type: "sound", Count: 1},
		{Role: soundsAny, Owner: "sound", Name: "any", Type: "any", Count: 1},
		{Role: soundsSolo, Owner: "sound", Name: "solo", Type: "solo", Count: 1},
		{Role: soundsEnsemble, Owner: "sound", Name: "ensemble", Type: "ensemble", Count: 1},
		{Role: soundsID, Owner: "sound", Name: "id", Type: "xs:ID", Attribute: true, Count: 1},
		{Role: soundsSoundPrimary, Owner: "sound", Name: "primary", Type: "yes-no", Attribute: true, Count: 1},
		{Role: soundsAnyPrimary, Owner: "any", Name: "primary", Type: "yes-no", Attribute: true, Count: 1},
		{Role: soundsSoloPrimary, Owner: "solo", Name: "primary", Type: "yes-no", Attribute: true, Count: 1},
		{Role: soundsEnsemblePrimary, Owner: "ensemble", Name: "primary", Type: "yes-no", Attribute: true, Count: 1},
		{Role: soundsNumber, Owner: "ensemble", Name: "number", Type: "xs:NMTOKEN", Attribute: true, Count: 1},
	}
}

func evaluateContextSoundsMapping(ctx context.Context, s *Session, _ *factNode, key RuleKey, p *contextPlan, final bool) (Decision, bool, error) {
	if !final {
		return Decision{}, true, nil
	}
	r := &contextReader{ctx: ctx, s: s, b: p.Bindings}
	for _, id := range s.sequence {
		if !r.step(1) {
			return Decision{}, false, r.err
		}
		n := s.nodes[id]
		role, primary := SoundMappingRole(""), contextRole("")
		for _, binding := range []struct {
			role    contextRole
			output  SoundMappingRole
			primary contextRole
		}{
			{soundsSound, SoundMappingSound, soundsSoundPrimary},
			{soundsAny, SoundMappingAny, soundsAnyPrimary},
			{soundsSolo, SoundMappingSolo, soundsSoloPrimary},
			{soundsEnsemble, SoundMappingEnsemble, soundsEnsemblePrimary},
		} {
			if r.matches(n, binding.role) {
				role, primary = binding.output, binding.primary
				break
			}
		}
		if r.err != nil {
			return Decision{}, false, r.err
		}
		if role == "" {
			continue
		}
		owner := n
		direction := SoundMappingToMusicXML
		library, number := Evidence[Atomic]{State: EvidenceAbsent}, Evidence[Atomic]{State: EvidenceAbsent}
		if role != SoundMappingSound {
			owner = s.nodes[n.header.Parent]
			if !r.matches(owner, soundsSound) {
				if r.err != nil {
					return Decision{}, false, r.err
				}
				return Decision{}, false, contract("sounds mapping owner", "exact sound parent required")
			}
			direction = SoundMappingFromMusicXML
			library = r.scalar(n)
		}
		if role == SoundMappingEnsemble {
			number = localRawAttribute(r, n, soundsNumber)
		}
		value := SoundMappingContextResult{Role: role, Direction: direction, Sound: NodeRef{s.documentID, owner.header.ID}, SoundID: r.attribute(owner, soundsID), LibraryID: library, Primary: localRawAttribute(r, n, primary), Number: number, Content: r.complete(owner)}
		if r.err != nil {
			return Decision{}, false, r.err
		}
		if err := s.publishContextResult(ctx, ContextResult{Provider: key, Subject: NodeRef{s.documentID, n.header.ID}, Property: ContextPropertySoundMapping, Value: Evidence[Atomic]{State: EvidenceAbsent}, Origins: []Location{n.header.Location}, SoundMapping: &value}); err != nil {
			return Decision{}, false, err
		}
	}
	return Decision{Outcome: OutcomePass}, false, r.err
}

func cloneSoundMappingContext(ctx context.Context, s *Session, p *SoundMappingContextResult) (*SoundMappingContextResult, error) {
	if p == nil {
		return nil, nil
	}
	// The existing bounded graph measurer accounts for every field, string and
	// causal reference before the typed deep copy. No slice/pointer aliases escape.
	if err := contextRetainProjection(ctx, s, *p); err != nil {
		return nil, err
	}
	q := stateClone(*p)
	return &q, nil
}
