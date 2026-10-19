package musicxml

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"
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
