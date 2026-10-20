package musicxml

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"github.com/go-muse/go-musicxml/validation"
	"strings"
	"testing"
)

func finalQARoleReply(q validation.ResourceRequest, mode string) validation.ResourceResult {
	r := validation.ResourceRoleResult{Request: *q.Roles, ScopeComplete: q.Roles.ScopeComplete, Authority: validation.EvidenceAuthority{ID: "qa-authoritative-role-table", ExactRelationComplete: true, Provenance: []validation.SourceRef{{IDs: []string{"qa-roles"}, URL: "https://final-qa.test/roles", Locator: "exact member table"}}}}
	for _, m := range q.Roles.Members {
		for _, role := range []validation.ResourceRole{validation.ResourceRoleScore, validation.ResourceRoleIndividualPart} {
			member := m
			known := m.ArchiveEntry == "named-part.musicxml" && role == validation.ResourceRoleScore || m.ArchiveEntry == "named-score.musicxml" && role == validation.ResourceRoleIndividualPart
			if mode == "exclude" {
				known = false
			}
			r.Resources = append(r.Resources, validation.ResourceRoleFact{Resource: m.Resource, Member: &member, Role: role, Membership: packageKnown(known)})
		}
	}
	if mode == "partial" {
		r.Resources = r.Resources[:1]
		r.ScopeComplete = packageUnknown[bool]("fixture-partial")
	}
	if mode == "forged" {
		r.Request.Members[0].ArchiveEntry = "forged.xml"
	}
	return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Roles: &r}
}
func TestFinalQAPublicRoleOrchestration(t *testing.T) {
	for _, surface := range []string{"raw", "current"} {
		for _, mode := range []string{"score-first", "part-first", "unselected", "unknown", "partial", "exclude", "unread-exclude", "forged", "error-body", "cancel"} {
			t.Run(surface+"/"+mode, func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				calls := 0
				var body *resourceTestBody
				boom := errors.New("role-transport")
				first, second := "named-part.musicxml", "named-score.musicxml"
				if mode == "part-first" {
					first, second = second, first
				}
				opts := MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLSource}, Identity: ResourceIdentity{RetrievalURI: "https://final-qa.test/package.mxl"}, Dependencies: []ResourceDependency{ResourceRoles}}
				if mode == "unselected" {
					opts.Dependencies = nil
				}
				opts.Resolver = graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
					calls++
					if q.Roles == nil || q.Document != nil {
						t.Fatalf("unexpected callback %+v", q)
					}
					want := 3
					if surface == "current" {
						want = 2
					}
					if len(q.Roles.Members) != want {
						t.Fatalf("member count%d want%d", len(q.Roles.Members), want)
					}
					for _, m := range q.Roles.Members {
						if m.BaseURI.State != validation.EvidenceKnown || m.BaseURI.Value != opts.Identity.RetrievalURI || m.ArchiveEntry == "" || m.Resource.CanonicalID == "" {
							t.Fatalf("exact member context missing %+v", m)
						}
					}
					if mode == "unread-exclude" {
						if q.Roles.ScopeComplete.State != validation.EvidenceUnknown {
							t.Fatal("unread member universe certified")
						}
					} else if q.Roles.ScopeComplete.State != validation.EvidenceKnown || !q.Roles.ScopeComplete.Value {
						t.Fatal("complete selected universe lost", q.Roles)
					}
					if mode == "unknown" {
						return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceUnknown, Reason: "qa-unavailable"}, nil
					}
					if mode == "error-body" {
						body = &resourceTestBody{Reader: strings.NewReader("unexpected")}
						return validation.ResourceResult{Body: body}, boom
					}
					if mode == "cancel" {
						cancel()
					}
					replyMode := mode
					if mode == "unread-exclude" {
						replyMode = "exclude"
					}
					return finalQARoleReply(q, replyMode), nil
				})
				source := resourceScore("")
				var b validation.AssessmentBundle
				var e error
				if surface == "raw" {
					c := `<container><rootfiles><rootfile full-path="` + first + `"/><rootfile full-path="` + second + `"/></rootfiles></container>`
					entries := []assessmentZIPFixture{{name: mxlMIMETypePath, body: mxlMIMEType}, {name: mxlContainerPath, body: c}, {name: first, body: source}, {name: second, body: source}}
					if mode == "unread-exclude" {
						entries = append(entries, assessmentZIPFixture{name: "unread.musicxml", body: "opaque"})
					}
					b, e = InspectMXL(ctx, bytes.NewReader(assessmentZIPBytes(entries, nil, "")), opts)
				} else {
					d, x := Decode(strings.NewReader(source))
					if x != nil {
						t.Fatal(x)
					}
					p := &MXLPackage{Document: d, RootFiles: []MXLRootFile{{FullPath: first}, {FullPath: second}}, Resources: []MXLResource{{Path: second, Data: []byte(source)}}}
					if mode == "unread-exclude" {
						p.Resources = append(p.Resources, MXLResource{Path: "unread.musicxml", Data: []byte("opaque")})
					}
					opts.Graph.Profile = ProfileMXLModel
					b, e = AssessMXLPackage(ctx, p, opts)
				}
				wantCalls := 1
				if mode == "unselected" {
					wantCalls = 0
				}
				if calls != wantCalls {
					t.Fatalf("role callbacks%d want%d", calls, wantCalls)
				}
				operational := mode == "forged" || mode == "error-body" || mode == "cancel"
				if (e != nil) != operational {
					t.Fatalf("err=%v", e)
				}
				if mode == "error-body" && (body.closed != 1 || !errors.Is(e, boom)) {
					t.Fatalf("body/cause %d %v", body.closed, e)
				}
				if mode == "cancel" && !errors.Is(e, context.Canceled) {
					t.Fatal(e)
				}
				failed := finalQAFinding(b, "score-must-precede-parts", validation.OutcomeFail)
				if failed != (mode == "part-first") {
					t.Fatalf("role outcome mismatch %+v", b.PackageReport().Findings)
				}
				if mode == "unselected" || mode == "unknown" || mode == "partial" || mode == "unread-exclude" {
					if b.Satisfied() {
						t.Fatal("partial/unread role scope certified")
					}
					found := false
					for _, f := range b.PackageReport().Findings {
						found = found || f.Rule.ID == "MX40-PACKAGE-score-before-parts" && f.Outcome == validation.OutcomeUnknown
					}
					if !found {
						t.Fatalf("missing role Unknown %+v", b.PackageReport().Findings)
					}
				}
				if mode == "score-first" || mode == "exclude" {
					for _, f := range b.PackageReport().Findings {
						if f.Rule.ID == "MX40-PACKAGE-score-before-parts" {
							t.Fatalf("authoritative role proof unavailable %+v", f)
						}
					}
				}
			})
		}
	}
}

