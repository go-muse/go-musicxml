package musicxml

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

type auxiliarySourceCase struct {
	name, assembly, source string
	valid                  bool
}

func auxiliarySourceCases() []auxiliarySourceCase {
	cases := []auxiliarySourceCase{
		{"container minimal", "container", `<container><rootfiles><rootfile full-path="score.musicxml"/></rootfiles></container>`, true},
		{"container multiple token values", "container", `<container><rootfiles><rootfile full-path="  a  b  " media-type=" application/custom "/><rootfile full-path="" media-type=""/></rootfiles></container>`, true},
		{"container percent literal", "container", `<container><rootfiles><rootfile full-path="a%20b.musicxml"/></rootfiles></container>`, true},
		{"container missing rootfiles", "container", `<container/>`, false},
		{"container empty rootfiles", "container", `<container><rootfiles/></container>`, false},
		{"container duplicate rootfiles", "container", `<container><rootfiles><rootfile full-path="a"/></rootfiles><rootfiles><rootfile full-path="b"/></rootfiles></container>`, false},
		{"container missing full path", "container", `<container><rootfiles><rootfile media-type="application/pdf"/></rootfiles></container>`, false},
		{"container unknown attribute", "container", `<container version="1"><rootfiles><rootfile full-path="a"/></rootfiles></container>`, false},
		{"container rootfiles attribute", "container", `<container><rootfiles id="x"><rootfile full-path="a"/></rootfiles></container>`, false},
		{"container rootfile attribute", "container", `<container><rootfiles><rootfile full-path="a" id="x"/></rootfiles></container>`, false},
		{"container empty content", "container", `<container><rootfiles><rootfile full-path="a">x</rootfile></rootfiles></container>`, false},
		{"container root text", "container", `<container>x<rootfiles><rootfile full-path="a"/></rootfiles></container>`, false},
		{"container rootfiles text", "container", `<container><rootfiles>x<rootfile full-path="a"/></rootfiles></container>`, false},
		{"container wrong root", "container", `<sounds><sound id="x"/></sounds>`, false},
		{"container foreign root", "container", `<container xmlns="urn:foreign"><rootfiles><rootfile full-path="a"/></rootfiles></container>`, false},
		{"container foreign child", "container", `<container><rootfiles><rootfile xmlns="urn:foreign" full-path="a"/></rootfiles></container>`, false},
		{"container qualified attribute", "container", `<container xmlns:q="urn:foreign"><rootfiles><rootfile q:full-path="a"/></rootfiles></container>`, false},
		{"container nil", "container", `<container xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"/>`, false},
		{"container same actual type", "container", `<container xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="container"><rootfiles><rootfile full-path="a"/></rootfiles></container>`, true},
		{"container wrong actual type", "container", `<container xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="rootfiles"><rootfile full-path="a"/></container>`, false},
		{"sounds empty mapping", "sounds", `<sounds><sound id="arbitrary.dictionary.extension"/></sounds>`, true},
		{"sounds repeated any", "sounds", `<sounds><sound id="a" primary="yes"><any primary="yes"> A  B </any><any primary="yes">A</any><any/></sound><sound id="b" primary="yes"><any>A</any></sound></sounds>`, true},
		{"sounds solo ensemble branch", "sounds", `<sounds><sound id="a" primary="no"><solo primary="yes">Solo</solo><solo/><ensemble number=" many:players " primary="no">Ensemble</ensemble><ensemble number="2"/></sound></sounds>`, true},
		{"sounds only solo", "sounds", `<sounds><sound id="a"><solo/><solo/></sound></sounds>`, true},
		{"sounds only ensemble", "sounds", `<sounds><sound id="a"><ensemble/><ensemble number="-0"/></sound></sounds>`, true},
		{"sounds empty root", "sounds", `<sounds/>`, false},
		{"sounds missing id", "sounds", `<sounds><sound/></sounds>`, false},
		{"sounds invalid id", "sounds", `<sounds><sound id="1bad"/></sounds>`, false},
		{"sounds duplicate id", "sounds", `<sounds><sound id="a"/><sound id=" a "/></sounds>`, false},
		{"sounds normalized id primary", "sounds", `<sounds><sound id=" a " primary=" yes "><any primary=" no "> x&#x9;y </any></sound></sounds>`, true},
		{"sounds invalid reverse primary", "sounds", `<sounds><sound id="a" primary="true"/></sounds>`, false},
		{"sounds invalid any primary", "sounds", `<sounds><sound id="a"><any primary="true"/></sound></sounds>`, false},
		{"sounds invalid solo primary", "sounds", `<sounds><sound id="a"><solo primary="true"/></sound></sounds>`, false},
		{"sounds invalid ensemble primary", "sounds", `<sounds><sound id="a"><ensemble primary="true"/></sound></sounds>`, false},
		{"sounds number empty", "sounds", `<sounds><sound id="a"><ensemble number=""/></sound></sounds>`, false},
		{"sounds number whitespace", "sounds", `<sounds><sound id="a"><ensemble number="two players"/></sound></sounds>`, false},
		{"sounds mixed branches", "sounds", `<sounds><sound id="a"><any/><solo/></sound></sounds>`, false},
		{"sounds reversed sequence", "sounds", `<sounds><sound id="a"><ensemble/><solo/></sound></sounds>`, false},
		{"sounds any nested content", "sounds", `<sounds><sound id="a"><any><solo/></any></sound></sounds>`, false},
		{"sounds solo nested content", "sounds", `<sounds><sound id="a"><solo><any/></solo></sound></sounds>`, false},
		{"sounds ensemble nested content", "sounds", `<sounds><sound id="a"><ensemble><any/></ensemble></sound></sounds>`, false},
		{"sounds root text", "sounds", `<sounds>x<sound id="a"/></sounds>`, false},
		{"sounds sound text", "sounds", `<sounds><sound id="a">x</sound></sounds>`, false},
		{"sounds unknown root attribute", "sounds", `<sounds version="4.0"><sound id="a"/></sounds>`, false},
		{"sounds unknown sound attribute", "sounds", `<sounds><sound id="a" tempo="1"/></sounds>`, false},
		{"sounds unknown any attribute", "sounds", `<sounds><sound id="a"><any number="1"/></sound></sounds>`, false},
		{"sounds unknown solo attribute", "sounds", `<sounds><sound id="a"><solo number="1"/></sound></sounds>`, false},
		{"sounds unknown ensemble attribute", "sounds", `<sounds><sound id="a"><ensemble id="b"/></sound></sounds>`, false},
		{"sounds foreign root", "sounds", `<sounds xmlns="urn:foreign"><sound id="a"/></sounds>`, false},
		{"sounds foreign mapping", "sounds", `<sounds><sound id="a"><any xmlns="urn:foreign"/></sound></sounds>`, false},
		{"sounds foreign primary", "sounds", `<sounds xmlns:q="urn:foreign"><sound id="a" q:primary="yes"/></sounds>`, false},
		{"sounds wrong root", "sounds", `<container><rootfiles><rootfile full-path="a"/></rootfiles></container>`, false},
		{"sounds nil", "sounds", `<sounds xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:nil="true"/>`, false},
		{"sounds same actual type", "sounds", `<sounds xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="sounds"><sound id="a"/></sounds>`, true},
		{"sounds wrong actual type", "sounds", `<sounds xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:type="sound" id="a"/>`, false},
	}
	for _, tc := range []struct{ name, assembly, source string }{
		{"container rootfiles nil", "container", `<container xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><rootfiles xsi:nil="true"><rootfile full-path="a"/></rootfiles></container>`},
		{"container rootfile nil", "container", `<container xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><rootfiles><rootfile full-path="a" xsi:nil="true"/></rootfiles></container>`},
		{"container qualified media type", "container", `<container xmlns:q="urn:foreign"><rootfiles><rootfile full-path="a" q:media-type="application/pdf"/></rootfiles></container>`},
		{"sounds sound nil", "sounds", `<sounds xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><sound id="a" xsi:nil="true"/></sounds>`},
		{"sounds any nil", "sounds", `<sounds xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><sound id="a"><any xsi:nil="true"/></sound></sounds>`},
		{"sounds solo nil", "sounds", `<sounds xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><sound id="a"><solo xsi:nil="true"/></sound></sounds>`},
		{"sounds ensemble nil", "sounds", `<sounds xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"><sound id="a"><ensemble xsi:nil="true"/></sound></sounds>`},
	} {
		cases = append(cases, auxiliarySourceCase{tc.name, tc.assembly, tc.source, false})
	}
	return cases
}

