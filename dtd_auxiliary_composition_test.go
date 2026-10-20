package musicxml

import (
	"context"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func dtdAuxiliaryInspect(t *testing.T, id validation.ProfileID, input string) validation.Report {
	t.Helper()
	var r validation.Report
	var err error
	if id == dtdSoundsProfile {
		r, err = inspectXMLWithDictionaries(context.Background(), strings.NewReader(input), dictionaryAssessmentOptions{Assessment: assessmentOptions{Context: validation.ContextFamilyAll, Profile: id}})
	} else {
		r, err = InspectXML(context.Background(), strings.NewReader(input), SourceOptions{Context: validation.ContextFamilyAll, Profile: SourceProfile(id)})
	}
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestDTDAuxiliaryContainerPhysicalRawAndEffectiveValues(t *testing.T) {
	for _, tc := range []struct {
		name, decl, body, raw, effective string
		state                            validation.EvidenceState
	}{
		{"no DTD", "", `<rootfile full-path="  main.opus  "/>`, "  main.opus  ", "main.opus", validation.EvidenceKnown},
		{"tokenized", `<!ATTLIST rootfile full-path NMTOKENS #IMPLIED>`, `<rootfile full-path="  main.opus  "/>`, "  main.opus  ", "main.opus", validation.EvidenceKnown},
		{"CDATA", `<!ATTLIST rootfile full-path CDATA #IMPLIED>`, `<rootfile full-path="  main.opus  "/>`, "  main.opus  ", "main.opus", validation.EvidenceKnown},
		{"default", `<!ATTLIST rootfile full-path CDATA " main.opus ">`, `<rootfile/>`, "", "main.opus", validation.EvidenceAbsent},
		{"entity markup", `<!ENTITY entry "<rootfile full-path='main.opus'/>">`, `&entry;`, "", "main.opus", validation.EvidenceAbsent},
		{"entity attribute", `<!ENTITY file "main.opus">`, `<rootfile full-path="&file;"/>`, "", "main.opus", validation.EvidenceUnknown},
		{"empty", "", `<rootfile full-path="" media-type=""/>`, "", "", validation.EvidenceKnown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := `<container><rootfiles>` + tc.body + `</rootfiles></container>`
			if tc.decl != "" {
				input = `<!DOCTYPE container [` + tc.decl + `]>` + input
			}
			r := dtdAuxiliaryInspect(t, dtdContainerProfile, input)
			if !r.Satisfied() {
				t.Fatalf("not satisfied %+v", r.Findings)
			}
			found := false
			for _, row := range r.Context {
				if row.Container != nil && row.Container.Role == validation.ContainerRootfile {
					found = true
					v := row.Container
					if v.FullPathRaw.State != tc.state || v.FullPathRaw.Value != tc.raw || v.FullPath.State != validation.EvidenceKnown || v.FullPath.Value.Text != tc.effective {
						t.Fatalf("wrong physical/effective distinction %+v", v)
					}
				}
			}
			if !found {
				t.Fatal("rootfile missing")
			}
		})
	}
	r := dtdAuxiliaryInspect(t, dtdContainerProfile, `<container><rootfiles><rootfile/></rootfiles></container>`)
	if r.Conformance != validation.ConformanceViolated {
		t.Fatal("required path was invented")
	}
	for _, row := range r.Context {
		if row.Container != nil && row.Container.Role == validation.ContainerRootfile {
			v := row.Container
			if v.FullPathRaw.State != validation.EvidenceAbsent || v.FullPath.State != validation.EvidenceInvalid || v.FullPath.Cause == nil {
				t.Fatal("missing path cause", v)
			}
		}
	}
}

func TestDTDAuxiliaryHintsPhysicalDefaultedAndEntity(t *testing.T) {
	for _, assembly := range []struct {
		id         validation.ProfileID
		root, body string
	}{{dtdContainerProfile, "container", `<rootfiles><rootfile full-path="main.opus"/></rootfiles>`}, {dtdSoundsProfile, "sounds", `<sound id="custom.sound"/>`}} {
		for _, mode := range []string{"physical", "defaulted", "entity", "odd"} {
			t.Run(assembly.root+"/"+mode, func(t *testing.T) {
				hints := `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation=" urn:a &#x9; a.xsd  " xsi:noNamespaceSchemaLocation=" no.xsd "`
				input := `<` + assembly.root + ` ` + hints + `>` + assembly.body + `</` + assembly.root + `>`
				wantRaw := validation.EvidenceKnown
				wantPairs := 1
				if mode == "defaulted" || mode == "odd" {
					value := "urn:a a.xsd"
					if mode == "odd" {
						value += " trailing"
					}
					input = `<!DOCTYPE ` + assembly.root + ` [<!ATTLIST ` + assembly.root + ` xsi:schemaLocation CDATA "` + value + `" xsi:noNamespaceSchemaLocation CDATA "no.xsd">]><` + assembly.root + ` xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` + assembly.body + `</` + assembly.root + `>`
					wantRaw = validation.EvidenceAbsent
				}
				if mode == "entity" {
					child := "rootfile"
					body := `<rootfiles>&hint;</rootfiles>`
					attributes := `full-path='main.opus'`
					if assembly.root == "sounds" {
						child = "sound"
						body = `&hint;`
						attributes = `id='custom.sound'`
					}
					input = `<!DOCTYPE ` + assembly.root + ` [<!ENTITY hint "<` + child + ` ` + attributes + ` xsi:schemaLocation='urn:a a.xsd' xsi:noNamespaceSchemaLocation='no.xsd'/>">]><` + assembly.root + ` xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` + body + `</` + assembly.root + `>`
					wantRaw = validation.EvidenceAbsent
				}
				r := dtdAuxiliaryInspect(t, assembly.id, input)
				if r.Conformance == validation.ConformanceViolated {
					t.Fatalf("hints invented XSD failure %+v", r.Findings)
				}
				found := false
				for _, row := range r.Context {
					if row.Resource == nil || row.Resource.SchemaHints == nil {
						continue
					}
					v := row.Resource.SchemaHints
					if len(v.Pairs) != wantPairs {
						continue
					}
					found = true
					if v.Raw.State != wantRaw || v.Pairs[0].Namespace.Value != "urn:a" || v.Pairs[0].Location.Value != "a.xsd" || v.NoNamespace.Value.State != validation.EvidenceKnown || v.NoNamespace.Value.Value.Text != "no.xsd" {
						t.Fatalf("hint evidence %+v", v)
					}
					if wantRaw == validation.EvidenceAbsent && v.NoNamespace.Presence.State != validation.EvidenceAbsent {
						t.Fatalf("default/entity presence %+v", v)
					}
					if mode == "odd" && (v.Complete.State != validation.EvidenceKnown || v.Complete.Value || v.Trailing.Value != "trailing") {
						t.Fatalf("odd pairing %+v", v)
					}
				}
				if !found {
					t.Fatal("missing effective pairs")
				}
			})
		}
	}
}

func TestDTDAuxiliarySoundsEffectiveFieldsAndIndependentFailures(t *testing.T) {
	input := `<!DOCTYPE sounds [<!ATTLIST sound id CDATA "custom.sound" primary CDATA "yes"><!ATTLIST any primary CDATA "no"><!ATTLIST solo primary CDATA "yes"><!ATTLIST ensemble number NMTOKEN "many:players"><!ENTITY alternatives "<any>A  B&#x9;C</any><any/>">]><sounds><sound>&alternatives;</sound><sound id="second"><solo/></sound><sound id="third"><ensemble primary="yes"/></sound><sound id="fourth"/></sounds>`
	r := dtdAuxiliaryInspect(t, dtdSoundsProfile, input)
	if r.Conformance == validation.ConformanceViolated {
		t.Fatalf("effective mapping failed %+v", r.Findings)
	}
	rows := soundMappings(t, r)
	if len(rows) != 8 {
		t.Fatal("alternatives disappeared", len(rows))
	}
	if rows[0].SoundMapping.SoundID.Value.Text != "custom.sound" || rows[0].SoundMapping.Primary.Value.Text != "yes" || rows[1].SoundMapping.Primary.Value.Text != "no" || rows[1].SoundMapping.LibraryID.Value.Text != "A  B\tC" || rows[3].SoundMapping.Direction != validation.SoundMappingToMusicXML || rows[4].SoundMapping.Primary.Value.Text != "yes" || rows[6].SoundMapping.Number.Value.Text != "many:players" {
		t.Fatalf("effective fields %+v", rows)
	}
	for _, tc := range []struct{ name, decl, body string }{
		{"duplicate ID", `<!ATTLIST sound id CDATA "duplicate">`, `<sound/><sound/>`},
		{"invalid primary", `<!ATTLIST sound id CDATA "id" primary CDATA "maybe">`, `<sound/>`},
		{"invalid number", `<!ATTLIST ensemble number CDATA "two players">`, `<sound id="id"><ensemble/></sound>`},
		{"content", `<!ATTLIST sound id CDATA "id">`, `<sound><any/><solo/></sound>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := dtdAuxiliaryInspect(t, dtdSoundsProfile, `<!DOCTYPE sounds [`+tc.decl+`]><sounds>`+tc.body+`</sounds>`)
			if r.Conformance != validation.ConformanceViolated || r.Counts.Fail == 0 {
				t.Fatal("missing dictionary suppressed independent failure", r.Findings)
			}
		})
	}
	r = dtdAuxiliaryInspect(t, dtdSoundsProfile, `<sounds><sound id="id"><ensemble/></sound></sounds>`)
	for _, row := range soundMappings(t, r) {
		if row.SoundMapping.Primary.State != validation.EvidenceAbsent {
			t.Fatal("invented implicit primary", row.SoundMapping)
		}
	}
}
