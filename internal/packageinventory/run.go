package packageinventory

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Run executes verify, refresh or test and writes its structured result to stdout.
// Usage errors return 2; inventory/I/O failures return 1. No command executes
// production MusicXML validation or promotes historical acceptance evidence.
func Run(args []string, stdout, stderr io.Writer) int {
	r := runner{fs: defaultFileOps()}
	return r.run(args, stdout, stderr)
}
func (r *runner) run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: package-inventory {verify|refresh|test} [options]")
		return 2
	}
	command := args[0]
	if command == "-h" || command == "--help" || command == "help" {
		fmt.Fprintln(stderr, "usage: package-inventory {verify|refresh|test} [options]")
		for _, name := range []string{"verify", "refresh", "test"} {
			fmt.Fprintln(stderr)
			var help runner
			help.commandFlags(name, stderr).Usage()
		}
		return 0
	}
	if command != "verify" && command != "refresh" && command != "test" {
		fmt.Fprintf(stderr, "unknown command: %s\n", command)
		return 2
	}
	flags := r.commandFlags(command, stderr)
	if err := flags.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintf(stderr, "unexpected arguments: %s\n", strings.Join(flags.Args(), " "))
		return 2
	}
	var result map[string]any
	switch command {
	case "verify":
		result = r.verify()
	case "refresh":
		result = r.refresh()
	case "test":
		result = guard(r.controls)
	}
	return r.emitResult(result, stdout)
}
func (r *runner) commandFlags(command string, stderr io.Writer) *flag.FlagSet {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&r.opts.root, "root", ".", "repository root")
	flags.StringVar(&r.opts.manifest, "manifest", defaultManifest, "ownership manifest")
	flags.StringVar(&r.opts.evidence, "evidence", defaultEvidence, "row evidence")
	flags.StringVar(&r.opts.output, "output", "", "atomic result JSON output (never an input)")
	if command != "test" {
		flags.StringVar(&r.opts.archive, "archive-dir", "", "external archive directory (raw bytes)")
	}
	if command == "verify" {
		flags.BoolVar(&r.opts.strict, "strict-evidence", false, "fail for changed inspected test bytes")
	}
	if command == "refresh" {
		flags.BoolVar(&r.opts.write, "write", false, "update only current tests[].file_sha256 after complete preflight")
	}
	flags.Usage = func() {
		fmt.Fprintf(stderr, "usage: package-inventory %s [options]\n", command)
		flags.PrintDefaults()
	}
	return flags
}
func resultFailures(result map[string]any) []string {
	if failures, ok := result["failures"].([]string); ok {
		return failures
	}
	return []string{}
}
func addFailure(result map[string]any, err error) {
	result["result"] = "FAIL"
	if _, ok := result["status"]; ok {
		result["status"] = "FAIL"
	}
	result["failures"] = append(resultFailures(result), err.Error())
}
func (r *runner) emitResult(result map[string]any, stdout io.Writer) int {
	if r.opts.output != "" {
		err := rejectAlias(r.opts.output, r.protected, r.fs)
		if err == nil && !r.known {
			err = errors.New("cannot write result: malformed inputs prevent complete output-alias protection; JSON is available on stdout")
		}
		if err == nil {
			err = atomicWrite(r.opts.output, prettyJSON(result), r.fs)
		}
		if err != nil {
			addFailure(result, err)
		}
	}
	if _, err := stdout.Write(prettyJSON(result)); err != nil {
		return 1
	}
	if len(resultFailures(result)) > 0 {
		return 1
	}
	return 0
}
func catch(fn func()) (err error) {
	defer func() {
		if v := recover(); v != nil {
			if e, ok := v.(inventoryError); ok {
				err = e
			} else {
				panic(v)
			}
		}
	}()
	fn()
	return nil
}
func (r *runner) refresh() map[string]any {
	result := r.verify()
	mode := "dry-run"
	if r.opts.write {
		mode = "write"
	}
	result["mode"] = mode
	result["changed_test_files"] = []string{}
	result["changed_test_routes"] = []string{}
	result["written"] = false
	err := catch(func() {
		if r.opts.output != "" {
			must(rejectAlias(r.opts.output, r.protected, r.fs))
		}
		if len(resultFailures(result)) != 0 {
			return
		}
		changes := map[string]string{}
		routes := map[string]string{}
		tests := r.evidence.get("tests")
		for _, ref := range sortedKeys(tests.object()) {
			record := tests.get(ref)
			file := record.get("file").text()
			current := r.hashes[file]
			if current != record.get("file_sha256").text() {
				changes[file] = current
				routes[ref] = current
			}
		}
		result["changed_test_files"] = sortedKeys(changes)
		result["changed_test_routes"] = sortedKeys(routes)
		if r.opts.write && len(changes) > 0 {
			protected := []string{}
			for _, p := range r.protected {
				if p != r.evidencePath {
					protected = append(protected, p)
				}
			}
			must(rejectAlias(r.evidencePath, protected, r.fs))
			data := replaceCurrentHashes(r.evidenceBytes, routes)
			must(atomicWrite(r.evidencePath, data, r.fs))
			result["written"] = true
		}
		if result["written"] == true {
			result["result"] = "PASS: current test hashes written; no execution acceptance"
		} else {
			result["result"] = "PASS: current test hash preview; no execution acceptance"
		}
	})
	if err != nil {
		addFailure(result, err)
	}
	return result
}