func TestFinalQAFileMetadataExternalTargets(t *testing.T) {
	for _, surface := range []string{"xml", "model", "raw-mxl", "current-mxl"} {
		for _, mode := range []string{"musicxml", "legacy-xml", "wrong-suffix", "wrong-media", "absent", "unsupported-uri", "malformed", "wrong-digest", "read-error"} {
			t.Run(surface+"/"+mode, func(t *testing.T) {
				bodyText := `<opus/>`
				if mode == "malformed" {
					bodyText = `<opus><unclosed`
				}
				retrieval := "https://final-qa.test/child.musicxml"
				media := "application/vnd.recordare.musicxml+xml"
				if mode == "legacy-xml" {
					retrieval = "https://final-qa.test/child.xml"
				}
				if mode == "wrong-suffix" {
					retrieval = "https://final-qa.test/child.bad"
				}
				if mode == "wrong-media" {
					media = "application/pdf"
				}
				if mode == "absent" {
					retrieval = ""
					media = ""
				}
				if mode == "unsupported-uri" {
					retrieval = "custom://host/child.musicxml"
				}
				calls := 0
				body := &resourceTestBody{Reader: strings.NewReader(bodyText)}
				boom := errors.New("file-read")
				if mode == "read-error" {
					body.Reader = &finalQAReadError{data: []byte(bodyText), err: boom}
				}
				resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
					calls++
					if q.Document == nil {
						t.Fatal("unrequested metadata callback")
					}
					digest := fmt.Sprintf("%x", sha256.Sum256([]byte(bodyText)))
					if mode == "wrong-digest" {
						digest = strings.Repeat("0", 64)
					}
					return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: "metadata-child", Representation: validation.ResourceSourceXML, Digest: digest}, RetrievalURI: retrieval, MediaType: media, Body: body}, nil
				})
				root := `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><opus-link xlink:href="https://final-qa.test/child"/></opus>`
				b, e := finalQARun(t, surface, root, context.Background(), finalQAOptions(resolver))
				operational := mode == "malformed" || mode == "wrong-digest" || mode == "read-error"
				if (e != nil) != operational || calls != 1 || body.closed != 1 {
					t.Fatalf("error=%v calls%d closes%d", e, calls, body.closed)
				}
				found := false
				for i := 0; i < b.FileMetadataCount(); i++ {
					m, _ := b.FileMetadataAt(i)
					if m.Pin.CanonicalID != "metadata-child" {
						continue
					}
					found = true
					if m.Source == "" || m.Entry != 0 || m.ArchiveEntry != "" || m.BaseURI != retrieval || m.Location.Path == "" || m.Href != "https://final-qa.test/child" {
						t.Fatalf("metadata lost exact link context %+v", m)
					}
					if mode == "absent" && (m.Name.State != validation.EvidenceAbsent || m.MediaType.State != validation.EvidenceAbsent) {
						t.Fatalf("absent fields invented %+v", m)
					}
					if mode == "unsupported-uri" && m.Name.State != validation.EvidenceUnknown {
						t.Fatal("URI policy guessed filename", m)
					}
					if mode == "malformed" && (b.EnumerationComplete() || b.Satisfied()) {
						t.Fatal("metadata promoted failed parse")
					}
				}
				want := mode != "wrong-digest" && mode != "read-error"
				if found != want {
					t.Fatalf("metadata present%v want%v count%d error%v", found, want, b.FileMetadataCount(), e)
				}
				if mode == "wrong-suffix" && !finalQAFinding(b, "nonstandard-file-suffix", validation.OutcomeFail) {
					t.Fatal("missing suffix advice")
				}
				if mode == "wrong-media" && !finalQAFinding(b, "nonstandard-file-media-type", validation.OutcomeFail) {
					t.Fatal("missing media advice")
				}
				if mode == "wrong-suffix" || mode == "wrong-media" {
					if b.PackageReport().Counts.Fail != 0 {
						t.Fatal("metadata advice became normative failure")
					}
				}
			})
		}
	}
}

