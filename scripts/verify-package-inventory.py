#!/usr/bin/env python3
"""Verify additive batch-8 record/test routes without executing or accepting code.

Use --archive-dir for the separately preserved complete evidence archive. Absence
of that directory is reported explicitly; repository originals remain verifiable.
No files are changed unless --output is supplied, which writes the result only.
"""
import argparse
import posixpath
import re
import subprocess
import sys
sys.dont_write_bytecode = True
from pathlib import Path

# Load trusted sibling helpers even with Python's -I / -P safe-path modes.
sys.path.insert(0, str(Path(__file__).resolve().parent))
from inventory_common import (InventoryError, canonical, canonical_lf, digest,
                              emit_result, failure_result, field, input_paths, inputs_known,
                              read_bytes, read_json, repository_path, require, resolve_root)


def argument_parser(include_strict=True):
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path("."))
    parser.add_argument("--manifest", type=Path,
                        default=Path("docs/validation/package-assessment-ownership.json"))
    parser.add_argument("--evidence", type=Path,
                        default=Path("docs/validation/evidence/package-row-evidence.json"))
    parser.add_argument("--archive-dir", type=Path)
    if include_strict:
        parser.add_argument("--strict-evidence", action="store_true",
                            help="fail if inspected test bytes changed; still not acceptance")
    parser.add_argument("--output", type=Path)
    return parser


def verify(args):
    """Complete read-only preflight reused by the narrow test-hash refresher."""
    args.inputs_known = False
    args.protected_inputs = []
    manifest_path, evidence_path = args.manifest, args.evidence
    try:
        root = resolve_root(args.root)
        manifest_path = args.manifest if args.manifest.is_absolute() else root / args.manifest
        evidence_path = args.evidence if args.evidence.is_absolute() else root / args.evidence
        args.protected_inputs = input_paths(root, {}, {}, manifest_path, evidence_path)
        m = read_json(manifest_path)
        evidence = read_json(evidence_path)
        args.protected_inputs = input_paths(root, m, evidence, manifest_path, evidence_path, args.archive_dir)
        args.inputs_known = inputs_known(m, evidence)
        require(m, dict, str(manifest_path))
        require(evidence, dict, str(evidence_path))
        args.manifest_value, args.evidence_value = m, evidence
        args.evidence_path = evidence_path
        return verify_loaded(args, root, m, evidence)
    except InventoryError as exc:
        return failure_result(exc)
    except (KeyError, IndexError, TypeError, ValueError, UnicodeError, OSError, AttributeError) as exc:
        return failure_result(InventoryError(f'invalid inventory: {manifest_path} / {evidence_path}: {type(exc).__name__}: {exc}'))


