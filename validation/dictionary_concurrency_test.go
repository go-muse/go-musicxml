package validation

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"sync"
	"testing"
)

func TestDictionaryConcurrentImmutableReadsKeepStoreUntilLastSession(t *testing.T) {
	ctx := context.Background()
	a := dictionaryCoreAccount(t)
	selected, input, _ := dictionaryFixture(t, DictionaryManifestSMuFL14Canonical)
	set, err := NewDictionarySet(ctx, a, []DictionarySelection{selected}, []DictionaryImport{input})
	if err != nil {
		t.Fatal(err)
	}
	e, unused := sessionFixture(t, Budgets{})
	_, _ = unused.Finish(ctx, Completion{Syntax: SyntaxIncomplete})
	var sessions []*Session
	for i := 0; i < 24; i++ {
		s, err := e.BeginWithDictionaries(ctx, RunOptions{DocumentID: "reader-" + strconv.Itoa(i), Profile: "test.source", Account: a}, set)
		if err != nil {
			t.Fatal(err)
		}
		sessions = append(sessions, s)
	}
	fingerprint, _ := set.Fingerprint()
	var group sync.WaitGroup
	start := make(chan struct{})
	for _, session := range sessions {
		group.Add(1)
		go func(s *Session) {
			defer group.Done()
			<-start
			for j := 0; j < 32; j++ {
				entry, known, err := s.dictionaryCapability(ctx, selected.Manifest, DictionaryRelationCanonicalName)
				if err != nil || known.State != EvidenceKnown || entry == nil {
					t.Errorf("concurrent capability: %+v %v", known, err)
					break
				}
				if s.dictionaryFingerprint() != fingerprint {
					t.Error("borrow fingerprint changed")
				}
				_, found, err := dictionarySearch(ctx, a, len(entry.smufl.Glyphs), "gClef", func(i int) string { return entry.smufl.Glyphs[i].Name })
				if err != nil || !found {
					t.Errorf("concurrent lookup: %v %v", found, err)
					break
				}
			}
			if _, err := s.Finish(ctx, Completion{Syntax: SyntaxIncomplete}); err != nil {
				t.Errorf("finish: %v", err)
			}
		}(session)
	}
	close(start)
	if err := set.Close(); err != nil {
		t.Fatal(err)
	}
	group.Wait()
	if dictionaryCoreUsage(t, a) != 17 {
		t.Fatal("concurrent readers leaked")
	}
}

func TestDictionaryNilAndUnusedAttachmentPreserveOriginalReportJSON(t *testing.T) {
	ctx := context.Background()
	e, unused := sessionFixture(t, Budgets{})
	_, _ = unused.Finish(ctx, Completion{Syntax: SyntaxIncomplete})
	var baseline []byte
	for _, mode := range []string{"old", "nil", "empty"} {
		t.Run(mode, func(t *testing.T) {
			a := dictionaryCoreAccount(t)
			opts := RunOptions{DocumentID: "doc", Profile: "test.source", Account: a}
			var set *DictionarySet
			var s *Session
			var err error
			if mode == "empty" {
				set, err = NewDictionarySet(ctx, a, nil, nil)
				if err != nil {
					t.Fatal(err)
				}
			}
			if mode == "old" {
				s, err = e.Begin(ctx, opts)
			} else {
				s, err = e.BeginWithDictionaries(ctx, opts, set)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Open(ctx, factHeader(1, 0, "test.root", "root")); err != nil {
				t.Fatal(err)
			}
			if err := s.Close(ctx, factEnd(1)); err != nil {
				t.Fatal(err)
			}
			report, err := s.Finish(ctx, Completion{InputComplete: true, Syntax: SyntaxWellFormed})
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "old" {
				baseline = encoded
			} else if string(encoded) != string(baseline) {
				t.Fatalf("old JSON changed:\n%s\n%s", baseline, encoded)
			}
			if set != nil {
				_ = set.Close()
			}
			if dictionaryCoreUsage(t, a) != 17 {
				t.Fatal("unused capability leaked")
			}
		})
	}
	var zero DictionarySet
	if err := zero.Close(); !errors.Is(err, ErrInvalidConfiguration) {
		t.Fatalf("zero close: %v", err)
	}
	var absent *DictionarySet
	if err := absent.Close(); err != nil {
		t.Fatalf("nil close: %v", err)
	}
}