func TestFinalQAAssociationTargetMatrix(t *testing.T) {
	source := `<score-partwise xmlns:xlink="http://www.w3.org/1999/xlink"><part-list><score-part id="P1"><part-link xlink:href="https://final-qa.test/part.musicxml"><instrument-link id="I1"/></part-link><part-name>P</part-name><score-instrument id="I1"><instrument-name>P</instrument-name></score-instrument></score-part></part-list><part id="P1"><measure number="1"/></part></score-partwise>`
	for _, surface := range []string{"xml", "model", "raw-mxl", "current-mxl", "nested"} {
		for _, mode := range []string{"positive", "negative", "unknown", "unselected"} {
			t.Run(surface+"/"+mode, func(t *testing.T) {
				docCalls, associationCalls := 0, 0
				target := resourceScore("")
				var bodies []*resourceTestBody
				resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
					if q.Document != nil {
						docCalls++
						data := []byte(target)
						rep := validation.ResourceSourceXML
						id := "target-part"
						if strings.Contains(q.Document.Href, "nested.mxl") {
							data = packageGraphBytes(source, "main.musicxml", "")
							rep = validation.ResourceRawMXL
							id = "nested-package"
						}
						body := &resourceTestBody{Reader: bytes.NewReader(data)}
						bodies = append(bodies, body)
						return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Pin: validation.ResourcePin{CanonicalID: id, Representation: rep, Digest: fmt.Sprintf("%x", sha256.Sum256(data))}, RetrievalURI: q.Document.Href, Body: body}, nil
					}
					if q.Association == nil {
						t.Fatalf("unexpected callback %+v", q)
					}
					associationCalls++
					a := q.Association
					if a.PartID != "P1" || a.InstrumentID != "I1" || a.SourcePart.DocumentID != a.SourceInstrument.DocumentID || a.SourcePart.DocumentID != a.PartLink.DocumentID || a.SourcePart.DocumentID != a.InstrumentLink.DocumentID || a.Target.CanonicalID != "target-part" {
						t.Fatalf("association scope mismatch %+v", a)
					}
					if mode == "unknown" {
						return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceUnknown, Reason: "qa-partial-association-table"}, nil
					}
					v := validation.AssociationResult{Request: *a, Membership: packageKnown(mode == "positive"), Authority: validation.EvidenceAuthority{ID: "qa-association", ExactRelationComplete: true, Provenance: []validation.SourceRef{{IDs: []string{"qa-membership"}, URL: "https://final-qa.test/association", Locator: "exact positive and negative table"}}}, TargetPartID: validation.Evidence[string]{State: validation.EvidenceAbsent}, TargetInstrumentID: validation.Evidence[string]{State: validation.EvidenceAbsent}}
					return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceKnown, Association: &v}, nil
				})
				opts := finalQAOptions(resolver)
				if mode != "unselected" {
					opts.Dependencies = append(opts.Dependencies, ResourceAssociations)
				}
				actualSurface, input := surface, source
				if surface == "nested" {
					actualSurface = "xml"
					input = `<opus xmlns:xlink="http://www.w3.org/1999/xlink"><score xlink:href="https://final-qa.test/nested.mxl"/></opus>`
				}
				b, e := finalQARun(t, actualSurface, input, context.Background(), opts)
				wantDoc := 1
				if surface == "nested" {
					wantDoc = 2
				}
				wantAssoc := 1
				if mode == "unselected" {
					wantAssoc = 0
				}
				if e != nil || docCalls != wantDoc || associationCalls != wantAssoc {
					t.Fatalf("err=%v document=%d/%d association=%d/%d", e, docCalls, wantDoc, associationCalls, wantAssoc)
				}
				for _, body := range bodies {
					if body.closed != 1 {
						t.Fatal("body close", body.closed)
					}
				}
				findings := []validation.Finding{}
				for _, f := range b.PackageReport().Findings {
					if f.Rule.ID == "MX40-PACKAGE-instrument-association" {
						findings = append(findings, f)
					}
				}
				if mode == "positive" {
					if len(findings) != 0 {
						t.Fatalf("known membership unavailable %+v", findings)
					}
				} else {
					want := validation.OutcomeUnknown
					if mode == "negative" {
						want = validation.OutcomeFail
					}
					found := false
					for _, f := range findings {
						found = found || f.Outcome == want
					}
					if !found || b.Satisfied() {
						t.Fatalf("membership outcome lost %+v", findings)
					}
				}
			})
		}
	}
}

