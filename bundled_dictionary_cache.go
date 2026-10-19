package musicxml

import (
	"context"
	"slices"
	"sync"

	"github.com/go-muse/go-musicxml/validation"
)

// bundledDictionaryCache holds the required selections of the bundled source,
// imported and verified once per process. The data is embedded and immutable,
// so every call that selects exactly these components can read one set.
var bundledDictionaryCache struct {
	mu  sync.Mutex
	set *validation.DictionarySet
}

// cachedBundledDictionarySet returns the process-wide set of the required
// bundled selections, importing it on first use. The import runs on its own
// account with the default budgets and without the caller's context: the
// caller pays neither its work nor its cancellation. A failed import is not
// cached and returns nil, so that the caller imports on its own account as
// before; the embedded data makes that unreachable in practice.
func cachedBundledDictionarySet() *validation.DictionarySet {
	bundledDictionaryCache.mu.Lock()
	defer bundledDictionaryCache.mu.Unlock()
	if bundledDictionaryCache.set != nil {
		return bundledDictionaryCache.set
	}
	account, err := validation.NewBudgetAccount(validation.Budgets{})
	if err != nil {
		return nil
	}
	set, err := importDictionarySet(context.Background(), account, requiredDictionaryOptions())
	if err != nil {
		if set != nil {
			_ = set.Close()
		}
		return nil
	}
	bundledDictionaryCache.set = set
	return set
}

// bundledRequiredDictionaries reports whether o selects exactly the required
// relations, in their canonical order, from the bundled source.
func bundledRequiredDictionaries(o DictionaryOptions) bool {
	if _, bundled := o.Source.(bundledDictionarySource); !bundled {
		return false
	}
	return slices.EqualFunc(o.Selections, requiredDictionarySelections(), func(a, b validation.DictionarySelection) bool {
		return a.Manifest == b.Manifest && slices.Equal(a.Relations, b.Relations)
	})
}
