package validation

import (
	"context"
	"reflect"
)

// These architecture-independent reserves include each complete fixed record
// once, plus alignment/allocation headroom. Nested evidence/location headers are
// already inside their containing record; only owned pointers and backing data
// add storage. Tests audit these bounds against the closed transport types.
const (
	sourceOriginRecordBytes   int64 = 640
	sourceOriginStepBytes     int64 = 320
	sourceOriginQNameBytes    int64 = 64
	sourceOriginPositionBytes int64 = 64
	sourceOriginCauseBytes    int64 = 96
)

type sourceOriginMeasure struct {
	ctx     context.Context
	session *Session
	bytes   int64
}

func (m *sourceOriginMeasure) add(n int64) error {
	var err error
	m.bytes, err = rationalCheckedAdd(BudgetBufferedBytes, m.bytes, n)
	return err
}

func (m *sourceOriginMeasure) strings(values ...string) error {
	for _, value := range values {
		// Admit copy work in bounded blocks. The fixed traversal work is admitted
		// separately for each record, including nil/empty fields.
		blocks := int64(len(value)) / resourceCopyChunk
		if len(value)%resourceCopyChunk != 0 {
			blocks++
		}
		if err := m.session.step(m.ctx, 1+blocks); err != nil {
			return err
		}
		if value == "" {
			continue
		}
		// Each string is independently cloned, even when contents coincide. Include
		// allocator headroom; no document-identity interning is assumed here.
		n, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(value)), 2)
		if err != nil {
			return err
		}
		if err = m.add(16); err != nil {
			return err
		}
		if err = m.add(n); err != nil {
			return err
		}
	}
	return nil
}

func (m *sourceOriginMeasure) evidence(state EvidenceState, reason Code, cause *DecisionRef) error {
	if err := m.strings(string(state), string(reason)); err != nil {
		return err
	}
	if cause == nil {
		return nil
	}
	if err := m.session.step(m.ctx, 8); err != nil {
		return err
	}
	if err := m.add(sourceOriginCauseBytes); err != nil {
		return err
	}
	return m.strings(string(cause.Rule.ID), cause.Subject.DocumentID)
}

func (m *sourceOriginMeasure) site(in Evidence[Location]) error {
	if err := m.session.step(m.ctx, 16); err != nil {
		return err
	}
	if err := m.evidence(in.State, in.Reason, in.Cause); err != nil {
		return err
	}
	v := in.Value
	if err := m.strings(string(v.Target), v.DocumentID, v.Entry, v.Path); err != nil {
		return err
	}
	if v.Position == nil {
		return nil
	}
	if err := m.add(sourceOriginPositionBytes); err != nil {
		return err
	}
	return m.strings(v.Position.CoordinateSpace)
}

func measureSourceOriginContext(ctx context.Context, s *Session, in *SourceOriginContextResult) (int64, error) {
	m := sourceOriginMeasure{ctx: ctx, session: s, bytes: sourceOriginRecordBytes}
	if err := s.step(ctx, 32); err != nil {
		return 0, err
	}
	if err := m.strings(in.Subject.DocumentID, string(in.Kind)); err != nil {
		return 0, err
	}
	if in.Attribute != nil {
		if err := m.add(sourceOriginQNameBytes); err != nil {
			return 0, err
		}
		if err := m.strings(in.Attribute.Space, in.Attribute.Local); err != nil {
			return 0, err
		}
	}
	if err := m.evidence(in.Specified.State, in.Specified.Reason, in.Specified.Cause); err != nil {
		return 0, err
	}
	if err := m.evidence(in.AncestryComplete.State, in.AncestryComplete.Reason, in.AncestryComplete.Cause); err != nil {
		return 0, err
	}
	for _, site := range []Evidence[Location]{in.Physical, in.Declaration, in.Use} {
		if err := m.site(site); err != nil {
			return 0, err
		}
	}
	ancestry, err := rationalCheckedMul(BudgetBufferedBytes, int64(len(in.Ancestry)), sourceOriginStepBytes)
	if err != nil {
		return 0, err
	}
	if err = m.add(ancestry); err != nil {
		return 0, err
	}
	for _, step := range in.Ancestry {
		if err = s.step(ctx, 8); err != nil {
			return 0, err
		}
		if err = m.strings(step.Entity); err != nil {
			return 0, err
		}
		if err = m.site(step.Declaration); err != nil {
			return 0, err
		}
		if err = m.site(step.Use); err != nil {
			return 0, err
		}
	}
	if _, err = AllocationSize(m.bytes); err != nil {
		return 0, err
	}
	return m.bytes, nil
}

func cloneSourceOriginContext(ctx context.Context, s *Session, in *SourceOriginContextResult) (*SourceOriginContextResult, error) {
	if in == nil {
		return nil, nil
	}
	size, err := measureSourceOriginContext(ctx, s, in)
	if err != nil {
		return nil, err
	}
	if err = s.retain(size); err != nil {
		return nil, err
	}
	// Fill a single owned root after admission. The shared copier checks context
	// throughout all nested sites, causes and bounded string-copy blocks.
	out := new(SourceOriginContextResult)
	if err = resourceCopyInto(ctx, reflect.ValueOf(out).Elem(), reflect.ValueOf(in).Elem(), "", ""); err != nil {
		return nil, err
	}
	return out, nil
}
