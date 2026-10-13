package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func reviewBoundaryFixture(t *testing.T, last string, existing bool) ([]byte, *MXLPackage) {
	t.Helper()
	main, one, two := depthLink("one.opus"), depthLink("two.opus"), depthLink(last)
	doc, err := Decode(strings.NewReader(main))
	if err != nil {
		t.Fatal(err)
	}
	entries := []assessmentZIPFixture{{name: "one.opus", body: one}, {name: "two.opus", body: two}}
	value := &MXLPackage{Document: doc, RootFiles: []MXLRootFile{{FullPath: "main.opus"}}, Resources: []MXLResource{{Path: "one.opus", Data: []byte(one)}, {Path: "two.opus", Data: []byte(two)}}}
	if existing {
		entries = append(entries, assessmentZIPFixture{name: "missing.opus", body: `<opus/>`})
		value.Resources = append(value.Resources, MXLResource{Path: "missing.opus", Data: []byte(`<opus/>`)})
	}
	return packageGraphBytes(main, "main.opus", "", entries...), value
}

func reviewBoundaryAbsenceCount(b validation.AssessmentBundle) int {
	n := 0
	for _, f := range b.PackageReport().Findings {
		if f.Rule.ID == "MX40-PACKAGE-link-target" && f.Outcome == validation.OutcomeFail && f.Reason == "linked-resource-authoritatively-absent" {
			n++
		}
	}
	return n
}

func TestReviewGraphBoundaryArchiveInventoryPublic(t *testing.T) {
	for _, current := range []bool{false, true} {
		for _, mode := range []string{"absent", "existing", "cycle"} {
			t.Run(fmt.Sprintf("current=%t/%s", current, mode), func(t *testing.T) {
				last := "missing.opus"
				if mode == "cycle" {
					last = "one.opus"
				}
				data, value := reviewBoundaryFixture(t, last, mode == "existing")
				calls := 0
				options := MXLAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileMXLSource, Budgets: validation.Budgets{MaxDepth: 3}}, Dependencies: []ResourceDependency{ResourceDocuments}, Resolver: graphResolverFunc(func(context.Context, validation.ResourceRequest) (validation.ResourceResult, error) {
					calls++
					return validation.ResourceResult{}, errors.New("archive-local target called resolver")
				})}
				var b validation.AssessmentBundle
				var err error
				if current {
					options.Graph.Profile = ProfileMXLModel
					b, err = AssessMXLPackage(context.Background(), value, options)
				} else {
					b, err = InspectMXL(context.Background(), bytes.NewReader(data), options)
				}
				if calls != 0 {
					t.Fatalf("resolver calls=%d", calls)
				}
				if mode == "existing" {
					var depth *validation.ResourceError
					if !errors.As(err, &depth) || depth.Kind != validation.BudgetDepth || depth.Limit != 3 || depth.Used != 3 || depth.Requested != 1 || b.EnumerationComplete() {
						t.Fatalf("existing unadmitted member bypassed depth: %v complete=%t", err, b.EnumerationComplete())
					}
					return
				}
				if err != nil || !b.EnumerationComplete() {
					t.Fatalf("local inventory outcome lost at boundary: %v complete=%t", err, b.EnumerationComplete())
				}
				want := 0
				if mode == "absent" {
					want = 1
				}
				if got := reviewBoundaryAbsenceCount(b); got != want {
					t.Fatalf("authoritative absence findings=%d want=%d: %+v", got, want, b.PackageReport().Findings)
				}
			})
		}
	}
}

