package musicxml

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestDictionaryGraphProfilesPinExactIntrinsicTable(t *testing.T) {
	_, profiles, err := buildDictionaryPackageAssessmentDefinitions()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 3 {
		t.Fatalf("profiles=%d", len(profiles))
	}
	for _, binding := range profiles {
		want := map[[2]string]validation.ProfileID{
			{string(validation.AssemblyMusicXML), string(validation.TargetSource)}:  ProfileSourceDictionaries.ID(),
			{string(validation.AssemblyOpus), string(validation.TargetSource)}:      ProfileSourceDictionaries.ID(),
			{string(validation.AssemblyContainer), string(validation.TargetSource)}: ProfileSourceContainer.ID(),
			{string(validation.AssemblySounds), string(validation.TargetSource)}:    ProfileSourceSoundsDictionaries.ID(),
		}
		if binding.Profile.ID != ProfileMXLSourceDictionaries.ID() {
			want[[2]string{string(validation.AssemblyMusicXML), string(validation.TargetModel)}] = ProfileModelDictionaries.ID()
			want[[2]string{string(validation.AssemblyOpus), string(validation.TargetModel)}] = ProfileModelDictionaries.ID()
		}
		if len(binding.Intrinsic) != len(want) {
			t.Errorf("%s rows=%d want=%d", binding.Profile.ID, len(binding.Intrinsic), len(want))
		}
		for _, row := range binding.Intrinsic {
			key := [2]string{string(row.Assembly), string(row.Target)}
			if row.Profile != want[key] {
				t.Errorf("unexpected row %+v", row)
			}
			var engine *validation.Engine
			if row.Target == validation.TargetSource {
				engine, err = sourceAssessmentEngineForProfile(row.Profile)
			} else {
				engine, err = modelAssessmentEngineForProfile(row.Profile)
			}
			if err != nil {
				t.Fatal(err)
			}
			profile, ok := engine.Profile(row.Profile)
			if !ok || row.CatalogDigest != profile.CatalogDigest {
				t.Errorf("unpinned intrinsic %+v", row)
			}
			delete(want, key)
		}
		if len(want) != 0 {
			t.Error("missing rows", want)
		}
	}
}

func TestDictionaryGraphEntryPointsKeepRepresentationAndProfiles(t *testing.T) {
	ctx := context.Background()
	doc, err := Decode(strings.NewReader(`<opus/>`))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name           string
		run            func() (validation.AssessmentBundle, error)
		target         validation.Target
		profile, graph validation.ProfileID
		count          int
	}{
		{"source", func() (validation.AssessmentBundle, error) {
			return inspectXMLWithResourcesAndDictionaries(ctx, strings.NewReader(`<opus/>`), linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraphDictionaries}}.internal()})
		}, validation.TargetSource, ProfileSourceDictionaries.ID(), ProfileResourceGraphDictionaries.ID(), 1},
		{"model", func() (validation.AssessmentBundle, error) {
			return assessModelWithResourcesAndDictionaries(ctx, doc, linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraphDictionaries}}.internal()})
		}, validation.TargetModel, ProfileModelDictionaries.ID(), ProfileResourceGraphDictionaries.ID(), 1},
		{"raw MXL", func() (validation.AssessmentBundle, error) {
			return inspectMXLWithDictionaries(ctx, bytes.NewReader(packageGraphBytes(`<opus/>`, "main.opus", "")), mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLSourceDictionaries}}.internal()})
		}, validation.TargetSource, ProfileSourceDictionaries.ID(), ProfileMXLSourceDictionaries.ID(), 2},
		{"current MXL", func() (validation.AssessmentBundle, error) {
			return assessMXLPackageWithDictionaries(ctx, &MXLPackage{Document: doc}, mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLModelDictionaries}}.internal()})
		}, validation.TargetModel, ProfileModelDictionaries.ID(), ProfileMXLModelDictionaries.ID(), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, e := tc.run()
			if e != nil {
				t.Fatal(e)
			}
			if b.ResourceCount() != tc.count || b.PackageReport().Profile.ID != tc.graph || !b.EnumerationComplete() {
				t.Fatalf("resources=%d profile=%s enumeration=%v", b.ResourceCount(), b.PackageReport().Profile.ID, b.EnumerationComplete())
			}
			found := false
			for i := 0; i < b.ResourceCount(); i++ {
				_, r, _ := b.ResourceReport(i)
				if r.Profile.ID == tc.profile {
					found = true
					if r.Target != tc.target {
						t.Error("wrong representation", r.Target)
					}
				} else if r.Profile.ID != ProfileSourceContainer.ID() {
					t.Error("unexpected intrinsic", r.Profile.ID)
				}
			}
			if !found {
				t.Error("missing primary profile")
			}
		})
	}
}

