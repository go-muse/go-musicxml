package musicxml

import (
	"context"
	"github.com/go-muse/go-musicxml/internal/xsd"
	"sort"
	"strings"
	"testing"
)

type kernelTestMeter struct{}

func (kernelTestMeter) Step(ctx context.Context, _ int64) error { return ctx.Err() }
func (kernelTestMeter) Reserve(int64) error                     { return nil }
func (kernelTestMeter) Release(int64) error                     { return nil }
func (kernelTestMeter) Scalar(int64) error                      { return nil }
func (kernelTestMeter) Identity(int64) error                    { return nil }
func kernelSourceIssues(t *testing.T, compiled *xsd.Compiled, source string) ([]string, bool) {
	t.Helper()
	node, err := parseValidationDocument([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	r, err := xsd.NewRun(compiled, kernelTestMeter{})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	ids := map[*validationNode]xsd.SubjectID{}
	var next xsd.SubjectID
	var number func(*validationNode)
	number = func(n *validationNode) {
		next++
		ids[n] = next
		for _, c := range n.Children {
			number(c)
		}
	}
	number(node)
	var issues []string
	var assess func(*validationNode, *validationNode, string, xsd.Use) xsd.Completion
	assess = func(n, parent *validationNode, path string, use xsd.Use) xsd.Completion {
		s := xsd.Subject{ID: ids[n], Parent: ids[parent], Name: n.Name, Path: path, Namespaces: n.Namespaces, Text: n.Text.String(), TextKnown: true, AttributesComplete: true, ChildrenComplete: true}
		for _, a := range n.Attrs {
			a.ValueKnown = true
			s.Attributes = append(s.Attributes, a)
		}
		paths := validationChildPaths(path, n.Children)
		for i, ch := range n.Children {
			s.Children = append(s.Children, xsd.Child{ID: ids[ch], Name: ch.Name, Path: paths[i]})
		}
		b, e := r.Bind(context.Background(), use, s)
		if e != nil {
			t.Fatal(e)
		}
		v := r.Results(b)
		for _, family := range []xsd.Result{v.Binding, v.Declaration, v.Attributes, v.Scalar, v.Content} {
			for _, d := range family.Issues {
				issues = append(issues, d.Path+":"+d.Constraint)
			}
		}
		var completions []xsd.Completion
		for _, w := range r.Witnesses(b) {
			completions = append(completions, assess(n.Children[w.Index], n, paths[w.Index], w.Use))
		}
		c, e := r.Complete(context.Background(), b, completions)
		if e != nil {
			t.Fatal(e)
		}
		return c
	}
	use, ok := compiled.Root(node.Name)
	if !ok {
		t.Fatalf("missing root %v", node.Name)
	}
	assess(node, nil, "/"+node.Name.Local, use)
	identity, err := r.Finalize(context.Background(), true)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range identity.BySubject {
		for _, d := range result.Issues {
			issues = append(issues, d.Path+":"+d.Constraint)
		}
	}
	sort.Strings(issues)
	return issues, identity.Complete
}
func TestKernelSplitSourceMatrices(t *testing.T) {
	groups := []struct {
		name   string
		schema validationSchemaSet
		cases  []validationSourceCase
	}{
		{"integer", validationIntegerGenerated, append(validationIntegerBuiltinCases(), validationIntegerFacetCases()...)},
		{"decimal", validationDecimalGenerated, append(validationDecimalCases(), validationDecimalDigitCases()...)},
		{"whitespace", validationWhitespaceGenerated, validationWhitespaceCases()},
		{"pattern", validationPatternGenerated, validationPatternGroupCases()},
		{"list", validationBuiltinListGenerated, validationBuiltinListCases()},
		{"aggregate", validationAggregateGenerated, validationAggregateCases()},
		{"union", validationUnionMemberGenerated, validationUnionMemberCases()},
		{"primitive", validationPrimitiveGenerated, validationPrimitiveCases()},
		{"names", validationDatatypeNameGenerated, validationDatatypeNameCases()},
		{"simple-content", validationSimpleContentGenerated, validationSimpleContentCases()},
		{"grammar", validationGrammarGenerated, validationGrammarCases()},
		{"xsi", validationXSIGenerated, validationXSICases()},
		{"xsi-defaults", validationXSIDefaultsGenerated, validationXSIDefaultCases()},
		{"identity", validationIdentityGenerated, validationIdentityCases()},
	}
	for _, group := range groups {
		t.Run(group.name, func(t *testing.T) {
			compiled, err := xsd.Compile(group.schema, xsd.CompileLimits{})
			if err != nil {
				t.Fatal(err)
			}
			for _, tc := range group.cases {
				t.Run(tc.name, func(t *testing.T) {
					got, identityComplete := kernelSourceIssues(t, compiled, tc.source)
					want := append([]string(nil), tc.issues...)
					sort.Strings(want)
					// The fixture item declares omitted optional ID/IDREF defaults.
					// Source preview exposes this deferred identity participation,
					// unlike the intentionally bounded legacy contract above.
					if group.name == "identity" && kernelIdentityPreviewLimited[tc.name] {
						if identityComplete {
							t.Fatal("deferred identity closure claimed complete")
						}
						kept := want[:0]
						for _, issue := range want {
							if !strings.HasSuffix(issue, ":IDREF") {
								kept = append(kept, issue)
							}
						}
						want = kept
					}
					if len(got) != len(want) {
						t.Fatalf("issues=%v want=%v", got, want)
					}
					for i := range got {
						if got[i] != want[i] {
							t.Fatalf("issues=%v want=%v", got, want)
						}
					}
				})
			}
		})
	}
}

// These exact exceptions retain the legacy normative expectations in the
// original matrices; only the preview's inability to prove absence differs.
var kernelIdentityPreviewLimited = map[string]bool{
	"ordinary string": true, "ordinary NCName": true, "invalid lexical candidate": true,
	"invalid facet candidate": true, "invalid simple attribute candidate": true,
	"invalid complex attributes candidate": true, "nil scalar no candidate": true,
	"union member no binding": true, "list item no binding": true, "invalid attribute ID": true,
	"abstract actual type no candidate": true,
}
