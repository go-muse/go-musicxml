package musicxml

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func FuzzMXLAssessmentZIPObserver(f *testing.F) {
	for _, seed := range [][]byte{
		{}, []byte("PK\x03\x04"), assessmentZIPBytes(nil, nil, ""),
		assessmentZIPBytes([]assessmentZIPFixture{{name: "nested.mxl", body: string(assessmentZIPBytes([]assessmentZIPFixture{{name: "inner", body: strings.Repeat("x", 12<<10)}}, nil, ""))}}, nil, "\x01"),
		assessmentZIPBytes([]assessmentZIPFixture{{name: "large.xml", body: strings.Repeat("x", 40<<10)}}, nil, ""),
		assessmentZIPBytes([]assessmentZIPFixture{{name: "bounded.xml", body: strings.Repeat("x", 12<<10)}}, nil, "\x01"),
		assessmentZIPBytes([]assessmentZIPFixture{{name: "mimetype", body: mxlMIMEType, localExtra: "\xfe\xca\x00\x00"}}, nil, ""),
		assessmentZIPBytes([]assessmentZIPFixture{{name: "a", body: "deflate", method: 8, flags: 8, descriptor: 12}}, nil, "comment"),
		assessmentZIPBytes([]assessmentZIPFixture{{name: "a", body: "signed", flags: 8, descriptor: 16}, {name: "b", body: "other"}}, []int{1, 0}, ""),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		for _, borrowed := range []bool{false, true} {
			func() {
				account := assessmentZIPAccount(t, validation.Budgets{MaxInputBytes: 1 << 20, MaxExternalBytes: 128 << 10, MaxBufferedBytes: 4 << 20, MaxNodes: 256, MaxScalarBytes: 65535, MaxRuleSteps: 1 << 20})
				options := MXLOptions{MaxArchiveBytes: 1 << 20, MaxMetadataBytes: 16 << 10, MaxDocumentBytes: 64 << 10, MaxResourceBytes: 32 << 10, MaxResourcesBytes: 128 << 10}
				if len(b) > 0 && b[len(b)-1]&1 != 0 {
					options.MaxDocumentBytes = 8 << 10 // XML prefix lowers the automatic cap.
				}
				var a *mxlAssessmentArchive
				var err error
				upstream := int64(0)
				if borrowed {
					if account.Charge(validation.BudgetInputBytes, int64(len(b))) != nil {
						return
					}
					if account.Charge(validation.BudgetBufferedBytes, int64(len(b))) != nil {
						return
					}
					upstream = int64(len(b))
					a, err = borrowMXLAssessmentArchive(context.Background(), b, options, account)
					input, _ := account.Usage(validation.BudgetInputBytes)
					external, _ := account.Usage(validation.BudgetExternalBytes)
					if input != upstream || external != 0 {
						t.Fatal("borrower recharged upstream representation")
					}
				} else {
					a, err = openMXLAssessmentArchive(context.Background(), bytes.NewReader(b), options, account)
				}
				defer func() {
					a.Close()
					if borrowed {
						live, _ := account.Usage(validation.BudgetBufferedBytes)
						if live != upstream {
							t.Fatalf("borrower released upstream ownership: %d, want %d", live, upstream)
						}
						if err := account.Release(validation.BudgetBufferedBytes, upstream); err != nil {
							t.Fatal(err)
						}
					}
					assessmentZIPClose(t, a, account)
				}()
				if err != nil {
					return
				}
				if !a.ready || !a.inventoryComplete || !a.countConsistent || len(a.entries) != len(a.reader.File) {
					t.Fatal("successful archive lacks complete inventory")
				}
				var end int64
				for _, i := range a.physical {
					entry := a.entries[i]
					if !entry.trustedIdentity || !entry.associated || entry.localOffset != end {
						t.Fatal("unproved physical range")
					}
					end = entry.end
				}
				for i := 0; i < len(a.entries) && i < 3; i++ {
					first := mxlAssessmentPrimaryDocument
					if i%2 != 0 {
						first = mxlAssessmentLinkedDocument
					}
					for _, role := range []mxlAssessmentEntryRole{first, mxlAssessmentLinkedDocument, mxlAssessmentPrimaryDocument, mxlAssessmentMetadata} {
						before, _ := account.Usage(validation.BudgetExternalBytes)
						cached := a.contents[i].attempted
						data, readErr := a.readEntry(i, role)
						after, _ := account.Usage(validation.BudgetExternalBytes)
						if after < before || after > options.MaxResourcesBytes || after > account.Limits().MaxExternalBytes || after != a.expanded {
							t.Fatal("expanded byte ledger escaped cap")
						}
						if cached && after != before {
							t.Fatal("cached role change recharged represented bytes")
						}
						if readErr == nil {
							limit := options.MaxMetadataBytes
							document, archive, automatic := mxlAssessmentRepresentationLimits(role, a.limits)
							if automatic {
								limit = mxlAssessmentRepresentationLimit(data, document, archive)
							}
							if int64(len(data)) > limit {
								t.Fatal("successful role read exceeded representation cap")
							}
						}
					}
				}
			}()
		}
	})
}
