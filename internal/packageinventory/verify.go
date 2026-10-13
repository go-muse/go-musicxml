package packageinventory

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf8"
)

const defaultManifest = "docs/validation/package-assessment-ownership.json"
const defaultEvidence = "docs/validation/evidence/package-row-evidence.json"

type options struct {
	root, manifest, evidence, archive, output string
	strict, write                             bool
}
type runner struct {
	fs                 fileOps
	opts               options
	protected          []string
	known              bool
	manifest, evidence node
	evidencePath       string
	evidenceBytes      []byte
	hashes             map[string]string
}

func (r *runner) inputPaths(root string, m, e any, mp, ep string) []string {
	paths := []string{mp, ep}
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, exe)
	}
	roots := []string{root}
	if _, file, _, ok := runtime.Caller(0); ok && filepath.IsAbs(file) {
		roots = append(roots, filepath.Dir(filepath.Dir(filepath.Dir(file))))
	}
	for _, base := range roots {
		for _, dir := range []string{"internal/packageinventory", "cmd/package-inventory"} {
			entries, err := os.ReadDir(filepath.Join(base, dir))
			if err != nil && !os.IsNotExist(err) {
				fail("cannot enumerate protected tool inputs: %s: %v", dir, err)
			}
			for _, entry := range entries {
				if strings.HasSuffix(entry.Name(), ".go") {
					paths = append(paths, filepath.Join(base, dir, entry.Name()))
				}
			}
		}
	}
	if m, ok := m.(map[string]any); ok {
		if source, ok := m["source_files"].(map[string]any); ok {
			for _, name := range sortedKeys(source) {
				paths = append(paths, repositoryPath(root, name))
			}
		}
		if corrections, ok := m["reviewed_source_corrections"].([]any); ok {
			for _, record := range corrections {
				if record, ok := record.(map[string]any); ok {
					if name, ok := record["reconciliation"].(string); ok {
						paths = append(paths, repositoryPath(root, name))
					}
				}
			}
		}
		if r.opts.archive != "" {
			if archive, ok := m["external_archive"].([]any); ok {
				for _, record := range archive {
					if record, ok := record.(map[string]any); ok {
						if name, ok := record["artifact"].(string); ok {
							paths = append(paths, repositoryPath(r.opts.archive, name))
						}
					}
				}
			}
		}
	}
	if e, ok := e.(map[string]any); ok {
		if tests, ok := e["tests"].(map[string]any); ok {
			for _, key := range sortedKeys(tests) {
				if record, ok := tests[key].(map[string]any); ok {
					if name, ok := record["file"].(string); ok {
						paths = append(paths, repositoryPath(root, name))
					}
				}
			}
		}
		if r.opts.archive != "" {
			if tests, ok := e["historical_tests"].(map[string]any); ok {
				for _, key := range sortedKeys(tests) {
					if record, ok := tests[key].(map[string]any); ok {
						if name, ok := record["artifact"].(string); ok {
							paths = append(paths, repositoryPath(r.opts.archive, name))
						}
					}
				}
			}
		}
	}
	return paths
}
func inputsKnown(m, e any, archive bool) bool {
	mm, ok := m.(map[string]any)
	if !ok {
		return false
	}
	ee, ok := e.(map[string]any)
	if !ok {
		return false
	}
	if _, ok := mm["source_files"].(map[string]any); !ok {
		return false
	}
	corrections, exists := mm["reviewed_source_corrections"]
	if exists {
		a, ok := corrections.([]any)
		if !ok {
			return false
		}
		for _, v := range a {
			record, ok := v.(map[string]any)
			if !ok {
				return false
			}
			if _, ok := record["reconciliation"].(string); !ok {
				return false
			}
		}
	}
	tests, ok := ee["tests"].(map[string]any)
	if !ok {
		return false
	}
	for _, v := range tests {
		record, ok := v.(map[string]any)
		if !ok {
			return false
		}
		if _, ok := record["file"].(string); !ok {
			return false
		}
	}
	if archive {
		records, ok := mm["external_archive"].([]any)
		if !ok {
			return false
		}
		historical, ok := ee["historical_tests"].(map[string]any)
		if !ok {
			return false
		}
		for _, v := range historical {
			records = append(records, v)
		}
		for _, v := range records {
			record, ok := v.(map[string]any)
			if !ok {
				return false
			}
			if _, ok := record["artifact"].(string); !ok {
				return false
			}
		}
	}
	return true
}
func (r *runner) loadInputs() string {
	root, err := resolveRoot(r.opts.root, r.fs)
	must(err)
	mp, ep := r.opts.manifest, r.opts.evidence
	if !filepath.IsAbs(mp) {
		mp = joinUnclean(root, mp)
	}
	if !filepath.IsAbs(ep) {
		ep = joinUnclean(root, ep)
	}
	r.protected = r.inputPaths(root, nil, nil, mp, ep)
	r.manifest = readJSON(mp)
	r.evidenceBytes = readBytes(ep)
	r.evidence = node{parseJSON(r.evidenceBytes, ep), ep}
	r.protected = r.inputPaths(root, r.manifest.value, r.evidence.value, mp, ep)
	r.known = inputsKnown(r.manifest.value, r.evidence.value, r.opts.archive != "")
	r.manifest.object()
	r.evidence.object()
	r.evidencePath = ep
	return root
}
func failureResult(err error) map[string]any {
	return map[string]any{"result": "FAIL", "failures": []string{err.Error()}, "warnings": []string{}, "semantic_resource_execution_acceptance": "NOT CLAIMED"}
}
func guard(fn func() map[string]any) (result map[string]any) {
	defer func() {
		if v := recover(); v != nil {
			if e, ok := v.(inventoryError); ok {
				result = failureResult(e)
			} else {
				panic(v)
			}
		}
	}()
	return fn()
}
func (r *runner) verify() map[string]any {
	return guard(func() map[string]any { return r.verifyLoaded(r.loadInputs()) })
}
func nodeSet(n node) map[string]bool {
	m := map[string]bool{}
	for _, v := range n.list() {
		m[(node{v, n.location}).text()] = true
	}
	return m
}
func keysSet(m map[string]any) map[string]bool {
	s := map[string]bool{}
	for k := range m {
		s[k] = true
	}
	return s
}
func (r *runner) verifyLoaded(root string) map[string]any {
	m, e := r.manifest, r.evidence
	failures, warnings, stale := []string{}, []string{}, []string{}
	check := func(ok bool, message string) {
		if !ok {
			failures = append(failures, message)
		}
	}
	m.fields(map[string]string{"hashing": "object", "source_files": "object", "preserved_collections": "object", "selected": "object", "counts": "object", "snapshot": "object", "external_archive": "array"})
	e.fields(map[string]string{"rows": "object", "tests": "object", "historical_tests": "object", "auxiliary_xsd_axes": "array", "hashing": "object"})
	for _, v := range m.optional("reviewed_source_corrections", []any{}).list() {
		(node{v, "reviewed_source_corrections record"}).get("reconciliation").text()
	}
	check(strings.HasPrefix(e.get("hashing").get("test_file_sha256").text(), "sha256-crlf-to-lf-v1;"), "unsupported or unspecified test hash convention")
	sources := m.get("source_files").object()
	check(len(sources) == 17, "source file denominator drift")
	check(m.get("hashing").get("repository_text").text() == "sha256-crlf-to-lf-v1", "unsupported or unspecified repository hash convention")
	loaded := map[string]node{}
	load := func(name string) node {
		if n, ok := loaded[name]; ok {
			return n
		}
		n := readJSON(repositoryPath(root, name))
		loaded[name] = n
		return n
	}
	resolve := func(source, pointer string) (value node) {
		location := source + "#" + strings.TrimPrefix(pointer, "#")
		defer func() {
			if v := recover(); v != nil {
				if err, ok := v.(inventoryError); ok {
					fail("unresolvable pointer: %s: %s", location, err.Error())
				}
				panic(v)
			}
		}()
		if _, ok := sources[source]; !ok {
			fail("source route is not declared in source_files: %s", source)
		}
		value = load(source)
		pointer = strings.TrimPrefix(pointer, "#")
		if pointer == "" {
			return value
		}
		if !strings.HasPrefix(pointer, "/") {
			fail("pointer must be empty or start with /")
		}
		for _, key := range strings.Split(pointer[1:], "/") {
			for i := 0; i < len(key); i++ {
				if key[i] == '~' {
					if i+1 >= len(key) || (key[i+1] != '0' && key[i+1] != '1') {
						fail("invalid JSON pointer escape")
					}
					i++
				}
			}
			key = strings.ReplaceAll(strings.ReplaceAll(key, "~1", "/"), "~0", "~")
			switch value.value.(type) {
			case []any:
				if !regexp.MustCompile(`^(0|[1-9][0-9]*)$`).MatchString(key) {
					fail("array index must be a nonnegative canonical integer")
				}
				index, err := strconv.ParseUint(key, 10, 64)
				if err != nil || index >= uint64(len(value.list())) {
					fail("array index out of range: %s", key)
				}
				value = value.at(int(index))
			case map[string]any:
				value = value.get(key)
			default:
				fail("cannot traverse a scalar")
			}
		}
		return value
	}
	routeSource := func(route node, key string) (string, string) {
		source := route.get(key).text()
		parts := strings.SplitN(source, "#", 2)
		if len(parts) != 2 {
			fail("unresolvable pointer: %s: missing # separator", source)
		}
		return parts[0], parts[1]
	}
	for _, name := range sortedKeys(sources) {
		record := node{sources[name], "source_files/" + name}
		record.fields(map[string]string{"sha256": "string", "bytes": "integer"})
		data := canonicalLF(readBytes(repositoryPath(root, name)))
		load(name)
		check(digest(data) == record.get("sha256").text(), "changed original source: "+name)
		check(int64(len(data)) == record.get("bytes").integer(), "changed original source size: "+name)
	}
	retained := map[string]int{}
	collections := m.get("preserved_collections")
	for _, name := range sortedKeys(collections.object()) {
		collection := collections.get(name)
		collection.fields(map[string]string{"source": "string", "pointer": "string", "records": "array", "count": "integer", "id_field": "string"})
		actual := resolve(collection.get("source").text(), collection.get("pointer").text())
		actual.list()
		rows := collection.get("records")
		retained[name] = len(rows.list())
		check(int64(len(rows.list())) == collection.get("count").integer() && len(rows.list()) == len(actual.list()), "record count: "+name)
		ids := map[string]bool{}
		indices := map[int64]bool{}
		for i := range rows.list() {
			route := rows.at(i)
			route.fields(map[string]string{"id": "string", "index": "integer", "sha256": "string"})
			id, index := route.get("id").text(), route.get("index").integer()
			ids[id] = true
			indices[index] = true
			check(index >= 0, "negative record index: "+id)
			if index < 0 || index >= int64(len(actual.list())) {
				continue
			}
			original := actual.at(int(index))
			original.object()
			check(original.get(collection.get("id_field").text()).value == id, "ID route: "+id)
			check(digest(canonicalJSON(original.value)) == route.get("sha256").text(), "original record: "+id)
			statuses := route.optional("original_status_fields", map[string]any{})
			for _, key := range sortedKeys(statuses.object()) {
				check(reflect.DeepEqual(original.optional(key, nil).value, statuses.get(key).value), "status promotion: "+id+"/"+key)
			}
		}
		check(len(ids) == len(rows.list()), "duplicate ID: "+name)
		complete := len(indices) == len(actual.list())
		for i := range actual.list() {
			complete = complete && indices[int64(i)]
		}
		check(complete, "lost index: "+name)
	}
	wanted := map[string]int{"requirements": 95, "context_checks": 307, "context_dispositions": 291, "constraint_checks": 51, "issue_gates": 28, "retained_splits": 20, "retained_source_contracts": 42, "retained_candidate_dispositions": 29, "source_subclauses": 2, "retained_boundaries": 2}
	check(reflect.DeepEqual(retained, wanted), "preserved denominator drift")
	countMatches := func(wanted map[string]int) bool {
		ok := true
		for _, key := range sortedKeys(wanted) {
			n := m.get("counts").get(key).integer()
			ok = ok && n == int64(wanted[key])
		}
		return ok
	}
	check(countMatches(wanted), "declared original denominator drift")
	check(countMatches(map[string]int{"selected_engineering": 44, "selected_context": 24, "selected_constraints": 10, "selected_issue_gates": 5, "selected_xsd_total": 175, "new_auxiliary_xsd": 36, "new_container_xsd": 11, "new_sounds_xsd": 25, "reused_transitive_xsd": 139, "selected_new_translations": 0}), "declared selected denominator drift")
	selected := m.get("selected")
	selectedCounts := map[string]int{}
	for _, category := range sortedKeys(selected.object()) {
		rows := selected.get(category)
		selectedCounts[category] = len(rows.list())
		for i := range rows.list() {
			route := rows.at(i)
			route.fields(map[string]string{"id": "string", "owner": "string"})
			check(strings.TrimSpace(route.get("owner").text()) != "", "missing selected owner: "+route.get("id").text())
		}
	}
	check(reflect.DeepEqual(selectedCounts, map[string]int{"requirements": 44, "context_checks": 24, "constraint_checks": 10, "issue_gates": 5, "xsd_instances": 175}), "selected denominator drift")
	selectedIDs := map[string]bool{}
	for _, category := range []string{"requirements", "context_checks", "constraint_checks"} {
		rows := selected.get(category)
		for i := range rows.list() {
			route := rows.at(i)
			id := route.get("id").text()
			check(!selectedIDs[id], "duplicate selected ID: "+id)
			selectedIDs[id] = true
			source, pointer := routeSource(route, "original_source")
			original := resolve(source, pointer)
			original.object()
			check(original.get("id").text() == id, "selected source ID: "+id)
			check(digest(canonicalJSON(original.value)) == route.get("original_record_sha256").text(), "selected original record: "+id)
			check(route.get("owner").text() != "" && route.get("bounded_contribution").text() != "", "missing selected owner/contribution: "+id)
		}
	}
	check(reflect.DeepEqual(selectedIDs, keysSet(e.get("rows").object())), "missing/extra row evidence")
	issueIDs := map[string]bool{}
	issues := selected.get("issue_gates")
	for i := range issues.list() {
		route := issues.at(i)
		id := route.get("id").text()
		issueIDs[id] = true
		source, pointer := routeSource(route, "source")
		original := resolve(source, pointer)
		original.object()
		check(original.get("id").text() == id, "issue ID: "+id)
		check(original.get("disposition").text() == "decision_required" && route.get("original_disposition").text() == "decision_required", "issue disposition promotion: "+id)
		check(original.get("hard_error_basis").value == false && route.get("original_hard_error_basis").value == false, "issue hard-error promotion: "+id)
	}
	check(len(issueIDs) == 5, "duplicate selected issue ID")
	xsd := selected.get("xsd_instances")
	xsdIDs, newIDs := map[string]bool{}, map[string]bool{}
	newCount, reused := 0, 0
	for i := range xsd.list() {
		route := xsd.at(i)
		id := route.get("id").text()
		xsdIDs[id] = true
		switch route.get("selection").text() {
		case "new_auxiliary":
			newCount++
			newIDs[id] = true
		case "reused_transitive":
			reused++
		}
		source, pointer := routeSource(route, "source")
		original := resolve(source, pointer)
		original.object()
		check(original.get("id").text() == id && digest(canonicalJSON(original.value)) == route.get("record_sha256").text(), "selected XSD record: "+id)
		check(reflect.DeepEqual(original.get("status").value, route.get("original_status").value), "XSD status: "+id)
	}
	check(len(xsdIDs) == 175, "duplicate selected XSD ID")
	check(newCount == 36, "new auxiliary count")
	check(reused == 139, "reused XSD count")
	axes := map[string]bool{"positive": true, "negative": true, "boundary": true, "missing_fact": true, "interaction": true}
	emptyAxes := 0
	usedTests := map[string]bool{}
	checkAxes := func(id string, row node) {
		rowAxes := row.get("axes")
		check(reflect.DeepEqual(keysSet(rowAxes.object()), axes), "missing original evidence axis: "+id)
		for _, axis := range sortedKeys(rowAxes.object()) {
			claims := rowAxes.get(axis)
			tests := claims.get("tests")
			archived := claims.optional("archived_tests", []any{})
			if len(tests.list()) == 0 && len(archived.list()) == 0 {
				emptyAxes++
			}
			for _, v := range archived.list() {
				ref := (node{v, id + "/archived_tests"}).text()
				_, ok := e.get("historical_tests").object()[ref]
				check(ok, "unknown archived test: "+ref)
			}
			for _, v := range tests.list() {
				ref := (node{v, id + "/tests"}).text()
				usedTests[ref] = true
				_, ok := e.get("tests").object()[ref]
				check(ok, "unknown test route: "+id+"/"+ref)
			}
		}
	}
	rows := e.get("rows")
	for _, id := range sortedKeys(rows.object()) {
		checkAxes(id, rows.get(id))
	}
	auxiliary := e.get("auxiliary_xsd_axes")
	check(len(auxiliary.list()) == 36, "auxiliary axis count")
	auxIDs := map[string]bool{}
	for i := range auxiliary.list() {
		row := auxiliary.at(i)
		row.fields(map[string]string{"id": "string", "source": "string"})
		id := row.get("id").text()
		check(!auxIDs[id], "duplicate auxiliary axis ID: "+id)
		auxIDs[id] = true
		source, pointer := routeSource(row, "source")
		source = path.Clean(path.Join("docs/validation/evidence", source))
		original := resolve(source, pointer)
		original.object()
		check(original.optional("instance_id", nil).value == id, "auxiliary source ID: "+id)
		checkAxes(id, row)
		fixture := original.get("fixture_axes")
		expected := map[string]bool{"positive": true, "negative": true, "boundary": true, "interaction": true}
		check(reflect.DeepEqual(keysSet(fixture.object()), expected), "auxiliary fixture route axis types: "+id)
		for _, axis := range sortedKeys(expected) {
			labels := row.get("axes").get(axis).get("case_labels")
			check(len(labels.list()) > 0 && reflect.DeepEqual(labels.value, fixture.optional(axis, nil).value), "auxiliary fixture route labels: "+id+"/"+axis)
		}
		missingTest := original.get("missing_fact_test").text()
		missing := row.get("axes").get("missing_fact")
		check(reflect.DeepEqual(missing.optional("case_labels", nil).value, []any{}) && reflect.DeepEqual(missing.get("tests").value, []any{"auxiliary_sounds_context_test.go::" + missingTest}), "auxiliary fixture route missing-fact test: "+id)
	}
	check(reflect.DeepEqual(auxIDs, newIDs), "auxiliary source ID denominator drift")
	check(reflect.DeepEqual(usedTests, keysSet(e.get("tests").object())), "unreferenced or missing current test records")
	r.hashes = map[string]string{}
	inspected := map[string][]byte{}
	for _, ref := range sortedKeys(e.get("tests").object()) {
		record := e.get("tests").get(ref)
		record.fields(map[string]string{"file": "string", "function": "string", "file_sha256": "string"})
		file, function := record.get("file").text(), record.get("function").text()
		check(ref == file+"::"+function, "test route key mismatch: "+ref)
		p := repositoryPath(root, file)
		info, err := os.Stat(p)
		if err != nil || !info.Mode().IsRegular() {
			check(false, "missing test file: "+file)
			continue
		}
		data, ok := inspected[file]
		if !ok {
			data = readBytes(p)
			inspected[file] = data
		}
		if !utf8.Valid(data) {
			fail("invalid inventory: %s: invalid UTF-8 test file", file)
		}
		r.hashes[file] = digest(canonicalLF(data))
		if r.hashes[file] != record.get("file_sha256").text() {
			stale = append(stale, file)
		}
		check(hasFunction(data, function), "missing test function: "+file+"::"+function)
	}
	archiveState := "not verified: supply --archive-dir to verify external archive bytes"
	if r.opts.archive != "" {
		artifacts := m.get("external_archive")
		for i := range artifacts.list() {
			record := artifacts.at(i)
			record.fields(map[string]string{"artifact": "string", "sha256": "string", "bytes": "integer"})
			name := record.get("artifact").text()
			p := repositoryPath(r.opts.archive, name)
			info, err := os.Stat(p)
			check(err == nil && info.Mode().IsRegular(), "missing archive: "+name)
			if err == nil && info.Mode().IsRegular() {
				data := readBytes(p)
				check(digest(data) == record.get("sha256").text() && int64(len(data)) == record.get("bytes").integer(), "changed archive: "+name)
			}
		}
		historical := e.get("historical_tests")
		for _, key := range sortedKeys(historical.object()) {
			record := historical.get(key)
			record.fields(map[string]string{"artifact": "string", "file_sha256": "string", "function": "string"})
			name := record.get("artifact").text()
			p := repositoryPath(r.opts.archive, name)
			info, err := os.Stat(p)
			check(err == nil && info.Mode().IsRegular(), "missing archived test: "+name)
			if err == nil && info.Mode().IsRegular() {
				data := readBytes(p)
				check(digest(data) == record.get("file_sha256").text(), "changed archived test bytes: "+name)
				if !utf8.Valid(data) {
					fail("invalid inventory: %s: invalid UTF-8 archived test", name)
				}
				check(hasFunction(data, record.get("function").text()), "missing archived test function: "+record.get("function").text())
			}
		}
		archiveState = "all supplied external artifacts checked against preserved hashes"
	} else {
		warnings = append(warnings, archiveState)
	}
	staleSet := map[string]bool{}
	for _, file := range stale {
		staleSet[file] = true
	}
	stale = sortedKeys(staleSet)
	if len(stale) > 0 && r.opts.strict {
		for _, file := range stale {
			failures = append(failures, "stale inspected test bytes: "+file)
		}
	} else if len(stale) > 0 {
		warnings = append(warnings, "Test bytes moved since inspection; refresh the row matrix before final acceptance.")
	}
	head, worktree := "unavailable", "unavailable"
	headBytes, err := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	if err == nil {
		var status []byte
		status, err = exec.Command("git", "-C", root, "status", "--porcelain").Output()
		if err == nil {
			head, worktree = strings.TrimSpace(string(headBytes)), strings.TrimSpace(string(status))
		}
	}
	if err != nil {
		warnings = append(warnings, "git observation unavailable: "+err.Error())
	}
	result := "PASS: inventory/source-route integrity only"
	if len(failures) > 0 {
		result = "FAIL"
	}
	return map[string]any{"result": result, "observed_current_head": head, "observed_worktree_status": worktree, "historical_observation": map[string]any{"head": m.get("snapshot").get("head_at_end").value, "state": m.get("snapshot").optional("state", "").value, "interpretation": "historical source/test observation; not current exact-head acceptance"}, "retained_counts": retained, "selected_counts": selectedCounts, "test_functions": len(e.get("tests").object()), "axes_without_runtime_route": emptyAxes, "external_archive": archiveState, "stale_test_files": stale, "warnings": warnings, "failures": failures, "repository_hash_convention": "SHA-256 and sizes after CRLF-to-LF only; JSON field values unmodified; external archives raw-byte exact", "semantic_resource_execution_acceptance": "NOT CLAIMED; independent final tree, seven local stages and nine exact-head CI jobs still required"}
}
func hasFunction(data []byte, function string) bool {
	return regexp.MustCompile(`(?m)^func\s+` + regexp.QuoteMeta(function) + `\s*\(`).Match(data)
}
