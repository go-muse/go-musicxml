package musicxml

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"math"

	"github.com/go-muse/go-musicxml/validation"
)

// assessmentResourceBody owns only returned stream bytes. Every newly returned
// stream is read and verified, even when its claimed pin matches a cached parse.
// The caller keeps this lease live while any parser/archive borrows these bytes.
type assessmentResourceBody struct {
	account  *validation.Account
	bytes    []byte
	digest   string
	retained int64
}

func (b *assessmentResourceBody) close() {
	if b == nil || b.account == nil {
		return
	}
	_ = b.account.Release(validation.BudgetBufferedBytes, b.retained)
	b.bytes = nil
	b.digest = ""
	b.retained = 0
	b.account = nil
}

// readAssessmentResourceBody takes ownership of body immediately and closes it
// exactly once on every path. Caller I/O must implement its own cancellation of
// a blocked Read. This helper imposes no transport/scheme/redirect authority.
func readAssessmentResourceBody(ctx context.Context, a *validation.Account, body io.ReadCloser) (out *assessmentResourceBody, err error) {
	if body == nil {
		return nil, &validation.ConfigurationError{Field: "resource body", Reason: "non-nil owned stream required"}
	}
	var owned *assessmentResourceBody
	defer func() {
		err = errors.Join(err, body.Close())
		if err != nil {
			if owned != nil {
				owned.close()
			}
			out = nil
		}
	}()
	if ctx == nil || a == nil {
		return nil, &validation.ConfigurationError{Field: "resource body", Reason: "context and root account required"}
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	const scratchBytes int64 = 4096 + 256
	if err = a.Charge(validation.BudgetBufferedBytes, scratchBytes); err != nil {
		return nil, err
	}
	defer a.Release(validation.BudgetBufferedBytes, scratchBytes)
	if err = a.Charge(validation.BudgetBufferedBytes, 256); err != nil {
		return nil, err
	}
	owned = &assessmentResourceBody{account: a, retained: 256}
	scratch := make([]byte, 4096)
	digest := sha256.New()
	emptyReads := 0
	for {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		// One unit admits one <=4096-byte read, append and digest operation,
		// including empty attempts and the bounded over-limit probe.
		if err = a.Charge(validation.BudgetRuleSteps, 1); err != nil {
			return nil, err
		}
		used, usageErr := a.Usage(validation.BudgetExternalBytes)
		if usageErr != nil {
			return nil, usageErr
		}
		limit := a.Limits().MaxExternalBytes
		remaining := limit - used
		size := len(scratch)
		if remaining < int64(size) {
			size = int(remaining) + 1
		}
		n, readErr := body.Read(scratch[:size])
		if n < 0 || n > size {
			return nil, errors.Join(&validation.ConfigurationError{Field: "resource reader", Reason: "invalid read count"}, readErr)
		}
		if n > 0 {
			emptyReads = 0
			if err = a.Charge(validation.BudgetExternalBytes, min(int64(n), remaining)); err != nil {
				return nil, errors.Join(err, readErr)
			}
			var byteErr error
			if int64(n) > remaining {
				byteErr = &validation.ResourceError{Kind: validation.BudgetExternalBytes, Limit: limit, Used: limit, Requested: 1}
			}
			if byteErr != nil {
				return nil, errors.Join(byteErr, readErr)
			}
			if int64(len(owned.bytes)) > math.MaxInt64-int64(n) {
				return nil, &validation.ConfigurationError{Field: "resource bytes", Reason: "size overflow"}
			}
			need := int64(len(owned.bytes)) + int64(n)
			if need > int64(cap(owned.bytes)) {
				capacity := need
				if old := int64(cap(owned.bytes)); old <= math.MaxInt64/2 && old*2 > capacity {
					capacity = old * 2
				}
				size, sizeErr := validationAllocationSize(capacity)
				if sizeErr != nil {
					return nil, sizeErr
				}
				if err = a.Charge(validation.BudgetBufferedBytes, capacity); err != nil {
					return nil, errors.Join(err, readErr)
				}
				next := make([]byte, len(owned.bytes), size)
				// Both capacities stay admitted until the relocation completes. Each
				// charged unit copies at most one bounded scratch-sized block.
				for at := 0; at < len(owned.bytes); {
					if copyErr := ctx.Err(); copyErr != nil {
						_ = a.Release(validation.BudgetBufferedBytes, capacity)
						return nil, errors.Join(copyErr, readErr)
					}
					if copyErr := a.Charge(validation.BudgetRuleSteps, 1); copyErr != nil {
						_ = a.Release(validation.BudgetBufferedBytes, capacity)
						return nil, errors.Join(copyErr, readErr)
					}
					end := resourceBodyChunkEnd(at, len(owned.bytes))
					copy(next[at:end], owned.bytes[at:end])
					at = end
				}
				old := int64(cap(owned.bytes))
				owned.bytes = next
				owned.retained += capacity - old
				_ = a.Release(validation.BudgetBufferedBytes, old)
			}
			owned.bytes = append(owned.bytes, scratch[:n]...)
			_, _ = digest.Write(scratch[:n])
		} else {
			emptyReads++
		}
		if err = ctx.Err(); err != nil {
			return nil, errors.Join(err, readErr)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, readErr
		}
		if emptyReads >= 100 {
			return nil, io.ErrNoProgress
		}
	}
	owned.digest = hex.EncodeToString(digest.Sum(nil))
	return owned, nil
}

// resourceBodyChunkEnd cannot overflow even at the maximum native slice index.
func resourceBodyChunkEnd(at, length int) int { return at + min(4096, length-at) }
