package musicxml

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConcurrentTransportAndValidation(t *testing.T) {
	t.Parallel()

	source := []byte(
		`<score-partwise version="4.0">` +
			`<part-list><score-part id="P1">` +
			`<part-name>Piano</part-name></score-part></part-list>` +
			`<part id="P1"><measure number="1">` +
			`<note><rest/><duration>1</duration></note>` +
			`</measure></part></score-partwise>`,
	)
	score, err := DecodeScorePartwise(bytes.NewReader(source))
	require.NoError(t, err)
	require.NoError(t, score.Validate())

	var archive bytes.Buffer
	require.NoError(t, EncodeMXL(&archive, score))
	archiveBytes := append([]byte(nil), archive.Bytes()...)

	const (
		workers    = 16
		iterations = 10
	)
	errors := make(chan error, workers)
	var wait sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for iteration := 0; iteration < iterations; iteration++ {
				var encoded bytes.Buffer
				if err := Encode(&encoded, score); err != nil {
					errors <- fmt.Errorf("encode shared score: %w", err)
					return
				}
				if err := Validate(score); err != nil {
					errors <- fmt.Errorf("validate shared score: %w", err)
					return
				}
				if _, err := DecodeScorePartwise(
					bytes.NewReader(source),
				); err != nil {
					errors <- fmt.Errorf("decode XML: %w", err)
					return
				}
				if _, err := DecodeMXLScorePartwise(
					bytes.NewReader(archiveBytes),
				); err != nil {
					errors <- fmt.Errorf("decode MXL: %w", err)
					return
				}
			}
		}()
	}
	wait.Wait()
	close(errors)

	for err := range errors {
		require.NoError(t, err)
	}
}
