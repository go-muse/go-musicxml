#!/usr/bin/env python3
"""Verify additive batch-8 record/test routes without executing or accepting code.

Use --archive-dir for the separately preserved complete evidence archive. Absence
of that directory is reported explicitly; repository originals remain verifiable.
No files are changed unless --output is supplied, which writes the result only.
"""
import argparse, hashlib, json, re, subprocess, sys
from pathlib import Path


def digest(data):
    return hashlib.sha256(data).hexdigest()


def canonical_lf(data):
    """Normalize checkout CRLF only; never strip lone CR or other content."""
    return data.replace(b"\r\n", b"\n")


def canonical(value):
    return digest(json.dumps(value, ensure_ascii=False, sort_keys=True,
                             separators=(",", ":")).encode())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path("."))
    parser.add_argument("--manifest", type=Path,
                        default=Path("docs/validation/package-assessment-ownership.json"))
    parser.add_argument("--evidence", type=Path,
                        default=Path("docs/validation/evidence/package-row-evidence.json"))
    parser.add_argument("--archive-dir", type=Path)
    parser.add_argument("--strict-evidence", action="store_true",
                        help="fail if inspected test bytes changed; still not acceptance")
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    root = args.root.resolve()
    manifest_path = args.manifest if args.manifest.is_absolute() else root / args.manifest
    evidence_path = args.evidence if args.evidence.is_absolute() else root / args.evidence
    m = json.loads(manifest_path.read_text(encoding="utf-8"))
    evidence = json.loads(evidence_path.read_text(encoding="utf-8"))
    failures, stale, warnings = [], [], []
    loaded = {}

    def check(ok, message):
        if not ok:
            failures.append(message)

    def load(path):
        if path not in loaded:
            loaded[path] = json.loads((root / path).read_text(encoding="utf-8"))
        return loaded[path]

    def resolve(path, pointer):
        value = load(path)
        for key in pointer.removeprefix("#").strip("/").split("/"):
            key = key.replace("~1", "/").replace("~0", "~")
            value = value[int(key)] if isinstance(value, list) else value[key]
        return value

    check(m.get("hashing", {}).get("repository_text") == "sha256-crlf-to-lf-v1",
          "unsupported or unspecified repository hash convention")
    for path, record in m["source_files"].items():
        data = canonical_lf((root / path).read_bytes())
        check(digest(data) == record["sha256"], "changed original source: " + path)
        check(len(data) == record["bytes"], "changed original source size: " + path)
    retained_counts = {}
    for name, collection in m["preserved_collections"].items():
        actual = resolve(collection["source"], collection["pointer"])
        rows = collection["records"]
        retained_counts[name] = len(rows)
        check(len(rows) == collection["count"] == len(actual), "record count: " + name)
        check(len({r["id"] for r in rows}) == len(rows), "duplicate ID: " + name)
        check({r["index"] for r in rows} == set(range(len(actual))), "lost index: " + name)
        for route in rows:
            if route["index"] >= len(actual):
                continue
            original = actual[route["index"]]
            check(original[collection["id_field"]] == route["id"], "ID route: " + route["id"])
            check(canonical(original) == route["sha256"], "original record: " + route["id"])
            for key, value in route.get("original_status_fields", {}).items():
                check(original.get(key) == value, "status promotion: " + route["id"] + "/" + key)
    wanted = {"requirements": 95, "context_checks": 307, "context_dispositions": 291,
              "constraint_checks": 51, "issue_gates": 28, "retained_splits": 20,
              "retained_source_contracts": 42, "retained_candidate_dispositions": 29,
              "source_subclauses": 2, "retained_boundaries": 2}
    check(retained_counts == wanted, "preserved denominator drift")
    selected_counts = {k: len(v) for k, v in m["selected"].items()}
    check(selected_counts == {"requirements": 44, "context_checks": 24,
                              "constraint_checks": 10, "issue_gates": 5,
                              "xsd_instances": 175}, "selected denominator drift")
    selected_ids = set()
    for category in ["requirements", "context_checks", "constraint_checks"]:
        for route in m["selected"][category]:
            check(route["id"] not in selected_ids, "duplicate selected ID: " + route["id"])
            selected_ids.add(route["id"])
            path, pointer = route["original_source"].split("#", 1)
            original = resolve(path, pointer)
            check(original["id"] == route["id"], "selected source ID: " + route["id"])
            check(canonical(original) == route["original_record_sha256"],
                  "selected original record: " + route["id"])
            check(bool(route["owner"]) and bool(route["bounded_contribution"]),
                  "missing selected owner/contribution: " + route["id"])
    check(selected_ids == set(evidence["rows"]), "missing/extra row evidence")
    for route in m["selected"]["issue_gates"]:
        path, pointer = route["source"].split("#", 1)
        original = resolve(path, pointer)
        check(original["id"] == route["id"], "issue ID: " + route["id"])
        check(original["disposition"] == route["original_disposition"] == "decision_required",
              "issue disposition promotion: " + route["id"])
        check(original["hard_error_basis"] is False and route["original_hard_error_basis"] is False,
              "issue hard-error promotion: " + route["id"])
    xsd = m["selected"]["xsd_instances"]
    check(len({r["id"] for r in xsd}) == 175, "duplicate selected XSD ID")
    check(sum(r["selection"] == "new_auxiliary" for r in xsd) == 36, "new auxiliary count")
    check(sum(r["selection"] == "reused_transitive" for r in xsd) == 139, "reused XSD count")
    for route in xsd:
        path, pointer = route["source"].split("#", 1)
        original = resolve(path, pointer)
        check(original["id"] == route["id"] and canonical(original) == route["record_sha256"],
              "selected XSD record: " + route["id"])
        check(original["status"] == route["original_status"], "XSD status: " + route["id"])
    axes = {"positive", "negative", "boundary", "missing_fact", "interaction"}
    empty_axes = 0
    for id, row in evidence["rows"].items():
        check(set(row["axes"]) == axes, "missing original evidence axis: " + id)
        for axis, claims in row["axes"].items():
            empty_axes += not bool(claims["tests"] or claims.get("archived_tests"))
            for archived_ref in claims.get("archived_tests", []):
                check(archived_ref in evidence.get("historical_tests", {}), "unknown archived test: " + archived_ref)
            for ref in claims["tests"]:
                check(ref in evidence["tests"], "unknown test route: " + id + "/" + ref)
    check(len(evidence["auxiliary_xsd_axes"]) == 36, "auxiliary axis count")
    for record in evidence["tests"].values():
        path = root / record["file"]
        if not path.is_file():
            failures.append("missing test file: " + record["file"])
            continue
        data = path.read_bytes()
        if digest(canonical_lf(data)) != record["file_sha256"]:
            stale.append(record["file"])
        check(re.search(r"^func\s+" + re.escape(record["function"]) + r"\s*\(",
                        data.decode(), re.M) is not None,
              "missing test function: " + record["file"] + "::" + record["function"])
    archive_state = "not verified: supply --archive-dir to verify external archive bytes"
    if args.archive_dir:
        for artifact in m["external_archive"]:
            path = args.archive_dir / artifact["artifact"]
            check(path.is_file(), "missing archive: " + artifact["artifact"])
            if path.is_file():
                data = path.read_bytes()
                check(digest(data) == artifact["sha256"] and len(data) == artifact["bytes"],
                      "changed archive: " + artifact["artifact"])
        for record in evidence.get("historical_tests", {}).values():
            path = args.archive_dir / record["artifact"]
            if path.is_file():
                data = path.read_bytes()
                check(digest(data) == record["file_sha256"], "changed archived test bytes: " + record["artifact"])
                check(re.search(r"^func\s+" + re.escape(record["function"]) + r"\s*\(", data.decode(), re.M) is not None,
                      "missing archived test function: " + record["function"])
        archive_state = "all supplied external artifacts checked against preserved hashes"
    else:
        warnings.append(archive_state)
    stale = sorted(set(stale))
    if stale and args.strict_evidence:
        failures.extend("stale inspected test bytes: " + file for file in stale)
    elif stale:
        warnings.append("Test bytes moved since inspection; refresh the row matrix before final acceptance.")
    try:
        head = subprocess.check_output(["git", "-C", str(root), "rev-parse", "HEAD"], text=True, stderr=subprocess.DEVNULL).strip()
        worktree = subprocess.check_output(["git", "-C", str(root), "status", "--porcelain"], text=True, stderr=subprocess.DEVNULL).strip()
    except subprocess.CalledProcessError:
        head, worktree = "unavailable", "unavailable"
    if head != m["snapshot"]["head_at_end"] or worktree != m["snapshot"]["status_at_end"]:
        warnings.append("Integration differs from the observation. This is expected during active work, but is not exact-head acceptance.")
    elif worktree:
        warnings.append("Observed integration remains dirty as recorded; inventory integrity does not establish final acceptance.")
    result = {"result": "FAIL" if failures else "PASS: inventory/source-route integrity only",
              "observed_current_head": head, "observed_worktree_status": worktree,
              "retained_counts": retained_counts, "selected_counts": selected_counts,
              "test_functions": len(evidence["tests"]), "axes_without_runtime_route": empty_axes,
              "external_archive": archive_state, "stale_test_files": stale,
              "warnings": warnings, "failures": failures,
              "repository_hash_convention": "SHA-256 and sizes after CRLF-to-LF only; JSON field values unmodified; external archives raw-byte exact",
              "semantic_resource_execution_acceptance": "NOT CLAIMED; independent final tree, seven local stages and nine exact-head CI jobs still required"}
    text = json.dumps(result, ensure_ascii=False, indent=2) + "\n"
    if args.output:
        args.output.write_text(text, encoding="utf-8")
    print(text, end="")
    return bool(failures)


if __name__ == "__main__":
    sys.exit(main())