// Patch only the already-validated string tokens. Unlike reserializing a map,
// this preserves unknown metadata, key order, whitespace and all other bytes.
func replaceCurrentHashes(raw []byte, changes map[string]string) []byte {
	type patch struct {
		start, end int
		text       []byte
	}
	patches := []patch{}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var walk func([]string)
	walk = func(route []string) {
		start := int(d.InputOffset())
		tok, err := d.Token()
		must(err)
		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{':
				for d.More() {
					key, err := d.Token()
					must(err)
					walk(append(append([]string{}, route...), key.(string)))
				}
				_, err = d.Token()
				must(err)
			case '[':
				for d.More() {
					walk(append(append([]string{}, route...), "[]"))
				}
				_, err = d.Token()
				must(err)
			}
			return
		}
		if len(route) == 3 && route[0] == "tests" && route[2] == "file_sha256" {
			if value, ok := changes[route[1]]; ok {
				end := int(d.InputOffset())
				for start < end && raw[start] != '"' {
					start++
				}
				var b bytes.Buffer
				appendJSONString(&b, value)
				patches = append(patches, patch{start, end, b.Bytes()})
			}
		}
	}
	walk(nil)
	if len(patches) != len(changes) {
		fail("cannot refresh current hashes: token count mismatch")
	}
	out := append([]byte(nil), raw...)
	for i := len(patches) - 1; i >= 0; i-- {
		p := patches[i]
		out = append(out[:p.start], append(p.text, out[p.end:]...)...)
	}
	parseJSON(out, "refreshed evidence")
	return out
}
func parseVerifierResult(stdout, stderr []byte, exitCode int, name string, expected int) (result map[string]any, err error) {
	detail := fmt.Sprintf("%s: expected exit %d, exit %d; stderr=%q", name, expected, exitCode, stderr)
	err = catch(func() {
		v := parseJSON(stdout, "verifier stdout")
		var ok bool
		result, ok = v.(map[string]any)
		if !ok {
			fail("invalid verifier result shape")
		}
		if _, ok := result["failures"].([]any); !ok {
			fail("invalid verifier result shape")
		}
	})
	if err != nil {
		return nil, fmt.Errorf("%s; invalid verifier JSON: %w; stdout=%q", detail, err, stdout)
	}
	if exitCode != expected {
		return nil, fmt.Errorf("%s; result=%s", detail, stdout)
	}
	return result, nil
}
func (r *runner) controls() map[string]any {
	root := r.loadInputs()
	if r.opts.output != "" {
		must(rejectAlias(r.opts.output, r.protected, r.fs))
	}
	originals := map[string][]byte{}
	for name := range r.manifest.get("source_files").object() {
		originals[name] = canonicalLF(readBytes(repositoryPath(root, name)))
	}
	for _, v := range r.evidence.get("tests").object() {
		name := (node{v, "test record"}).get("file").text()
		originals[name] = canonicalLF(readBytes(repositoryPath(root, name)))
	}
	tmp, err := os.MkdirTemp("", "package-inventory-checkout-")
	must(err)
	defer os.RemoveAll(tmp)
	cases := []struct {
		name     string
		expected int
	}{{"LF", 0}, {"mixed", 0}, {"CRLF", 0}, {"mutated-source-whitespace", 1}, {"mutated-test-content", 1}, {"lone-CR-not-normalized", 1}, {"promoted-original-status", 1}, {"invented-test-route", 1}}
	results := []any{}
	for _, c := range cases {
		for _, name := range sortedKeys(originals) {
			data := originals[name]
			if c.name == "CRLF" {
				data = bytes.ReplaceAll(data, []byte("\n"), []byte("\r\n"))
			} else if c.name == "mixed" {
				data = bytes.Replace(data, []byte("\n"), []byte("\r\n"), 1)
			}
			dest := repositoryPath(tmp, name)
			must(os.MkdirAll(filepath.Dir(dest), 0700))
			must(os.WriteFile(dest, data, 0600))
		}
		m := node{parseJSON(prettyJSON(r.manifest.value), "manifest fixture"), "manifest fixture"}
		e := node{parseJSON(prettyJSON(r.evidence.value), "evidence fixture"), "evidence fixture"}
		source := filepath.Join(tmp, "docs/validation/plan/context-dispositions.json")
		switch c.name {
		case "mutated-source-whitespace":
			must(os.WriteFile(source, append(readBytes(source), ' '), 0600))
		case "lone-CR-not-normalized":
			must(os.WriteFile(source, append(readBytes(source), '\r'), 0600))
		case "mutated-test-content":
			p := filepath.Join(tmp, "link_intent_test.go")
			must(os.WriteFile(p, append(readBytes(p), []byte("// Actual content mutation.\n")...), 0600))
		case "promoted-original-status":
			m.get("preserved_collections").get("requirements").get("records").at(0).get("original_status_fields").object()["status"] = "implemented"
		case "invented-test-route":
			claims := e.get("rows").get("CHECK-context-group-link-match").get("axes").get("positive")
			claims.object()["tests"] = append(claims.get("tests").list(), "missing_test.go::TestInvented")
		}
		mp, ep := filepath.Join(tmp, "manifest.json"), filepath.Join(tmp, "evidence.json")
		must(os.WriteFile(mp, prettyJSON(m.value), 0600))
		must(os.WriteFile(ep, prettyJSON(e.value), 0600))
		v := runner{fs: defaultFileOps(), opts: options{root: tmp, manifest: mp, evidence: ep, strict: true}}
		result := v.verify()
		code := 0
		if len(resultFailures(result)) > 0 {
			code = 1
		}
		parsed, err := parseVerifierResult(prettyJSON(result), nil, code, c.name, c.expected)
		must(err)
		results = append(results, map[string]any{"case": c.name, "exit_code": code, "expected_exit_code": c.expected, "failures": parsed["failures"]})
	}
	return map[string]any{"status": "PASS", "hash_convention": "CRLF-to-LF only; all other bytes remain significant", "original_value_preservation": "All fixture edits are in temporary copies; source IDs/statuses/provenance untouched", "scope": "Inventory checker controls only, no production/runtime acceptance", "cases": results, "failures": []string{}}
}
