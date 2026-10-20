package validation

import (
	"context"
	"iter"
)

// Resources yields each distinct canonical intrinsic assessment with its
// report, in the order of ResourceReport. Each report is a fresh copy.
func (b AssessmentBundle) Resources() iter.Seq2[ResourceAssessmentID, Report] {
	return func(yield func(ResourceAssessmentID, Report) bool) {
		for i := range b.ResourceCount() {
			id, report, _ := b.ResourceReport(i)
			if !yield(id, report) {
				return
			}
		}
	}
}

// Adoptions yields the canonical assessment identity and the occurrence
// document ID of each adoption, in the order of Adoption.
func (b AssessmentBundle) Adoptions() iter.Seq2[ResourceAssessmentID, string] {
	return func(yield func(ResourceAssessmentID, string) bool) {
		for i := range b.AdoptionCount() {
			id, document, _ := b.Adoption(i)
			if !yield(id, document) {
				return
			}
		}
	}
}

// AdoptionContexts yields an owned copy of each adoption's context, in the
// order of AdoptionContext.
func (b AssessmentBundle) AdoptionContexts() iter.Seq[ResourceAdoptionContext] {
	return func(yield func(ResourceAdoptionContext) bool) {
		for i := range b.AdoptionCount() {
			adoption, _ := b.AdoptionContext(i)
			if !yield(adoption) {
				return
			}
		}
	}
}

// Contexts yields an owned copy of each selected context result, as ContextAt
// does, admitting each copy against the live Account. A failure, such as
// cancellation, a resource limit or a closed snapshot, is yielded once with a
// zero result and ends the sequence.
func (s *ResourceSnapshot) Contexts(ctx context.Context) iter.Seq2[ContextResult, error] {
	return snapshotSequence(ctx, s.ContextAt)
}

// IDs yields an owned copy of each schema ID occurrence, as IDAt does, with
// the failure handling of Contexts.
func (s *ResourceSnapshot) IDs(ctx context.Context) iter.Seq2[ResourceIDFact, error] {
	return snapshotSequence(ctx, s.IDAt)
}

func snapshotSequence[T any](ctx context.Context, at func(context.Context, int) (T, bool, error)) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for i := 0; ; i++ {
			value, ok, err := at(ctx, i)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			if !ok || !yield(value, nil) {
				return
			}
		}
	}
}