func TestAuxiliarySourceAssemblies(t *testing.T) {
	for _, tc := range auxiliarySourceCases() {
		t.Run(tc.name, func(t *testing.T) {
			report, err := InspectXML(context.Background(), strings.NewReader(tc.source), AssessmentOptions{Profile: validation.ProfileID("musicxml-4.0/source-" + tc.assembly)})
			if strings.HasSuffix(tc.name, "wrong root") || strings.HasSuffix(tc.name, "foreign root") {
				if !errors.Is(err, ErrUnsupportedRoot) || report.Satisfied() {
					t.Fatalf("root profile admission: %v %+v", err, report)
				}
				return
			}
			if err != nil {
				t.Fatalf("profile unavailable or operation failed: %v", err)
			}
			if report.Satisfied() != tc.valid {
				t.Fatalf("Satisfied=%v want %v: %+v", report.Satisfied(), tc.valid, report)
			}
			if !tc.valid && report.Conformance != validation.ConformanceViolated {
				t.Fatalf("missing normative failure: %+v", report)
			}
		})
	}
}

func TestAuxiliarySourceAgainstOriginalSchema(t *testing.T) {
	xmllint, err := exec.LookPath("xmllint")
	if err != nil {
		if os.Getenv("MUSICXML_REQUIRE_XMLLINT") == "1" {
			t.Fatal(err)
		}
		t.Skip("xmllint is unavailable")
	}
	for _, tc := range auxiliarySourceCases() {
		t.Run(tc.name, func(t *testing.T) {
			assertXMLLintOutcome(t, xmllint, filepath.Join("schema", "musicxml-4.0", tc.assembly+".xsd"), filepath.Join("schema", "musicxml-4.0", "catalog.xml"), tc.source, tc.valid)
		})
	}
}

