package validation

import (
	"context"
	"unsafe"
)

// ContextFamilies selects the context families that a run keeps in
// Report.Context. The zero value keeps none. Providers compute and check every
// context whatever the selection; the session only keeps no copy of an
// unselected record, which saves the storage it would hold. Within budgets
// that admit both runs, every decision, count and finding is the same, and a
// selection keeps exactly the records of the complete projection whose family
// it names, in the same order. A smaller selection can therefore complete
// where a larger one exceeds MaxBufferedBytes.
type ContextFamilies uint16

const (
	// ContextFamilyTiming keeps the "timing" records.
	ContextFamilyTiming ContextFamilies = 1 << iota
	// ContextFamilyLayout keeps the effective local and layout properties
	// ("print-*", "layout.*" and the other properties with a Layout payload).
	ContextFamilyLayout
	// ContextFamilySpans keeps the "span" records and the "span.*"
	// properties.
	ContextFamilySpans
	// ContextFamilyRepeats keeps the "graphical-repeat" records.
	ContextFamilyRepeats
	// ContextFamilyNavigation keeps the "navigation" and
	// "navigation-traversal" records.
	ContextFamilyNavigation
	// ContextFamilyPerformance keeps the "performance-*" properties.
	ContextFamilyPerformance
	// ContextFamilySounds keeps the "sounds.mapping" records.
	ContextFamilySounds
	// ContextFamilyResources keeps the "resource.*" records.
	ContextFamilyResources
	// ContextFamilyContainer keeps the "container.metadata" records.
	ContextFamilyContainer
	// ContextFamilySourceOrigins keeps the "source-origin" records.
	ContextFamilySourceOrigins
	// ContextFamilyDictionaries keeps the "dictionary.*" records.
	ContextFamilyDictionaries
	contextFamilyEnd

	// ContextFamilyAll keeps the complete projection.
	ContextFamilyAll = contextFamilyEnd - 1
)

// Family returns the single family of the record's property, or zero for a
// property that no session publishes.
func (r ContextResult) Family() ContextFamilies {
	return contextFamilyOf(r.Property)
}

func contextFamilyOf(p ContextProperty) ContextFamilies {
	switch p {
	case ContextPropertyTiming:
		return ContextFamilyTiming
	case ContextPropertySpan, ContextPropertySpanLyricSyllable:
		return ContextFamilySpans
	case ContextPropertyRepeat:
		return ContextFamilyRepeats
	case ContextPropertyNavigation, ContextPropertyNavigationTraversal:
		return ContextFamilyNavigation
	case ContextPropertySoundMapping:
		return ContextFamilySounds
	case ContextPropertyContainerMetadata:
		return ContextFamilyContainer
	case ContextPropertySourceOrigin:
		return ContextFamilySourceOrigins
	}
	switch {
	case dictionaryPropertyValid(p):
		return ContextFamilyDictionaries
	case resourcePropertyValid(p):
		return ContextFamilyResources
	case contextLocalPropertyValid(p):
		return ContextFamilyLayout
	case contextSpanPropertyValid(p):
		return ContextFamilySpans
	case contextPerformancePropertyValid(p):
		return ContextFamilyPerformance
	}
	return 0
}

// resourceContextFamilies holds every family of a property that a resource
// snapshot copies (resourceSelectedContext).
const resourceContextFamilies = ContextFamilySourceOrigins | ContextFamilyDictionaries | ContextFamilyContainer | ContextFamilyResources | ContextFamilySounds | ContextFamilyLayout

// resourceSnapshotRun prepares a run that finishes with
// FinishWithResourceSnapshot. Its session also keeps the unselected records
// that the snapshot copies, outside Report.Context.
func resourceSnapshotRun(opts RunOptions) RunOptions {
	opts.resourceSnapshot = true
	return opts
}

// hiddenContext is a record kept only for a resource snapshot. at is the
// length of the published projection when it was kept, so the snapshot sees
// the records in the order of the complete projection.
type hiddenContext struct {
	at     int
	result ContextResult
}

var hiddenContextBytes = int64(unsafe.Sizeof(hiddenContext{}))

// keepHiddenContext stores a validated, owned record that the report does not
// select. The backing array grows by doubling under the same accounting as
// growContextResults; no rollback restores an earlier header, so the replaced
// array's charge is released at once.
func (s *Session) keepHiddenContext(out ContextResult) (*ContextResult, error) {
	if len(s.hiddenContexts) == cap(s.hiddenContexts) {
		old := cap(s.hiddenContexts)
		grown := max(8, 2*old)
		if err := s.retain(allocationSlack(int64(grown) * hiddenContextBytes)); err != nil {
			return nil, err
		}
		next := make([]hiddenContext, len(s.hiddenContexts), grown)
		copy(next, s.hiddenContexts)
		s.hiddenContexts = next
		if old != 0 {
			released := allocationSlack(int64(old) * hiddenContextBytes)
			_ = s.account.Release(BudgetBufferedBytes, released)
			s.retained -= released
		}
	}
	s.hiddenContexts = append(s.hiddenContexts, hiddenContext{at: len(s.contextResults), result: out})
	return &s.hiddenContexts[len(s.hiddenContexts)-1].result, nil
}

// snapshotContexts yields the records a resource snapshot copies, in the
// order of the complete projection, checking ctx at every record.
func (s *Session) snapshotContexts(ctx context.Context, yield func(*ContextResult) error) error {
	hidden := 0
	emitHidden := func(upTo int) error {
		for ; hidden < len(s.hiddenContexts) && s.hiddenContexts[hidden].at <= upTo; hidden++ {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := yield(&s.hiddenContexts[hidden].result); err != nil {
				return err
			}
		}
		return nil
	}
	for i := range s.contextResults {
		if err := emitHidden(i); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if !resourceSelectedContext(s.contextResults[i].Property) {
			continue
		}
		if err := yield(&s.contextResults[i]); err != nil {
			return err
		}
	}
	return emitHidden(len(s.contextResults))
}