// Run the same ingestion/walk as the public entry points while retaining the
// account and archive so assertions can inspect selection and cumulative usage.
func reviewBoundaryImported(t *testing.T, ctx context.Context, current, existing bool, last string, linkedLimit int64) (*resourceGraphRun, *graphPackage, int64, *int) {
	t.Helper()
	data, value := reviewBoundaryFixture(t, last, existing)
	profile := ProfileMXLSource
	if current {
		profile = ProfileMXLModel
	}
	calls := new(int)
	options := LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: profile, Budgets: validation.Budgets{MaxDepth: 3, MaxLinkedDocuments: linkedLimit}}, Dependencies: []ResourceDependency{ResourceDocuments}, Resolver: graphResolverFunc(func(context.Context, validation.ResourceRequest) (validation.ResourceResult, error) {
		(*calls)++
		return validation.ResourceResult{}, errors.New("archive-local target called resolver")
	})}
	g, err := startResourceGraph(ctx, options, profile)
	if err != nil {
		t.Fatal(err)
	}
	var p *graphPackage
	wantBytes := int64(len(value.Resources[0].Data) + len(value.Resources[1].Data))
	if current {
		p, err = g.newPackage(g.primaryPin(validation.ResourceCurrentMXL, ""), "", "", nil)
		if err == nil {
			err = g.importCurrentPackage(p, value, "")
		}
	} else {
		var archive *mxlAssessmentArchive
		archive, err = openMXLAssessmentArchive(ctx, bytes.NewReader(data), MXLOptions{}, g.options.account)
		if err == nil {
			err = g.ownArchive(archive)
		}
		if err == nil {
			digest := sha256.Sum256(data)
			_, _, _, err = g.importRawPackage(archive, g.primaryPin(validation.ResourceRawMXL, hex.EncodeToString(digest[:])), "", "", true)
		}
		if err == nil {
			p = g.packageList[0]
			for _, name := range []string{mxlMIMETypePath, mxlContainerPath, "main.opus"} {
				wantBytes += int64(len(archive.contents[archive.index[name]].data))
			}
		}
	}
	if err != nil {
		g.finish(err)
		t.Fatal(err)
	}
	return g, p, wantBytes, calls
}

func TestReviewGraphBoundaryArchiveInventoryAccounting(t *testing.T) {
	for _, current := range []bool{false, true} {
		for _, mode := range []string{"absent", "existing", "cycle", "incomplete", "invalid", "quota-exact", "quota"} {
			t.Run(fmt.Sprintf("current=%t/%s", current, mode), func(t *testing.T) {
				last, linkedLimit := "missing.opus", int64(0)
				wantLinked := int64(3)
				if !current {
					wantLinked++ // The original container has its own adoption.
				}
				if mode == "cycle" {
					last = "one.opus"
				}
				if mode == "quota" {
					linkedLimit = wantLinked - 1
				} else if mode == "quota-exact" {
					linkedLimit = wantLinked
				}
				g, p, wantBytes, calls := reviewBoundaryImported(t, context.Background(), current, mode == "existing", last, linkedLimit)
				cause := validation.DecisionRef{Rule: validation.RuleKey{ID: "inventory-provider", Revision: 1}, Subject: validation.NodeRef{DocumentID: "inventory", Node: 7}}
				wantCause := cause
				if mode == "incomplete" {
					p.entriesComplete = validation.Evidence[bool]{State: validation.EvidenceUnknown, Reason: "inventory-interrupted"}
				} else if mode == "invalid" {
					p.entriesComplete = validation.Evidence[bool]{State: validation.EvidenceInvalid, Reason: "inventory-invalid", Cause: &cause}
				}
				err := g.walkResources()
				// Edge ingress owns its causal evidence independently of the provider.
				cause.Subject.Node = 99
				if mode == "existing" || mode == "quota" {
					wantLinked--
					wantKind := validation.BudgetDepth
					if mode == "quota" {
						wantKind = validation.BudgetLinkedDocuments
					}
					var limit *validation.ResourceError
					if !errors.As(err, &limit) || limit.Kind != wantKind {
						t.Errorf("expected %s refusal, got %v", wantKind, err)
					}
				} else if err != nil {
					t.Errorf("inventory-only outcome refused: %v", err)
				}
				if p.archive != nil {
					if index, ok := p.archive.index["missing.opus"]; ok && p.archive.contents[index].attempted {
						t.Error("unadmitted raw member was read")
					}
				} else if _, copied := p.copied["missing.opus"]; copied {
					t.Error("unadmitted current member was copied")
				}
				bytesUsed, _ := g.options.account.Usage(validation.BudgetExternalBytes)
				linked, _ := g.options.account.Usage(validation.BudgetLinkedDocuments)
				if *calls != 0 || bytesUsed != wantBytes || linked != wantLinked {
					t.Errorf("callback/read/admission counts: calls=%d bytes=%d want=%d linked=%d want=%d", *calls, bytesUsed, wantBytes, linked, wantLinked)
				}
				if err == nil {
					err = g.observePackageMetadata()
				}
				if err == nil {
					err = g.sealPackages()
				}
				b, _ := g.finish(err)
				if mode == "incomplete" || mode == "invalid" {
					if reviewBoundaryAbsenceCount(b) != 0 || b.EnumerationComplete() {
						t.Error("incomplete inventory fabricated absence or complete enumeration")
					}
					found := false
					for _, f := range b.PackageReport().Findings {
						if f.Rule.ID == "MX40-PACKAGE-link-target" && (mode == "incomplete" && f.Outcome == validation.OutcomeUnknown && f.Reason == "inventory-interrupted" && f.Cause == nil || mode == "invalid" && f.Outcome == validation.OutcomeBlocked && f.Reason == "inventory-invalid" && f.Cause != nil && *f.Cause == wantCause) {
							found = true
						}
					}
					if !found {
						t.Errorf("missing target lost inventory reason/cause: %+v", b.PackageReport().Findings)
					}
				}
				reviewGraphNoLiveLeases(t, g)
			})
		}
	}
}

