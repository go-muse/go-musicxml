package validation

import (
	"context"
	"math/big"
)

// Exact reference arithmetic does not quantize or clamp to a hardware velocity.
func performanceVelocity(ctx context.Context, s *Session, d Evidence[Decimal]) (Evidence[Rational], error) {
	if err := s.step(ctx, 1); err != nil {
		return Evidence[Rational]{}, err
	}
	if d.State != EvidenceKnown {
		return stateUnavailable[Decimal, Rational](d), nil
	}
	r, err := s.rationalFromDecimal(ctx, d.Value)
	if err != nil {
		return Evidence[Rational]{}, err
	}
	r, err = s.rationalMul(ctx, r, performanceRatio(9, 10))
	return Evidence[Rational]{State: EvidenceKnown, Value: r}, err
}
func performanceRatio(n, d int64) Rational {
	v := big.NewRat(n, d)
	return rationalOwned(v, v.RatString())
}

// Dots and time-modification deliberately do not participate: this is a swing
// applicability test against nominal undotted type, not notation conformance.
func performanceSwingEligible(ctx context.Context, s *Session, typ, active Evidence[string], duration Evidence[Rational], grace Evidence[bool]) (Evidence[bool], error) {
	if err := s.step(ctx, 1); err != nil {
		return Evidence[bool]{}, err
	}
	if grace.State == EvidenceKnown && grace.Value || typ.State == EvidenceAbsent {
		return Evidence[bool]{State: EvidenceKnown, Value: false}, nil
	}
	if grace.State != EvidenceKnown {
		return stateUnavailable[bool, bool](grace), nil
	}
	if typ.State != EvidenceKnown {
		return stateUnavailable[string, bool](typ), nil
	}
	if active.State != EvidenceKnown {
		return stateUnavailable[string, bool](active), nil
	}
	if err := s.step(ctx, int64(len(typ.Value)+len(active.Value)+1)); err != nil {
		return Evidence[bool]{}, err
	}
	if typ.Value != active.Value {
		return Evidence[bool]{State: EvidenceKnown, Value: false}, nil
	}
	if duration.State != EvidenceKnown {
		return stateUnavailable[Rational, bool](duration), nil
	}
	types := [...]string{"maxima", "long", "breve", "whole", "half", "quarter", "eighth", "16th", "32nd", "64th", "128th", "256th", "512th", "1024th"}
	for i, t := range types {
		if typ.Value == t {
			n, d := int64(32), int64(1)
			if i <= 5 {
				n >>= i
			} else {
				n = 1
				d <<= i - 5
			}
			cmp, err := s.rationalCmp(ctx, duration.Value, performanceRatio(n, d))
			return Evidence[bool]{State: EvidenceKnown, Value: cmp == 0}, err
		}
	}
	return Evidence[bool]{State: EvidenceUnsupported, Reason: "swing-note-type-unavailable"}, nil
}
