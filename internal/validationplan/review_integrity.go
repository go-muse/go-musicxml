package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// These expectations belong to the deliberately pinned registry revision. A pin
// migration must review them and the snapshot metadata together.
var pinnedInventoryCounts = map[string]int{
	"research": 2560, "xsd_instances": 2218, "prose_dispositions": 291,
	"open_questions": 28, "deferred_items": 15, "overlap_reviews": 342,
	"advisory_reviews": 59, "occurrence_mappings": 2939,
}

func (p *plan) inventoryCount(key string) int {
	counts, _ := p.docs["external-snapshot.json"]["inventory_counts"].(map[string]any)
	n, _ := counts[key].(float64)
	return int(n)
}
func (p *plan) validateCounts(snapshot object) error {
	counts, ok := snapshot["inventory_counts"].(map[string]any)
	if !ok {
		return fmt.Errorf("missing inventory count metadata")
	}
	for key, want := range pinnedInventoryCounts {
		if counts[key] != float64(want) {
			return fmt.Errorf("snapshot %s count disagrees with pinned revision", key)
		}
	}
	if snapshot["research_count"] != float64(len(p.research)) {
		return fmt.Errorf("snapshot research_count disagrees with actual inventory")
	}
	actual := map[string]int{"research": len(p.research), "xsd_instances": len(p.instances), "prose_dispositions": len(p.dispositions), "open_questions": len(p.questions), "deferred_items": len(p.deferred)}
	for key, n := range actual {
		if n != p.inventoryCount(key) {
			return fmt.Errorf("incomplete %s inventory: got %d", key, n)
		}
	}
	return nil
}
func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]string{}, a...)
	bb := append([]string{}, b...)
	sort.Strings(aa)
	sort.Strings(bb)
	for n := range aa {
		if aa[n] != bb[n] {
			return false
		}
	}
	return true
}

func (p *plan) validateOverlaps() error {
	byResearch := map[string]object{}
	seenIDs := map[string]bool{}
	texts := map[string][]string{}
	for filename, d := range p.docs {
		value, exists := d["overlap_reviews"]
		if !exists {
			continue
		}
		list, ok := value.([]any)
		if !ok {
			return fmt.Errorf("%s overlap_reviews is not an array", filename)
		}
		for _, item := range list {
			r, ok := item.(map[string]any)
			if !ok {
				return fmt.Errorf("%s overlap row is not an object", filename)
			}
			rid, id := str(r, "research_id"), str(r, "id")
			if _, exists := byResearch[rid]; exists {
				return fmt.Errorf("duplicate overlap research_id %s", rid)
			}
			if id == "" || seenIDs[id] {
				return fmt.Errorf("duplicate or empty overlap ID %s", id)
			}
			seenIDs[id] = true
			if str(p.research[rid], "origin") != "prose" {
				return fmt.Errorf("orphan prose overlap %s", rid)
			}
			if !has([]string{"full", "partial", "none"}, str(r, "kind")) {
				return fmt.Errorf("%s invalid overlap kind", rid)
			}
			if err := links(r, "xsd_research_ids", p.research, false); err != nil {
				return withID(rid, err)
			}
			if err := links(r, "additional_condition_check_ids", p.checks, true); err != nil {
				return withID(rid, err)
			}
			if !sameStrings(stringsAt(r, "additional_condition_check_ids"), stringsAt(p.research[rid], "check_ids")) {
				return fmt.Errorf("%s additional-condition checks belong to another record", rid)
			}
			if err := nonempty(r, "shared_condition", "additional_condition", "disposition", "justification"); err != nil {
				return withID(rid, err)
			}
			text := str(r, "additional_condition")
			switch str(r, "additional_condition_source") {
			case "disposition_summary":
				own, ok := p.dispositions[rid]
				if !ok {
					return fmt.Errorf("%s has no disposition summary", rid)
				}
				expected := str(own, "produces")
				if expected == "" {
					expected = str(own, "rationale")
				}
				if text != expected {
					return fmt.Errorf("%s additional condition differs from its own disposition", rid)
				}
			case "check_clause":
				matched := false
				for _, cid := range stringsAt(r, "additional_condition_check_ids") {
					clauses, _ := p.checks[cid]["research_clauses"].(map[string]any)
					if clause, ok := clauses[rid].(string); ok && clause == text {
						matched = true
					}
				}
				if !matched {
					return fmt.Errorf("%s additional condition differs from its own check clause", rid)
				}
			case "reviewed_overlap":
				if _, isContext := p.dispositions[rid]; isContext {
					return fmt.Errorf("%s context condition must retain its own-record binding", rid)
				}
			default:
				return fmt.Errorf("%s missing additional-condition source binding", rid)
			}
			byResearch[rid] = r
			texts[text] = append(texts[text], rid)
		}
	}
	if len(byResearch) != p.inventoryCount("overlap_reviews") {
		return fmt.Errorf("incomplete prose overlap inventory")
	}
	// Shared text is legitimate only for a reviewed, closed set of clauses. This
	// heuristic supplements the own-record bindings above; it is no semantic proof.
	allowed := map[string][]string{}
	for _, a := range rows(p.docs["overlap-policy.json"], "shared_additional_conditions") {
		hash := str(a, "condition_sha256")
		if _, exists := allowed[hash]; exists {
			return fmt.Errorf("duplicate shared-clause policy %s", hash)
		}
		if err := links(a, "research_ids", p.research, true); err != nil {
			return err
		}
		if len(stringsAt(a, "research_ids")) < 2 || str(a, "justification") == "" {
			return fmt.Errorf("invalid shared-clause policy")
		}
		allowed[hash] = stringsAt(a, "research_ids")
	}
	used := map[string]bool{}
	for text, ids := range texts {
		if len(ids) > 1 {
			hash := digest([]byte(text))
			if !sameStrings(ids, allowed[hash]) {
				return fmt.Errorf("unapproved duplicate additional condition: %v", ids)
			}
			used[hash] = true
		}
	}
	for hash := range allowed {
		if !used[hash] {
			return fmt.Errorf("stale shared-clause policy %s", hash)
		}
	}
	return nil
}