func TestReviewGraphArchiveInventoryPrerequisiteLookup(t *testing.T) {
	for _, mode := range []string{"absent", "missing-package", "incomplete", "invalid", "known-incomplete", "other-package", "encoded-name", "canceled", "terminated", "work-refused"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			g, err := startResourceGraph(ctx, LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph}}, ProfileResourceGraph)
			if err != nil {
				t.Fatal(err)
			}
			path := "missing.opus"
			p := &graphPackage{id: "root", entries: map[string]validation.PackageFactID{}, entriesComplete: packageKnown(true)}
			g.packages[p.id] = p
			want := unavailableTarget(validation.EvidenceAbsent, "", nil)
			var wantErr error
			cause := validation.DecisionRef{Rule: validation.RuleKey{ID: "inventory-provider", Revision: 1}, Subject: validation.NodeRef{DocumentID: "inventory", Node: 7}}
			switch mode {
			case "missing-package":
				delete(g.packages, p.id)
				want = unavailableTarget(validation.EvidenceUnknown, "archive-resource-unavailable", nil)
			case "incomplete":
				p.entriesComplete = packageUnknown[bool]("partial-inventory")
				want = unavailableTarget(validation.EvidenceUnknown, "partial-inventory", nil)
			case "invalid":
				p.entriesComplete = validation.Evidence[bool]{State: validation.EvidenceInvalid, Reason: "invalid-inventory", Cause: &cause}
				want = unavailableTarget(validation.EvidenceInvalid, "invalid-inventory", &cause)
			case "known-incomplete":
				p.entriesComplete = packageKnown(false)
				want = unavailableTarget(validation.EvidenceUnknown, "archive-resource-unavailable", nil)
			case "other-package":
				g.packages["sibling"] = &graphPackage{id: "sibling", entries: map[string]validation.PackageFactID{path: 1}, entriesComplete: packageKnown(true)}
			case "encoded-name":
				path = "dir%2Fscore.opus"
				p.entries["dir/score.opus"] = 1
			case "canceled":
				cancel()
				wantErr = context.Canceled
			case "terminated":
				wantErr = g.session.ObserveEdge(ctx, validation.PackageEdgeObservation{})
				if wantErr == nil || g.session.TerminalCause() == nil {
					t.Fatal("fixture failed to terminate session")
				}
			case "work-refused":
				used, _ := g.options.account.Usage(validation.BudgetRuleSteps)
				if err = g.options.account.Charge(validation.BudgetRuleSteps, g.options.account.Limits().MaxRuleSteps-used); err != nil {
					t.Fatal(err)
				}
				wantErr = validation.ErrResourceLimit
			}
			before, _ := g.options.account.Usage(validation.BudgetBufferedBytes)
			steps, _ := g.options.account.Usage(validation.BudgetRuleSteps)
			child, base, pkg, target, _, err := g.loadArchiveDocument(graphExpansion{pkg: "root", depth: 3}, path)
			if wantErr != nil {
				if !errors.Is(err, wantErr) || target.State == validation.EvidenceAbsent {
					t.Errorf("terminal prerequisite bypassed: err=%v want=%v target=%+v", err, wantErr, target)
				}
			} else if err != nil || target != want {
				t.Errorf("inventory evidence=%+v want=%+v error=%v", target, want, err)
			}
			after, _ := g.options.account.Usage(validation.BudgetBufferedBytes)
			bytesUsed, _ := g.options.account.Usage(validation.BudgetExternalBytes)
			stepsAfter, _ := g.options.account.Usage(validation.BudgetRuleSteps)
			if child != nil || base != (resourceURIBase{}) || pkg != "root" || after != before || bytesUsed != 0 {
				t.Errorf("inventory lookup selected a body: child=%v base=%+v pkg=%s buffered=%d/%d bytes=%d", child, base, pkg, before, after, bytesUsed)
			}
			if wantErr == nil && stepsAfter-steps != int64(len(path))+1 || wantErr != nil && stepsAfter != steps {
				t.Errorf("inventory lookup work=%d want=%d or zero after refusal", stepsAfter-steps, len(path)+1)
			}
			g.finish(err)
			reviewGraphNoLiveLeases(t, g)
		})
	}
}