func TestAuxiliaryGeneratedSchemaMetadata(t *testing.T) {
	for _, tc := range []struct {
		assembly validation.Assembly
		root     string
		data     validation.SchemaData
	}{{validation.AssemblyContainer, "container", containerValidationSchema}, {validation.AssemblySounds, "sounds", soundsValidationSchema}} {
		root := tc.data.Elements[validationQName{Local: tc.root}]
		if root == nil || root.Nillable || root.Abstract || root.Block != validationDerivationExtension|validationDerivationSubstitution || root.Final != validationDerivationExtension|validationDerivationRestriction {
			t.Fatalf("%s root declaration lost nil/abstract/block/final contract: %+v", tc.root, root)
		}
		schema, err := validation.CompileSchema(tc.assembly, tc.data)
		if err != nil {
			t.Fatal(err)
		}
		for _, u := range schema.Uses() {
			if u.Declaration.Assembly != tc.assembly || u.DeclaredTypeKnown && u.DeclaredType.Assembly != tc.assembly || u.OwnerTypeKnown && u.OwnerType.Assembly != tc.assembly {
				t.Errorf("cross-assembly metadata: %+v", u)
			}
		}
	}
	// The same-spelled types retain different content/category domains.
	if soundsValidationSchema.Types[validationQName{Local: "ensemble"}].Complex.Base.Name.Local != "string" || scoreValidationSchema.Types[validationQName{Local: "positive-integer-or-empty"}].Simple == nil {
		t.Fatal("sounds and score ensemble homonyms collapsed")
	}
}