func TestFinalQAPrimaryRetrievalAdviceMatrix(t *testing.T) {
	for _, surface := range []string{"xml", "model", "raw-mxl", "current-mxl"} {
		for _, kind := range []string{"recommended", "legacy", "wrong", "absent", "unsupported"} {
			t.Run(surface+"/"+kind, func(t *testing.T) {
				name := "root.musicxml"
				compressed := surface == "raw-mxl" || surface == "current-mxl"
				if compressed {
					name = "root.mxl"
				}
				if kind == "legacy" {
					name = "root.xml"
				}
				if kind == "wrong" {
					name = "root.bad"
				}
				uri := "https://final-qa.test/" + name
				if kind == "absent" {
					uri = ""
				}
				if kind == "unsupported" {
					uri = "custom://host/" + name
				}
				opts := finalQAOptions(nil)
				opts.Identity = ResourceIdentity{CanonicalID: "qa-primary-id", RetrievalURI: uri}
				b, e := finalQARun(t, surface, `<opus/>`, context.Background(), opts)
				if e != nil || b.PackageReport().Counts.Fail != 0 {
					t.Fatalf("advice became normative: %v %+v", e, b.PackageReport())
				}
				found := false
				for i := 0; i < b.FileMetadataCount(); i++ {
					m, _ := b.FileMetadataAt(i)
					if m.Pin.CanonicalID != "qa-primary-id" {
						continue
					}
					found = true
					if m.Entry != 0 || m.Source != "" || m.BaseURI != uri || m.ArchiveEntry != "" || m.MediaType.State != validation.EvidenceAbsent {
						t.Fatalf("primary context invented %+v", m)
					}
					if kind == "unsupported" {
						if m.Name.State != validation.EvidenceUnknown {
							t.Fatal("unsupported name guessed", m)
						}
					} else if m.Name.State != validation.EvidenceKnown || m.Name.Value != name {
						t.Fatalf("name metadata %+v", m)
					}
					if surface == "model" || surface == "current-mxl" {
						if m.Pin.Digest != "" || m.Pin.SnapshotID == "" {
							t.Fatal("model source digest fabricated")
						}
					} else if len(m.Pin.Digest) != 64 {
						t.Fatal("source not verified")
					}
				}
				if found != (kind != "absent") {
					t.Fatalf("primary metadata present=%v kind=%s count=%d", found, kind, b.FileMetadataCount())
				}
				if kind == "wrong" || kind == "legacy" && compressed {
					if !finalQAFinding(b, "nonstandard-file-suffix", validation.OutcomeFail) {
						t.Fatal("missing suffix advice")
					}
				}
			})
		}
	}
}
