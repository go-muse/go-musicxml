package validation

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestResourceMediaParameterIdentityAndDuplicates(t *testing.T) {
	for _, tc := range []struct {
		name, value    string
		state          EvidenceState
		representation ResourceRepresentation
	}{
		{"xml-unknown", `application/vnd.recordare.musicxml+xml; extension=value`, EvidenceKnown, ResourceSourceXML},
		{"archive-unknown", `application/vnd.recordare.musicxml; extension=value`, EvidenceKnown, ResourceRawMXL},
		{"archive-charset", `application/vnd.recordare.musicxml; charset=utf-8`, EvidenceKnown, ResourceRawMXL},
		{"quoted-delimiters", `application/vnd.recordare.musicxml+xml; extension="a;b=c"; other="x\\y"`, EvidenceKnown, ResourceSourceXML},
		{"escaped-quote", `application/vnd.recordare.musicxml+xml; foo="a\";b"; bar=c`, EvidenceKnown, ResourceSourceXML},
		{"escaped-backslash", `application/vnd.recordare.musicxml+xml; foo="a\\;b"; bar=c`, EvidenceKnown, ResourceSourceXML},
		{"tab-separator", "application/vnd.recordare.musicxml+xml;\tfoo=bar", EvidenceKnown, ResourceSourceXML},
		{"extended-distinct", `application/vnd.recordare.musicxml+xml; foo=a; foo*=utf-8''b`, EvidenceKnown, ResourceSourceXML},
		{"continuations", `application/vnd.recordare.musicxml+xml; foo*0=a; foo*1=b`, EvidenceKnown, ResourceSourceXML},
		{"continuation-duplicate", `application/vnd.recordare.musicxml+xml; foo*0=a; FOO*0=a`, EvidenceUnknown, ""},
		{"equal-duplicate", `application/vnd.recordare.musicxml+xml; charset=utf-8; charset=utf-8`, EvidenceUnknown, ""},
		{"case-duplicate", `application/vnd.recordare.musicxml; Extension=value; extension=value`, EvidenceUnknown, ""},
		{"conflicting-duplicate", `application/vnd.recordare.musicxml+xml; charset=utf-8; charset=us-ascii`, EvidenceUnknown, ""},
		{"binary-duplicate", `image/png; foo=bar; FOO=bar`, EvidenceUnknown, ""},
		{"trailing-semicolon", `application/vnd.recordare.musicxml+xml;`, EvidenceUnknown, ""},
		{"unicode-space", "application/vnd.recordare.musicxml+xml;\u00a0foo=bar", EvidenceUnknown, ""},
		{"unicode-value", `application/vnd.recordare.musicxml+xml; foo="é"`, EvidenceUnknown, ""},
		{"quoted-control", "application/vnd.recordare.musicxml+xml; foo=\"\x00\"", EvidenceUnknown, ""},
		{"malformed", `application/vnd.recordare.musicxml+xml; extension`, EvidenceUnknown, ""},
		{"generic-xml", `application/xml; extension=value`, EvidenceUnknown, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account, err := NewBudgetAccount(Budgets{})
			if err != nil {
				t.Fatal(err)
			}
			got, err := ClassifyResourceMediaType(context.Background(), account, tc.value)
			if err != nil || got.State != tc.state || got.Value != tc.representation {
				t.Fatalf("got=%+v error=%v want=%s/%s", got, err, tc.state, tc.representation)
			}
			if used, _ := account.Usage(BudgetBufferedBytes); used != 0 {
				t.Fatalf("retained scratch=%d", used)
			}
		})
	}
}

type mediaParameterCancelContext struct {
	context.Context
	calls, stop int
}

func (c *mediaParameterCancelContext) Err() error {
	c.calls++
	if c.stop > 0 && c.calls >= c.stop {
		return context.Canceled
	}
	return nil
}

func TestResourceMediaParametersAdmissionAndCancellation(t *testing.T) {
	value := "application/vnd.recordare.musicxml+xml"
	for i := 0; i < 100; i++ {
		value += fmt.Sprintf("; p%d=\"value;=%d\"", i, i)
	}
	account, _ := NewBudgetAccount(Budgets{})
	counter := &mediaParameterCancelContext{Context: context.Background()}
	got, err := ClassifyResourceMediaType(counter, account, value)
	if err != nil || got.State != EvidenceKnown {
		t.Fatalf("baseline=%+v error=%v", got, err)
	}
	steps, _ := account.Usage(BudgetRuleSteps)
	scratch := int64(len(value))*128 + 4096
	for _, tc := range []struct {
		name    string
		budgets Budgets
		limited bool
	}{
		{"exact", Budgets{MaxRuleSteps: steps, MaxBufferedBytes: scratch}, false},
		{"one-short-work", Budgets{MaxRuleSteps: steps - 1, MaxBufferedBytes: scratch}, true},
		{"one-short-storage", Budgets{MaxRuleSteps: steps, MaxBufferedBytes: scratch - 1}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := NewBudgetAccount(tc.budgets)
			got, err := ClassifyResourceMediaType(context.Background(), a, value)
			if tc.limited {
				if !errors.Is(err, ErrResourceLimit) || got.State == EvidenceKnown {
					t.Fatalf("got=%+v error=%v", got, err)
				}
			} else if err != nil || got.State != EvidenceKnown {
				t.Fatalf("got=%+v error=%v", got, err)
			}
			if used, _ := a.Usage(BudgetBufferedBytes); used != 0 {
				t.Fatal(used)
			}
		})
	}
	for stop := 1; stop <= counter.calls; stop++ {
		a, _ := NewBudgetAccount(Budgets{})
		c := &mediaParameterCancelContext{Context: context.Background(), stop: stop}
		got, err := ClassifyResourceMediaType(c, a, value)
		if !errors.Is(err, context.Canceled) || got.State == EvidenceKnown {
			t.Fatalf("stop=%d got=%+v error=%v", stop, got, err)
		}
		if used, _ := a.Usage(BudgetBufferedBytes); used != 0 {
			t.Fatalf("stop=%d retained=%d", stop, used)
		}
	}
}
