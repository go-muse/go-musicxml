#!/usr/bin/env python3
"""Preview current test-file hash updates; --write commits only those hash fields.

Original sources/records, ownership/statuses, denominators, routes and historical
execution evidence are never regenerated. Source-record reconciliation requires
separate review of explicit record IDs; there is deliberately no source refresh.
"""
import importlib.util
from pathlib import Path
import sys
sys.dont_write_bytecode = True

from inventory_common import (InventoryError, atomic_write_json,
                              emit_result, reject_alias)

spec = importlib.util.spec_from_file_location('package_inventory_verifier',
                                             Path(__file__).with_name('verify-package-inventory.py'))
verifier = importlib.util.module_from_spec(spec)
spec.loader.exec_module(verifier)


def main():
    parser = verifier.argument_parser(include_strict=False)
    parser.description = __doc__
    parser.add_argument('--write', action='store_true',
                        help='atomically update only current tests[].file_sha256 after complete preflight')
    args = parser.parse_args()
    # Stale test hashes are the only failure this operation is allowed to repair.
    args.strict_evidence = False
    result = verifier.verify(args)
    result['mode'] = 'write' if args.write else 'dry-run'
    result['changed_test_files'] = []
    result['changed_test_routes'] = []
    result['written'] = False
    try:
        # Validate result-output aliases before the evidence write as well.
        if args.output is not None:
            reject_alias(args.output, args.protected_inputs)
        if not result['failures']:
            evidence = args.evidence_value
            changes = {}
            for ref, record in evidence['tests'].items():
                current = args.current_test_hashes[record['file']]
                if current != record['file_sha256']:
                    changes[record['file']] = current
                    result['changed_test_routes'].append(ref)
            result['changed_test_files'] = sorted(changes)
            result['changed_test_routes'].sort()
            if args.write and changes:
                reject_alias(args.evidence_path,
                             [p for p in args.protected_inputs if p != args.evidence_path])
                for record in evidence['tests'].values():
                    if record['file'] in changes:
                        record['file_sha256'] = changes[record['file']]
                atomic_write_json(args.evidence_path, evidence)
                result['written'] = True
            result['result'] = ('PASS: current test hashes written; no execution acceptance'
                                if result['written'] else 'PASS: current test hash preview; no execution acceptance')
    except InventoryError as exc:
        result['result'] = 'FAIL'
        result['failures'].append(str(exc))
    return emit_result(result, args.output, args.protected_inputs, args.inputs_known)


if __name__ == '__main__':
    sys.exit(main())
