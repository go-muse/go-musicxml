package musicxml

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestLegacyValidationBudgetPolicy(t *testing.T) {
	limits := legacyValidationBudgets()
	fields := reflect.ValueOf(limits)
	for i := 0; i < fields.NumField(); i++ {
		name := fields.Type().Field(i).Name
		want := int64(math.MaxInt64)
		if name == "MaxDepth" {
			want = maximumDocumentDepth
		}
		if got := fields.Field(i).Int(); got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
	account, err := validation.NewBudgetAccount(limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := account.Charge(validation.BudgetRuleSteps, math.MaxInt64); err != nil {
		t.Fatal(err)
	}
	if err := account.Charge(validation.BudgetRuleSteps, 1); !errors.Is(err, validation.ErrResourceLimit) {
		t.Fatalf("legacy accounting overflow was not rejected: %v", err)
	}
	if strconv.IntSize == 32 {
		hostMax := int64(^uint(0) >> 1)
		if _, err := validation.AllocationSize(hostMax + 1); !errors.Is(err, validation.ErrResourceLimit) {
			t.Fatalf("host allocation overflow was not rejected: %v", err)
		}
	}
}

func TestValidateLegacyLargeScore(t *testing.T) {
	// Repeated independent occurrences exercise the facade's old size/work
	// contract without committing or changing an upstream fixture.
	score := NewScorePartwise(PartDefinition{ID: "P1", Name: "Large score"})
	for i := 0; i < 3000; i++ {
		measure := NewScorePartwiseMeasure(strconv.Itoa(i + 1))
		measure.AddNote(NewRestNote(1))
		score.Part[0].AddMeasure(measure)
	}
	if err := Validate(score); err != nil {
		t.Errorf("legacy validation rejected a 3000-measure score: %v", err)
	}
	// The new assessment API remains explicitly bounded. Raising its exact
	// relevant ceilings admits the same model; a caller's small quota still wins.
	limits := validation.Budgets{MaxBufferedBytes: math.MaxInt64, MaxRuleSteps: math.MaxInt64}
	report, err := AssessModel(context.Background(), score, AssessmentOptions{Profile: ProfileModelXSD, Budgets: limits})
	if err != nil || !report.Satisfied() {
		t.Fatalf("explicit large assessment budgets: satisfied=%t counts=%+v err=%v", report.Satisfied(), report.Counts, err)
	}
	limits.MaxNodes = 100
	report, err = AssessModel(context.Background(), score, AssessmentOptions{Profile: ProfileModelXSD, Budgets: limits})
	if !errors.Is(err, validation.ErrResourceLimit) || report.Satisfied() || !errors.Is(err, validation.ErrIncompleteAssessment) {
		t.Fatalf("explicit small assessment budget ignored: satisfied=%t err=%v", report.Satisfied(), err)
	}
}

func TestValidateLegacyLargeCurrentText(t *testing.T) {
	score := NewScorePartwise(PartDefinition{ID: "P1", Name: strings.Repeat("a", (1<<20)+1)})
	measure := NewScorePartwiseMeasure("1")
	measure.AddNote(NewRestNote(1))
	score.Part[0].AddMeasure(measure)
	if err := Validate(score); err != nil {
		t.Fatalf("legacy validation added a scalar ceiling: %v", err)
	}
	report, err := AssessModel(context.Background(), score, AssessmentOptions{Profile: ProfileModelXSD})
	if !errors.Is(err, validation.ErrResourceLimit) || report.Satisfied() {
		t.Fatalf("default bounded assessment lost its scalar ceiling: satisfied=%t err=%v", report.Satisfied(), err)
	}
}