func TestReviewGraphRepeatedEdgesRetainOnlyLiveIndexes(t *testing.T) {
	const href = "target.musicxml"
	source := `<opus xmlns:xlink="http://www.w3.org/1999/xlink">` + strings.Repeat(`<opus-link xlink:href="`+href+`"/>`, 3) + `</opus>`
	// Each occurrence has a wrong-kind target and therefore needs its own finding,
	// even though all three requests resolve to one canonical resource/context.
	bodyText := resourceScore("")
	var g *resourceGraphRun
	var bodies []*resourceTestBody
	var retained []int64
	resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
		n := g.retained
		for key := range g.fileMetadata {
			n -= 512 + 2*int64(len(key.base)+len(key.entry)+len(key.origin)+len(key.pkg)+len(key.pin.CanonicalID)+len(key.pin.Revision)+len(key.pin.Digest)+len(key.pin.SnapshotID))
		}
		retained = append(retained, n)
		body := &resourceTestBody{Reader: strings.NewReader(bodyText)}
		bodies = append(bodies, body)
		digest := sha256.Sum256([]byte(bodyText))
		return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "target", Representation: validation.ResourceSourceXML, Digest: hex.EncodeToString(digest[:])}, RetrievalURI: "https://e.test/" + href, Body: body}, nil
	})
	var result resourceIntrinsicResult
	var pin validation.ResourcePin
	var id string
	g, result, pin, id = reviewGraphSource(t, context.Background(), source, LinkedAssessmentOptions{Graph: AssessmentOptions{Profile: ProfileResourceGraph}, Identity: ResourceIdentity{RetrievalURI: "https://e.test/root.opus"}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceDocuments}})
	b, err := g.finishPrimary(result, pin, id)
	if err != nil || !b.EnumerationComplete() || len(bodies) != 3 || b.ResourceCount() != 2 || b.AdoptionCount() != 4 {
		t.Fatalf("repeated occurrence identities changed: err=%v bodies=%d resources=%d adoptions=%d", err, len(bodies), b.ResourceCount(), b.AdoptionCount())
	}
	// After the first occurrence, canonical storage and expansion are fixed.
	// Each further edge needs only the 512-byte live linked-target index, after
	// subtracting its independently owned file-metadata index above.
	if growth := retained[2] - retained[1]; growth != 512 {
		t.Errorf("alias retained unused per-request state: growth=%d want=512", growth)
	}
	findings := map[string]bool{}
	for _, f := range b.PackageReport().Findings {
		if f.Rule.ID == "MX40-PACKAGE-link-target" && f.Reason == "wrong-linked-document-kind" {
			findings[f.Location.Path] = true
		}
	}
	linked, _ := g.options.account.Usage(validation.BudgetLinkedDocuments)
	external, _ := g.options.account.Usage(validation.BudgetExternalBytes)
	if len(findings) != 3 || linked != 3 || external != int64(3*len(bodyText)) {
		t.Errorf("canonical reuse lost per-edge outcomes/accounting: findings=%d linked=%d bytes=%d", len(findings), linked, external)
	}
	for _, body := range bodies {
		if body.closed != 1 {
			t.Errorf("body closes=%d", body.closed)
		}
	}
	reviewGraphNoLiveLeases(t, g)
}
