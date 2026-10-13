package musicxml

import (
	"context"
	"errors"
	"github.com/go-muse/go-musicxml/validation"
	"io"
)

// resourceIntrinsicResult separates the always-returned best report from the
// optional transferable certificate. Source digest is the existing observer's
// exact original-byte digest, not a serialization or duplicate hashing pass.
type resourceIntrinsicResult struct {
	Report         validation.Report
	Snapshot       *validation.ResourceSnapshot
	Digest         [32]byte
	DigestComplete bool
	// XMLDepth is the peak represented element depth, including scalar leaves.
	// It certifies a maximum only when Err is nil; interrupted walks are partial.
	XMLDepth int
	Err      error
}

func assessResourceSource(ctx context.Context, reader io.Reader, config sourceAssessmentConfig) (out resourceIntrinsicResult) {
	run, err := newSourceAssessmentRunWithConfig(ctx, reader, DecodeOptions{}, &config, false)
	if err != nil {
		if run != nil {
			out.Report = run.startupReport
		}
		out.Err = err
		return
	}
	inputErr := run.drain()
	summary := run.source.lexical.Summary()
	out.XMLDepth = run.source.lexical.peakDepth
	out.Digest, out.DigestComplete = summary.ReadPrefixSHA256, summary.DigestComplete
	completion, err := run.finishInput(inputErr)
	if err != nil {
		out.Err = errors.Join(inputErr, err)
		return
	}
	out.Report, out.Snapshot, err = run.session.FinishWithResourceSnapshot(ctx, completion)
	out.Err = errors.Join(inputErr, err)
	return
}
func assessResourceModel(ctx context.Context, document Document, config assessmentRunConfig) (out resourceIntrinsicResult) {
	run, err := newModelAssessmentRun(ctx, document, config)
	if err != nil {
		out.Err = err
		return
	}
	status, walkErr := run.walk(modelWalkOptions{})
	out.XMLDepth = status.XMLDepth
	completion, err := run.finishInput(walkErr)
	if err != nil {
		out.Err = errors.Join(walkErr, err)
		return
	}
	out.Report, out.Snapshot, err = run.session.FinishWithResourceSnapshot(ctx, completion)
	out.Err = errors.Join(walkErr, err)
	return
}
