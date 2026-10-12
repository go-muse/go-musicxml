package validation

import "context"

// TimingPlayScope distinguishes a note-local encoding from a subsequent-note update.
type TimingPlayScope string

const (
	// TimingPlayCurrentNote affects only the containing note.
	TimingPlayCurrentNote TimingPlayScope = "current-note"
	// TimingPlayFollowingNotes changes subsequent notes in established musical order.
	TimingPlayFollowingNotes TimingPlayScope = "following-notes"
)

// TimingPlayTechnique retains one exact encoded performance technique.
type TimingPlayTechnique struct {
	Kind        Code
	Value, Type Evidence[string]
}

// TimingPlayInstruction is a typed raw instruction, before navigation pass selection.
type TimingPlayInstruction struct {
	Source     NodeRef
	Instrument Evidence[NodeRef]
	Scope      TimingPlayScope
	Value      Evidence[[]TimingPlayTechnique]
}

func performancePlayInstructions(ctx context.Context, s *Session, raw []playbackInstruction) ([]TimingPlayInstruction, error) {
	if err := stateRetainCount(s, len(raw), 1024); err != nil {
		return nil, err
	}
	out := make([]TimingPlayInstruction, len(raw))
	for i, v := range raw {
		if err := s.step(ctx, 1); err != nil {
			return nil, err
		}
		p := TimingPlayInstruction{Source: v.Source, Instrument: cloneEvidence(v.Instrument), Scope: TimingPlayCurrentNote, Value: stateUnavailable[playState, []TimingPlayTechnique](v.Value)}
		if v.Scope == playbackFollowingNotes {
			p.Scope = TimingPlayFollowingNotes
		}
		if v.Value.State == EvidenceKnown {
			if err := stateRetainCount(s, len(v.Value.Value.Techniques), 512); err != nil {
				return nil, err
			}
			p.Value.Value = make([]TimingPlayTechnique, len(v.Value.Value.Techniques))
			for j, t := range v.Value.Value.Techniques {
				if err := s.step(ctx, 1); err != nil {
					return nil, err
				}
				kind := Code("other")
				switch t.Kind {
				case playIPA:
					kind = "ipa"
				case playMute:
					kind = "mute"
				case playSemiPitched:
					kind = "semi-pitched"
				}
				p.Value.Value[j] = TimingPlayTechnique{Kind: kind, Value: cloneEvidence(t.Value), Type: cloneEvidence(t.Type)}
			}
		}
		p.Value = cloneEvidence(p.Value)
		out[i] = p
	}
	return out, nil
}
func cloneTimingPlayInstructions(v []TimingPlayInstruction) []TimingPlayInstruction {
	out := make([]TimingPlayInstruction, len(v))
	for i, p := range v {
		p.Instrument = cloneEvidence(p.Instrument)
		p.Value = cloneEvidence(p.Value)
		p.Value.Value = make([]TimingPlayTechnique, len(v[i].Value.Value))
		for j, t := range v[i].Value.Value {
			t.Value = cloneEvidence(t.Value)
			t.Type = cloneEvidence(t.Type)
			p.Value.Value[j] = t
		}
		out[i] = p
	}
	return out
}
