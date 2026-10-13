package packageinventory

import (
	"encoding/json"
	"testing"
)

// Additional Go-port regressions complement the 43 one-for-one legacy cases.
func TestMaintenanceMalformedArchiveRoutingFailsClosed(t *testing.T) {
	cases := []struct {
		name   string
		change func(*maintenanceFixture)
	}{
		{"null external archive", func(f *maintenanceFixture) { f.m["external_archive"] = nil }},
		{"object external archive", func(f *maintenanceFixture) { f.m["external_archive"] = map[string]any{} }},
		{"nonrecord external artifact", func(f *maintenanceFixture) { f.m["external_archive"] = []any{"invalid"} }},
		{"missing external artifact path", func(f *maintenanceFixture) { f.m["external_archive"] = []any{map[string]any{}} }},
		{"numeric external artifact path", func(f *maintenanceFixture) { f.m["external_archive"] = []any{map[string]any{"artifact": 7}} }},
		{"null historical routes", func(f *maintenanceFixture) { f.e["historical_tests"] = nil }},
		{"array historical routes", func(f *maintenanceFixture) { f.e["historical_tests"] = []any{} }},
		{"missing historical artifact path", func(f *maintenanceFixture) {
			tests := mo(f.e["historical_tests"])
			delete(mo(tests[maintenanceFirst(tests)]), "artifact")
		}},
		{"numeric historical artifact path", func(f *maintenanceFixture) {
			tests := mo(f.e["historical_tests"])
			mo(tests[maintenanceFirst(tests)])["artifact"] = 7
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newMaintenanceFixture(t)
			tc.change(f)
			f.save()
			f.mutateTest()
			before := f.read(maintenanceEvidence)
			f.write("existing-result.json", []byte("preserve result"))
			for _, command := range []string{"verify", "refresh"} {
				t.Run(command, func(t *testing.T) {
					args := []string{"--archive-dir", f.root, "--output", f.path("existing-result.json")}
					if command == "refresh" {
						args = append(args, "--write")
					}
					result := f.run(t, command, 1, args...)
					if command == "refresh" {
						maintenanceEqual(t, result["written"], false)
					}
					maintenanceContains(t, maintenanceFailures(result), "malformed inputs")
					maintenanceEqual(t, f.read(maintenanceEvidence), before)
					maintenanceEqual(t, string(f.read("existing-result.json")), "preserve result")
				})
			}
		})
	}
}

func TestMaintenanceRefreshPreservesLargeIntegerTokens(t *testing.T) {
	f := newMaintenanceFixture(t)
	values := []any{json.Number("4255983792"), json.Number("9007199254740993"), json.Number("-9007199254740993"), json.Number("9223372036854775808123456789")}
	mo(f.e["snapshot"])["maintenance_numeric_preservation_probe"] = values
	f.save()
	f.mutateTest()
	f.run(t, "refresh", 0, "--write")
	actual := maintenanceJSON(t, f.read(maintenanceEvidence))
	maintenanceEqual(t, mo(actual["snapshot"])["maintenance_numeric_preservation_probe"], values)
	f.run(t, "verify", 0, "--strict-evidence")
}
