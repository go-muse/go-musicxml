package musicxml

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

// DecodeMXLAssessed builds its model from the DTD infoset that its source
// stage assessed, as DecodeAssessed does: DTD defaults reach the model and
// internal-subset entities expand.
func TestDecodeMXLAssessedModelFromTheAssessedInfoset(t *testing.T) {
	body := func(partName string) string {
		return `<score-partwise><part-list><score-part id="P1"><part-name>` + partName + `</part-name></score-part></part-list>` +
			`<part id="P1"><measure number="1"><attributes><divisions>1</divisions></attributes><note><rest/><duration>1</duration></note></measure></part></score-partwise>`
	}
	for name, tc := range map[string]struct{ doctype, partName string }{
		"bundled DTD defaults":            {`<!DOCTYPE score-partwise PUBLIC "-//Recordare//DTD MusicXML 4.0 Partwise//EN" "http://www.musicxml.org/dtds/partwise.dtd">`, "Piano"},
		"bundled DTD and internal entity": {`<!DOCTYPE score-partwise PUBLIC "-//Recordare//DTD MusicXML 4.0 Partwise//EN" "http://www.musicxml.org/dtds/partwise.dtd" [<!ENTITY name "Piano">]>`, "&name;"},
		"internal entity":                 {`<!DOCTYPE score-partwise [<!ENTITY name "Piano">]>`, "&name;"},
	} {
		doctype := tc.doctype
		t.Run(name, func(t *testing.T) {
			score := `<?xml version="1.0" encoding="UTF-8"?>` + doctype + body(tc.partName)
			xml, err := DecodeAssessed(context.Background(), strings.NewReader(score), DecodeAssessmentOptions{Lenient: true})
			if err != nil || xml.Document == nil {
				t.Fatalf("DecodeAssessed: %v", err)
			}
			mxl, err := DecodeMXLAssessed(context.Background(), bytes.NewReader(rawMXL(t, score)), DecodeMXLAssessmentOptions{Lenient: true})
			if err != nil || mxl.Package == nil {
				t.Fatalf("DecodeMXLAssessed: %v conversion=%s", err, mxl.Conversion.State)
			}
			if !reflect.DeepEqual(mxl.Package.Document, xml.Document) {
				t.Fatalf("models differ:\nMXL %+v\nXML %+v", mxl.Package.Document, xml.Document)
			}
			partwise := mxl.Package.Document.(*ScorePartwise)
			if part := partwise.PartList.Content[0].ScorePart; part == nil || part.PartName.Value != "Piano" {
				t.Errorf("entity not expanded: %+v", partwise.PartList.Content[0])
			}
			if strings.Contains(doctype, "PUBLIC") && (partwise.Version == nil || *partwise.Version != "1.0") {
				t.Errorf("DTD default version missing: %v", partwise.Version)
			}
		})
	}
}

// Decoding the root document from the infoset does not charge its bytes as
// input a second time: the smallest input budget the source stage accepts
// also converts.
func TestDecodeMXLAssessedInfosetChargesInputOnce(t *testing.T) {
	archive := rawMXL(t, `<score-partwise version="4.0"><part-list><score-part id="P1"><part-name>Piano</part-name></score-part></part-list>`+
		`<part id="P1"><measure number="1"><attributes><divisions>1</divisions></attributes><note><rest/><duration>1</duration></note></measure></part></score-partwise>`)
	accepts := func(limit int64) bool {
		bundle, err := InspectMXL(context.Background(), bytes.NewReader(archive), MXLAssessmentOptions{Graph: AssessmentOptions{Budgets: validation.Budgets{MaxInputBytes: limit}}})
		return err == nil && bundle.Satisfied()
	}
	low, high := int64(1), int64(len(archive)*4)
	for low < high {
		if mid := (low + high) / 2; accepts(mid) {
			high = mid
		} else {
			low = mid + 1
		}
	}
	result, err := DecodeMXLAssessed(context.Background(), bytes.NewReader(archive), DecodeMXLAssessmentOptions{Assessment: MXLAssessmentOptions{Graph: AssessmentOptions{Budgets: validation.Budgets{MaxInputBytes: low}}}})
	if err != nil || result.Package == nil {
		t.Fatalf("MaxInputBytes=%d: source accepted but loading failed: conversion=%s error=%v", low, result.Conversion.State, err)
	}
}

// cancelAfterContext reports cancellation after a number of Err calls once
// it is armed.
type cancelAfterContext struct {
	context.Context
	cancel context.CancelFunc
	armed  bool
	calls  int
	limit  int
}

func (c *cancelAfterContext) Err() error {
	if c.armed {
		c.calls++
		if c.calls >= c.limit {
			c.cancel()
		}
	}
	return c.Context.Err()
}

// A cancellation at any point of the start of the conversion in strict MXL
// loading, which reads and decodes the container from the DTD infoset, is
// reported as cancellation, never as an invalid archive.
func TestDecodeMXLAssessedCancellationIsNeverAnInvalidArchive(t *testing.T) {
	archive := rawMXL(t, mandatoryScore("1", ""))
	var ctx *cancelAfterContext
	decodeMXLStageHook = func(stage string, _ *validation.Account) {
		if stage == "conversion" {
			ctx.armed = true
		}
	}
	t.Cleanup(func() { decodeMXLStageHook = nil })
	canceled := 0
	for limit := 1; limit <= 300; limit++ {
		parent, cancel := context.WithCancel(context.Background())
		ctx = &cancelAfterContext{Context: parent, cancel: cancel, limit: limit}
		_, err := DecodeMXLAssessed(ctx, bytes.NewReader(archive), DecodeMXLAssessmentOptions{Lenient: true})
		cancel()
		if err == nil {
			break
		}
		canceled++
		if !errors.Is(err, context.Canceled) || errors.Is(err, ErrInvalidMXL) {
			t.Fatalf("cancellation after %d conversion checks: %v", limit, err)
		}
	}
	if canceled != 300 {
		t.Fatalf("only %d cancellation points reached", canceled)
	}
}
