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

// This finite table contains only literal semitone/type/velocity constants;
// runtime arithmetic never allocates constants before quota admission.
var performanceRatioConstants = func() map[[2]int64]Rational {
	out := make(map[[2]int64]Rational)
	add := func(n, d int64) { v := big.NewRat(n, d); out[[2]int64{n, d}] = rationalOwned(v, v.RatString()) }
	for n := int64(-12); n <= 32; n++ {
		add(n, 1)
	}
	for d := int64(2); d <= 256; d *= 2 {
		add(1, d)
	}
	add(9, 10)
	return out
}()

func performanceRatio(n, d int64) Rational { return performanceRatioConstants[[2]int64{n, d}] }

// Dots and time-modification deliberately do not participate: this is a swing
// applicability test against nominal undotted type, not notation conformance.
func performanceSwingEligible(ctx context.Context, s *Session, typ, active Evidence[string], duration Evidence[Rational], grace Evidence[bool]) (Evidence[bool], error) {
	if err := s.step(ctx, 1); err != nil {
		return Evidence[bool]{}, err
	}
	no := Evidence[bool]{State: EvidenceKnown, Value: false}
	if grace.State == EvidenceKnown && grace.Value || typ.State == EvidenceAbsent {
		return no, nil
	}
	if typ.State == EvidenceKnown && active.State == EvidenceKnown {
		if err := s.step(ctx, int64(len(typ.Value)+len(active.Value)+1)); err != nil {
			return Evidence[bool]{}, err
		}
		if typ.Value != active.Value {
			return no, nil
		}
	}
	nominal := Evidence[bool]{State: EvidenceUnsupported, Reason: "swing-note-type-unavailable"}
	if typ.State == EvidenceKnown && duration.State == EvidenceKnown {
		types := [...]string{"maxima", "long", "breve", "whole", "half", "quarter", "eighth", "16th", "32nd", "64th", "128th", "256th", "512th", "1024th"}
		for i, t := range types {
			if err := s.step(ctx, 1+int64(len(typ.Value))); err != nil {
				return Evidence[bool]{}, err
			}
			if typ.Value == t {
				n, d := int64(32), int64(1)
				if i <= 5 {
					n >>= i
				} else {
					n = 1
					d <<= i - 5
				}
				cmp, err := s.rationalCmp(ctx, duration.Value, performanceRatio(n, d))
				if err != nil {
					return Evidence[bool]{}, err
				}
				nominal = Evidence[bool]{State: EvidenceKnown, Value: cmp == 0}
				break
			}
		}
		if nominal.State == EvidenceKnown && !nominal.Value {
			return no, nil
		}
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
	if duration.State != EvidenceKnown {
		return stateUnavailable[Rational, bool](duration), nil
	}
	return nominal, nil
}
