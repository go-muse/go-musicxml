package musicxml

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestAuxiliaryOracleRequirementValue(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", "0", "true", "1"} {
		t.Run("value="+value, func(t *testing.T) {
			command := exec.Command(executable, "-test.run=^TestAuxiliarySourceAgainstOriginalSchema$", "-test.v")
			for _, entry := range os.Environ() {
				key, _, _ := strings.Cut(entry, "=")
				if !strings.EqualFold(key, "PATH") && key != "MUSICXML_REQUIRE_XMLLINT" {
					command.Env = append(command.Env, entry)
				}
			}
			command.Env = append(command.Env, "PATH="+t.TempDir(), "MUSICXML_REQUIRE_XMLLINT="+value)
			output, err := command.CombinedOutput()
			if value == "1" {
				if err == nil || !strings.Contains(string(output), "--- FAIL: TestAuxiliarySourceAgainstOriginalSchema") {
					t.Fatalf("required oracle did not fail without xmllint: %v\n%s", err, output)
				}
			} else if err != nil || !strings.Contains(string(output), "--- SKIP: TestAuxiliarySourceAgainstOriginalSchema") {
				t.Fatalf("nonrequired oracle value %q must skip without xmllint: %v\n%s", value, err, output)
			}
		})
	}
}

func TestReviewCatalogSourceLocators(t *testing.T) {
	for _, tc := range []struct {
		profile validation.ProfileID
		source  string
	}{
		{ProfileSourceContainer, `<container><rootfiles><rootfile full-path="score.musicxml"/></rootfiles></container>`},
		{ProfileSourceSounds, `<sounds><sound id="piano"/></sounds>`},
		{ProfileSourceLinks, `<opus version="4.0"/>`},
	} {
		t.Run(string(tc.profile), func(t *testing.T) {
			report, err := InspectXML(context.Background(), strings.NewReader(tc.source), AssessmentOptions{Profile: tc.profile})
			if err != nil {
				t.Fatal(err)
			}
			if tc.profile == ProfileSourceLinks {
				definitions, err := buildLinkAssessmentDefinitions()
				if err != nil {
					t.Fatal(err)
				}
				foundHint, foundLink, foundOpus := false, false, false
				for _, rule := range definitions.rules {
					for _, source := range rule.Sources {
						if strings.Contains(string(rule.Key.ID), "schema-hints/") {
							foundHint = true
							if source.Locator != "XML Schema 1.0 Second Edition Part 1 section 4.3.2" {
								t.Errorf("schema locator=%q", source.Locator)
							}
						}
						if strings.Contains(source.URL, "REC-xlink-20010627/#link-behaviors") {
							foundLink = true
							if source.Locator != "XLink 1.0 section 5.6 intended behavior; no application or retrieval authority" {
								t.Errorf("XLink locator=%q", source.Locator)
							}
						}
						if strings.Contains(source.Locator, "exact Opus counterpart") {
							foundOpus = true
							if source.Locator != "MusicXML link-attributes defaults; exact Opus counterpart opus.xsd lines 52-53" {
								t.Errorf("Opus locator=%q", source.Locator)
							}
						}
					}
				}
				if !foundHint || !foundLink || !foundOpus {
					t.Fatalf("missing source routes: hint=%v link=%v opus=%v", foundHint, foundLink, foundOpus)
				}
			}
			// These identities include the corrected locator strings. Earlier released
			// profiles are separately checked by the eight-profile frozen controls.
			want := "85518be7283021faf6b5080788c5efa0eebee1429d310f94d82fbdc5cbb232b2"
			if tc.profile == ProfileSourceLinks {
				want = migrateStructuralRolesCatalog(migrateValidationLinesCatalog(migrateDecimalContractCatalog("5fb200a688a054add8a551d288486d2724ff7d1530b42a3e0fbed40f3df46cb3")))
			}
			if report.Profile.CatalogDigest != want {
				t.Errorf("corrected source metadata digest=%s want %s", report.Profile.CatalogDigest, want)
			}
			t.Logf("%s digest %s", tc.profile, report.Profile.CatalogDigest)
		})
	}
}