func headingAnchor(text string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			b.WriteRune(r)
		} else if r == ' ' {
			b.WriteByte('-')
		}
	}
	return b.String()
}
func (p *plan) validateDeferredInventory(snapshot object) error {
	source, ok := snapshot["deferred_inventory"].(map[string]any)
	if !ok {
		return fmt.Errorf("missing published deferred inventory")
	}
	path := str(source, "path")
	if path != "../deferred-inventory.md" {
		return fmt.Errorf("unexpected deferred inventory path %q", path)
	}
	b, err := os.ReadFile(filepath.Join(p.planDir, filepath.FromSlash(path)))
	if err != nil {
		return fmt.Errorf("published deferred inventory: %w", err)
	}
	if digest(b) != str(source, "sha256") {
		return fmt.Errorf("published deferred inventory hash mismatch")
	}
	keys := []string{}
	seen := map[string]bool{}
	sections := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "### DEF") {
			key := strings.TrimPrefix(line, "### ")
			if seen[key] {
				return fmt.Errorf("duplicate deferred source heading %s", key)
			}
			seen[key] = true
			keys = append(keys, key)
		}
		if strings.HasPrefix(line, "## ") {
			heading := strings.TrimPrefix(line, "## ")
			first, _, _ := strings.Cut(heading, " ")
			n, err := strconv.Atoi(first)
			if err == nil {
				sections[fmt.Sprintf("D%02d", n)] = headingAnchor(heading)
			}
		}
	}
	if !sameStrings(keys, stringsAt(snapshot, "deferred_source_bullets")) {
		return fmt.Errorf("published deferred keys disagree with snapshot")
	}
	if len(sections) != p.inventoryCount("deferred_items") {
		return fmt.Errorf("published deferred sections incomplete")
	}
	for id, d := range p.deferred {
		if str(d, "source_section") != path+"#"+sections[id] {
			return fmt.Errorf("%s source_section does not resolve to its published section", id)
		}
	}
	for id, r := range p.requirements {
		if str(r, "source_inventory") != path {
			return fmt.Errorf("%s lacks published source inventory", id)
		}
		for _, key := range stringsAt(r, "source_bullets") {
			if !seen[key] {
				return fmt.Errorf("%s source key %s has no published heading", id, key)
			}
		}
	}
	return nil
}

func externalRecord(item any) (object, error) {
	ext, ok := item.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("external catalog row is not an object")
	}
	if str(ext, "registry_origin") == "XSD" {
		if _, ok := ext["record"].(map[string]any); !ok {
			return nil, fmt.Errorf("external XSD record %s lacks record object", str(ext, "id"))
		}
	}
	return ext, nil
}

// Specificity applies to every case in an axis, not just the complete array:
// mixing one shared recipe with distinct cases does not make that axis specific.
func (p *plan) validateSharedRecipes() error {
	owners := map[string]map[string]bool{}
	axes := []string{"positive", "negative", "boundary", "missing_fact", "interaction"}
	for id, t := range p.tests {
		cases, _ := t["cases"].(map[string]any)
		for _, axis := range axes {
			for _, text := range stringsAt(cases, axis) {
				key := axis + "\x00" + text
				if owners[key] == nil {
					owners[key] = map[string]bool{}
				}
				owners[key][id] = true
			}
		}
	}
	for id, t := range p.tests {
		cases, _ := t["cases"].(map[string]any)
		status, _ := t["axis_status"].(map[string]any)
		for _, axis := range axes {
			if str(status, axis) != "record_specific" {
				continue
			}
			for _, text := range stringsAt(cases, axis) {
				if len(owners[axis+"\x00"+text]) > 1 {
					return fmt.Errorf("%s %s has a shared case recipe labeled record_specific", id, axis)
				}
			}
		}
	}
	stage := p.stages["STAGE-context"]
	if str(stage, "dependency_interpretation") != "batch_acceptance" || str(stage, "scope_note") == "" {
		return fmt.Errorf("STAGE-context must distinguish batch acceptance from per-clause evidence")
	}
	return nil
}