def verify_loaded(args, root, m, evidence):
    failures, stale, warnings = [], [], []
    loaded = {}

    def check(ok, message):
        if not ok:
            failures.append(message)

    def load(path):
        if path not in loaded:
            loaded[path] = read_json(repository_path(root, path))
        return loaded[path]

    def resolve(path, pointer):
        location = f'{path}#{pointer.removeprefix("#")}' if isinstance(pointer, str) else f'{path}#{pointer!r}'
        try:
            require(path, str, 'source route path')
            require(pointer, str, location)
            if path not in m['source_files']:
                raise InventoryError('source route is not declared in source_files: ' + path)
            value = load(path)
            pointer = pointer.removeprefix('#')
            if pointer and not pointer.startswith('/'):
                raise ValueError('pointer must be empty or start with /')
            for key in pointer[1:].split('/') if pointer else []:
                if re.search(r'~(?![01])', key):
                    raise ValueError('invalid JSON pointer escape')
                key = key.replace('~1', '/').replace('~0', '~')
                if isinstance(value, list):
                    if not re.fullmatch(r'0|[1-9][0-9]*', key):
                        raise ValueError('array index must be a nonnegative canonical integer')
                    value = value[int(key)]
                elif isinstance(value, dict):
                    value = value[key]
                else:
                    raise TypeError('cannot traverse a scalar')
            return value
        except (KeyError, IndexError, TypeError, ValueError, InventoryError) as exc:
            raise InventoryError(f'unresolvable pointer: {location}: {exc}') from exc

    def route_source(route, key):
        source = field(route, key, str, 'source route')
        if '#' not in source:
            raise InventoryError(f'unresolvable pointer: {source}: missing # separator')
        return source.split('#', 1)

    def fields(record, location, **schema):
        for key, kind in schema.items():
            field(record, key, kind, location)

    # Validate input shapes before interpreting their values or considering writes.
    fields(m, 'manifest', hashing=dict, source_files=dict, preserved_collections=dict,
           selected=dict, counts=dict, snapshot=dict, external_archive=list)
    fields(evidence, 'evidence', rows=dict, tests=dict, historical_tests=dict,
           auxiliary_xsd_axes=list, hashing=dict)
    corrections = require(m.get('reviewed_source_corrections', []), list,
                          'reviewed_source_corrections')
    for correction in corrections:
        fields(correction, 'reviewed_source_corrections record', reconciliation=str)
    check(field(evidence['hashing'], 'test_file_sha256', str, 'evidence/hashing').startswith('sha256-crlf-to-lf-v1;'),
          'unsupported or unspecified test hash convention')
    check(len(m['source_files']) == 17, 'source file denominator drift')
    check(m.get("hashing", {}).get("repository_text") == "sha256-crlf-to-lf-v1",
          "unsupported or unspecified repository hash convention")
    for path, record in m["source_files"].items():
        fields(record, 'source_files/' + path, sha256=str, bytes=int)
        data = canonical_lf(read_bytes(repository_path(root, path)))
        load(path)  # Parse every source, not only the ones reached by selected routes.
        check(digest(data) == record["sha256"], "changed original source: " + path)
        check(len(data) == record["bytes"], "changed original source size: " + path)
    retained_counts = {}
    for name, collection in m["preserved_collections"].items():
        fields(collection, 'preserved_collections/' + name,
               source=str, pointer=str, records=list, count=int, id_field=str)
        actual = require(resolve(collection['source'], collection['pointer']), list,
                         collection['source'] + collection['pointer'])
        for route in collection['records']:
            fields(route, 'preserved_collections/' + name, id=str, index=int, sha256=str)
            check(route['index'] >= 0, 'negative record index: ' + route['id'])
        rows = collection["records"]
        retained_counts[name] = len(rows)
        check(len(rows) == collection["count"] == len(actual), "record count: " + name)
        check(len({r["id"] for r in rows}) == len(rows), "duplicate ID: " + name)
        check({r["index"] for r in rows} == set(range(len(actual))), "lost index: " + name)
        for route in rows:
            if not 0 <= route["index"] < len(actual):
                continue
            original = require(actual[route["index"]], dict, "original record: " + route["id"])
            check(original[collection["id_field"]] == route["id"], "ID route: " + route["id"])
            check(canonical(original) == route["sha256"], "original record: " + route["id"])
            statuses = require(route.get("original_status_fields", {}), dict, "original_status_fields/" + route["id"])
            for key, value in statuses.items():
                check(original.get(key) == value, "status promotion: " + route["id"] + "/" + key)
    wanted = {"requirements": 95, "context_checks": 307, "context_dispositions": 291,
              "constraint_checks": 51, "issue_gates": 28, "retained_splits": 20,
              "retained_source_contracts": 42, "retained_candidate_dispositions": 29,
              "source_subclauses": 2, "retained_boundaries": 2}
    check(retained_counts == wanted, "preserved denominator drift")
    check(all(m['counts'].get(name) == count for name, count in wanted.items()),
          'declared original denominator drift')
    selected_denominators = {'selected_engineering': 44, 'selected_context': 24,
                             'selected_constraints': 10, 'selected_issue_gates': 5,
                             'selected_xsd_total': 175, 'new_auxiliary_xsd': 36,
                             'new_container_xsd': 11, 'new_sounds_xsd': 25,
                             'reused_transitive_xsd': 139, 'selected_new_translations': 0}
    check(all(m['counts'].get(name) == count for name, count in selected_denominators.items()),
          'declared selected denominator drift')
    for category, rows in m['selected'].items():
        require(rows, list, 'selected/' + category)
        for route in rows:
            fields(route, 'selected/' + category, id=str, owner=str)
            check(bool(route['owner'].strip()), 'missing selected owner: ' + route['id'])
    selected_counts = {k: len(v) for k, v in m["selected"].items()}
    check(selected_counts == {"requirements": 44, "context_checks": 24,
                              "constraint_checks": 10, "issue_gates": 5,
                              "xsd_instances": 175}, "selected denominator drift")
    selected_ids = set()
    for category in ["requirements", "context_checks", "constraint_checks"]:
        for route in m["selected"][category]:
            check(route["id"] not in selected_ids, "duplicate selected ID: " + route["id"])
            selected_ids.add(route["id"])
            path, pointer = route_source(route, "original_source")
            original = require(resolve(path, pointer), dict, route["original_source"])
            check(original["id"] == route["id"], "selected source ID: " + route["id"])
            check(canonical(original) == route["original_record_sha256"],
                  "selected original record: " + route["id"])
            check(bool(route["owner"]) and bool(route["bounded_contribution"]),
                  "missing selected owner/contribution: " + route["id"])
    check(selected_ids == set(evidence["rows"]), "missing/extra row evidence")
    check(len({r["id"] for r in m["selected"]["issue_gates"]}) == 5, "duplicate selected issue ID")
    for route in m["selected"]["issue_gates"]:
        path, pointer = route_source(route, "source")
        original = require(resolve(path, pointer), dict, route["source"])
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
        path, pointer = route_source(route, "source")
        original = require(resolve(path, pointer), dict, route["source"])
        check(original["id"] == route["id"] and canonical(original) == route["record_sha256"],
              "selected XSD record: " + route["id"])
        check(original["status"] == route["original_status"], "XSD status: " + route["id"])
    axes = {"positive", "negative", "boundary", "missing_fact", "interaction"}
    empty_axes = 0
    used_test_routes = set()
    def check_axes(id, row):
        nonlocal empty_axes
        fields(row, 'evidence row/' + id, axes=dict)
        check(set(row['axes']) == axes, 'missing original evidence axis: ' + id)
        for axis, claims in row['axes'].items():
            fields(claims, id + '/' + axis, tests=list)
            require(claims.get('archived_tests', []), list, id + '/' + axis + '/archived_tests')
            empty_axes += not bool(claims['tests'] or claims.get('archived_tests'))
            for archived_ref in claims.get('archived_tests', []):
                require(archived_ref, str, id + '/archived_tests')
                check(archived_ref in evidence['historical_tests'], 'unknown archived test: ' + archived_ref)
            for ref in claims['tests']:
                require(ref, str, id + '/tests')
                used_test_routes.add(ref)
                check(ref in evidence['tests'], 'unknown test route: ' + id + '/' + ref)

    for id, row in evidence['rows'].items():
        check_axes(id, row)
    check(len(evidence['auxiliary_xsd_axes']) == 36, 'auxiliary axis count')
    auxiliary_ids = set()
    for row in evidence['auxiliary_xsd_axes']:
        fields(row, 'auxiliary_xsd_axes', id=str, source=str)
        check(row['id'] not in auxiliary_ids, 'duplicate auxiliary axis ID: ' + row['id'])
        auxiliary_ids.add(row['id'])
        # These pointers are relative to the evidence document's committed location.
        source, pointer = route_source(row, 'source')
        source = posixpath.normpath(posixpath.join('docs/validation/evidence', source))
        original = require(resolve(source, pointer), dict, row['source'])
        check(original.get('instance_id') == row['id'], 'auxiliary source ID: ' + row['id'])
        check_axes(row['id'], row)
        fixture_axes = field(original, 'fixture_axes', dict, row['source'])
        check(set(fixture_axes) == axes - {'missing_fact'},
              'auxiliary fixture route axis types: ' + row['id'])
        for axis in axes - {'missing_fact'}:
            labels = field(row['axes'][axis], 'case_labels', list, row['id'] + '/' + axis)
            check(bool(labels) and labels == fixture_axes.get(axis),
                  'auxiliary fixture route labels: ' + row['id'] + '/' + axis)
        missing_test = field(original, 'missing_fact_test', str, row['source'])
        missing = row['axes']['missing_fact']
        check(missing.get('case_labels') == [] and
              missing['tests'] == ['auxiliary_sounds_context_test.go::' + missing_test],
              'auxiliary fixture route missing-fact test: ' + row['id'])
    check(auxiliary_ids == {r['id'] for r in xsd if r['selection'] == 'new_auxiliary'},
          'auxiliary source ID denominator drift')
    check(used_test_routes == set(evidence['tests']), 'unreferenced or missing current test records')
    args.current_test_hashes = {}
    inspected = {}
    for ref, record in evidence['tests'].items():
        fields(record, 'tests/' + ref, file=str, function=str, file_sha256=str)
        check(ref == record['file'] + '::' + record['function'], 'test route key mismatch: ' + ref)
        path = repository_path(root, record['file'])
        if not path.is_file():
            failures.append('missing test file: ' + record['file'])
            continue
        if record['file'] not in inspected:
            inspected[record['file']] = read_bytes(path)
        data = inspected[record['file']]
        args.current_test_hashes[record['file']] = digest(canonical_lf(data))
        if args.current_test_hashes[record['file']] != record['file_sha256']:
            stale.append(record['file'])
        check(re.search(r'^func\s+' + re.escape(record['function']) + r'\s*\(',
                        data.decode('utf-8'), re.M) is not None,
              'missing test function: ' + record['file'] + '::' + record['function'])
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
            check(path.is_file(), "missing archived test: " + record["artifact"])
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
    except (subprocess.CalledProcessError, OSError) as exc:
        warnings.append(f"git observation unavailable: {exc}")
        head, worktree = "unavailable", "unavailable"
    # The preserved snapshot describes an unpublished historical observation.
    # Comparing it to every later HEAD produces a permanent, uninformative warning;
    # report both identities without implying either one is execution acceptance.
    historical = {"head": m["snapshot"]["head_at_end"],
                  "state": m["snapshot"].get("state", ""),
                  "interpretation": "historical source/test observation; not current exact-head acceptance"}
    result = {"result": "FAIL" if failures else "PASS: inventory/source-route integrity only",
              "observed_current_head": head, "observed_worktree_status": worktree,
              "historical_observation": historical,
              "retained_counts": retained_counts, "selected_counts": selected_counts,
              "test_functions": len(evidence["tests"]), "axes_without_runtime_route": empty_axes,
              "external_archive": archive_state, "stale_test_files": stale,
              "warnings": warnings, "failures": failures,
              "repository_hash_convention": "SHA-256 and sizes after CRLF-to-LF only; JSON field values unmodified; external archives raw-byte exact",
              "semantic_resource_execution_acceptance": "NOT CLAIMED; independent final tree, seven local stages and nine exact-head CI jobs still required"}
    return result


def main():
    args = argument_parser().parse_args()
    result = verify(args)
    return emit_result(result, args.output, args.protected_inputs, args.inputs_known)


if __name__ == '__main__':
    sys.exit(main())
