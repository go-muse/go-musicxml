package musicxml

import (
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

type timelineGapSink struct {
	*validation.Session
	headers map[validation.NodeID]validation.NodeHeader
	gapName string
}

func (s *timelineGapSink) Open(ctx context.Context, n validation.NodeHeader) error {
	s.headers[n.ID] = n
	return s.Session.Open(ctx, n)
}
func (s *timelineGapSink) Close(ctx context.Context, n validation.NodeEnd) error {
	if s.headers[n.ID].Name.Local == s.gapName {
		n.ChildrenComplete = validation.Evidence[bool]{State: validation.EvidenceUnknown, Reason: "test-hierarchy-enumeration-gap"}
	}
	return s.Session.Close(ctx, n)
}
func TestTimelineHierarchyGapCannotProveMissingKey(t *testing.T) {
	source := (localFixture{body: timelineNumeral("", "")}).xml()
	document, err := Decode(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"score-partwise", "part"} {
		t.Run(name, func(t *testing.T) {
			engine, err := newTimelineAssessmentEngine()
			if err != nil {
				t.Fatal(err)
			}
			account, err := validation.NewBudgetAccount(validation.Budgets{})
			if err != nil {
				t.Fatal(err)
			}
			session, err := engine.Begin(context.Background(), validation.RunOptions{DocumentID: modelDocumentID, Profile: ProfileModelTimeline, Account: account})
			if err != nil {
				t.Fatal(err)
			}
			sink := &timelineGapSink{Session: session, headers: map[validation.NodeID]validation.NodeHeader{}, gapName: name}
			_, walkErr := walkModel(context.Background(), document, sink, account, modelDocumentID, modelWalkOptions{})
			if walkErr != nil {
				t.Fatal(walkErr)
			}
			report, err := session.Finish(context.Background(), validation.Completion{InputComplete: true, Syntax: validation.SyntaxNotAssessed})
			if err != nil {
				t.Fatal(err)
			}
			seen := false
			for _, f := range report.Findings {
				if f.Rule.ID == "MX40-TIMELINE-numeral-requires-key" {
					seen = true
					if f.Outcome == validation.OutcomeFail {
						t.Fatalf("unknown %s list fabricated missing key: %s", name, localFindings(report))
					}
				}
			}
			if !seen {
				t.Fatalf("missing causal timeline result: %s", localFindings(report))
			}
		})
	}
}
