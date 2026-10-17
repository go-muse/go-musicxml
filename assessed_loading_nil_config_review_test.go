package musicxml

import (
	"context"
	"errors"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestSourceAssessmentRejectsNilConfigurationBeforeRead(t *testing.T) {
	for _, converting := range []bool{false, true} {
		for _, configured := range []bool{false, true} {
			input := &assessmentCountingReader{}
			var run *sourceAssessmentRun
			var err error
			if configured {
				run, err = newSourceAssessmentRunWithConfig(context.Background(), input, DecodeOptions{}, nil, converting)
			} else {
				run, err = newSourceAssessmentRun(context.Background(), input, DecodeOptions{}, nil, converting)
			}
			if run != nil {
				_, _ = run.finish(err)
			}
			var configuration *validation.ConfigurationError
			if !errors.As(err, &configuration) || run != nil || input.reads != 0 {
				t.Fatalf("converting=%t configured=%t run=%p reads=%d error=%v; want configuration rejection before input", converting, configured, run, input.reads, err)
			}
		}
	}
}