type dictionaryGraphSourceFunc func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error)

func (f dictionaryGraphSourceFunc) Open(ctx context.Context, q DictionaryComponentRequest) (DictionaryComponentResult, error) {
	return f(ctx, q)
}

type dictionaryGraphReader struct{ reads int }

func (r *dictionaryGraphReader) Read([]byte) (int, error) { r.reads++; return 0, io.EOF }

func TestDictionaryGraphConfigurationBeforeAnyIO(t *testing.T) {
	calls := 0
	source := dictionaryGraphSourceFunc(func(context.Context, DictionaryComponentRequest) (DictionaryComponentResult, error) {
		calls++
		return DictionaryComponentResult{}, errors.New("unexpected I/O")
	})
	for _, tc := range []struct {
		name    string
		options linkedDictionaryAssessmentOptions
	}{
		{"old graph profile", linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraph}}.internal()}},
		{"unknown manifest", linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraphDictionaries}}.internal(), Dictionaries: DictionaryOptions{Selections: []validation.DictionarySelection{{Manifest: "unknown", Relations: []validation.DictionaryRelation{"canonical-name"}}}}}},
		{"bad dependency", linkedDictionaryAssessmentOptions{Resources: LinkedAssessmentOptions{Graph: GraphOptions{Profile: ProfileResourceGraphDictionaries}, Dependencies: []ResourceDependency{"invalid"}}.internal()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &dictionaryGraphReader{}
			tc.options.Dictionaries.Source = source
			if len(tc.options.Dictionaries.Selections) == 0 {
				tc.options.Dictionaries.Selections = dictionaryGraphSelection()
			}
			_, err := inspectXMLWithResourcesAndDictionaries(context.Background(), r, tc.options)
			if !errors.Is(err, validation.ErrInvalidConfiguration) || r.reads != 0 || calls != 0 {
				t.Fatalf("err=%v reads=%d callbacks=%d", err, r.reads, calls)
			}
		})
	}
	r := &dictionaryGraphReader{}
	_, err := inspectMXLWithDictionaries(context.Background(), r, mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLSourceDictionaries}, MXL: MXLOptions{MaxXMLDepth: -1}}.internal(), Dictionaries: DictionaryOptions{Source: source, Selections: dictionaryGraphSelection()}})
	if err == nil || r.reads != 0 || calls != 0 {
		t.Fatalf("invalid MXL: %v %d %d", err, r.reads, calls)
	}
}

func TestDictionaryGraphPreservesPackageRoleSelection(t *testing.T) {
	ctx := context.Background()
	doc, err := Decode(strings.NewReader(`<opus/>`))
	if err != nil {
		t.Fatal(err)
	}
	for _, current := range []bool{false, true} {
		calls := 0
		resolver := graphResolverFunc(func(_ context.Context, q validation.ResourceRequest) (validation.ResourceResult, error) {
			calls++
			if q.Roles == nil {
				t.Errorf("unexpected dependency request: %+v", q)
			}
			return validation.ResourceResult{RequestID: q.ID, State: validation.EvidenceUnknown, Reason: "test-role-unavailable"}, nil
		})
		opts := mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLSourceDictionaries}, Resolver: resolver, Dependencies: []ResourceDependency{ResourceRoles}}.internal()}
		var e error
		if current {
			opts.Resources.Graph.Profile = ProfileMXLModelDictionaries.ID()
			_, e = assessMXLPackageWithDictionaries(ctx, &MXLPackage{Document: doc}, opts)
		} else {
			_, e = inspectMXLWithDictionaries(ctx, bytes.NewReader(packageGraphBytes(`<opus/>`, "main.opus", "")), opts)
		}
		if e != nil || calls != 1 {
			t.Errorf("current=%t err=%v role callbacks=%d", current, e, calls)
		}
	}
}

func TestDictionaryGraphPreservesMXLXMLDepth(t *testing.T) {
	root := `<opus><opus><opus><opus/></opus></opus></opus>`
	_, err := inspectMXLWithDictionaries(context.Background(), bytes.NewReader(packageGraphBytes(root, "main.opus", "")), mxlDictionaryAssessmentOptions{Resources: MXLAssessmentOptions{Graph: MXLGraphOptions{Profile: ProfileMXLSourceDictionaries}, MXL: MXLOptions{MaxXMLDepth: 3}}.internal()})
	if !errors.Is(err, ErrXMLTooDeep) || !errors.Is(err, validation.ErrResourceLimit) {
		t.Fatal("dictionary profile bypassed package XML cap", err)
	}
}
