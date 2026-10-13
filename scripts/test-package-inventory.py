#!/usr/bin/env python3
"""Exercise inventory integrity under LF/CRLF/mixed checkouts and real mutations.

Only temporary copies are changed. This does not execute production Go code or
establish semantic/resource acceptance.
"""
import argparse
import json
import subprocess
import sys
sys.dont_write_bytecode = True
import tempfile
from pathlib import Path

from inventory_common import (InventoryError, emit_result, failure_result, input_paths, inputs_known,
                              read_json, reject_alias, require)


def parse_verifier_result(run, name, expected):
    """Keep the real subprocess diagnostic when JSON parsing itself fails."""
    detail = f'{name}: expected exit {expected}, exit {run.returncode}; stderr={run.stderr!r}'
    try:
        result = json.loads(run.stdout)
    except (ValueError, TypeError) as exc:
        raise InventoryError(f'{detail}; invalid verifier JSON: {exc}; stdout={run.stdout!r}') from exc
    if not isinstance(result, dict) or not isinstance(result.get('failures'), list):
        raise InventoryError(f'{detail}; invalid verifier result shape: {result!r}')
    if run.returncode != expected:
        raise InventoryError(f'{detail}; result={result!r}')
    return result


def argument_parser():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path("."))
    parser.add_argument("--manifest", type=Path,
                        default=Path("docs/validation/package-assessment-ownership.json"))
    parser.add_argument("--evidence", type=Path,
                        default=Path("docs/validation/evidence/package-row-evidence.json"))
    parser.add_argument("--output", type=Path)
    return parser


def run_controls(args):
    root = args.root.resolve()
    mp = args.manifest if args.manifest.is_absolute() else root / args.manifest
    ep = args.evidence if args.evidence.is_absolute() else root / args.evidence
    args.protected_inputs = input_paths(root, {}, {}, mp, ep)
    manifest = read_json(mp)
    evidence = read_json(ep)
    args.protected_inputs = input_paths(root, manifest, evidence, mp, ep)
    args.inputs_known = inputs_known(manifest, evidence)
    require(manifest, dict, str(mp))
    require(evidence, dict, str(ep))
    if args.output is not None:
        reject_alias(args.output, args.protected_inputs)
    paths = set(manifest["source_files"])
    paths.update(record["file"] for record in evidence["tests"].values())
    originals = {path: (root / path).read_bytes().replace(b"\r\n", b"\n") for path in paths}
    verifier = Path(__file__).with_name("verify-package-inventory.py")
    results = []
    cases = [("LF", 0), ("mixed", 0), ("CRLF", 0),
             ("mutated-source-whitespace", 1), ("mutated-test-content", 1),
             ("lone-CR-not-normalized", 1), ("promoted-original-status", 1),
             ("invented-test-route", 1)]
    with tempfile.TemporaryDirectory(prefix="package-inventory-checkout-") as tmp:
        checkout = Path(tmp)
        for name, expected in cases:
            for path, data in originals.items():
                if name == "CRLF":
                    data = data.replace(b"\n", b"\r\n")
                elif name == "mixed":
                    data = data.replace(b"\n", b"\r\n", 1)
                dest = checkout / path
                dest.parent.mkdir(parents=True, exist_ok=True)
                dest.write_bytes(data)
            m = json.loads(mp.read_text(encoding="utf-8"))
            e = json.loads(ep.read_text(encoding="utf-8"))
            source_path = "docs/validation/plan/context-dispositions.json"
            if name == "mutated-source-whitespace":
                dest = checkout / source_path
                dest.write_bytes(dest.read_bytes() + b" ")
            if name == "mutated-test-content":
                dest = checkout / "link_intent_test.go"
                dest.write_bytes(dest.read_bytes() + b"// Actual content mutation.\n")
            if name == "lone-CR-not-normalized":
                dest = checkout / source_path
                dest.write_bytes(dest.read_bytes() + b"\r")
            if name == "promoted-original-status":
                m["preserved_collections"]["requirements"]["records"][0]["original_status_fields"]["status"] = "implemented"
            if name == "invented-test-route":
                e["rows"]["CHECK-context-group-link-match"]["axes"]["positive"]["tests"].append("missing_test.go::TestInvented")
            cm, ce = checkout / "manifest.json", checkout / "evidence.json"
            cm.write_text(json.dumps(m, ensure_ascii=False), encoding="utf-8")
            ce.write_text(json.dumps(e, ensure_ascii=False), encoding="utf-8")
            run = subprocess.run([sys.executable, str(verifier), "--root", str(checkout),
                                  "--manifest", str(cm), "--evidence", str(ce),
                                  "--strict-evidence"], text=True, encoding="utf-8", capture_output=True)
            result = parse_verifier_result(run, name, expected)
            results.append({"case": name, "exit_code": run.returncode,
                            "expected_exit_code": expected, "failures": result["failures"]})
    output = {"status": "PASS", "hash_convention": "CRLF-to-LF only; all other bytes remain significant",
              "original_value_preservation": "All fixture edits are in temporary copies; source IDs/statuses/provenance untouched",
              "scope": "Inventory checker controls only, no production/runtime acceptance", "cases": results, "failures": []}
    return output


def main():
    args = argument_parser().parse_args()
    args.protected_inputs = []
    args.inputs_known = False
    try:
        result = run_controls(args)
    except InventoryError as exc:
        result = failure_result(exc)
    except (OSError, KeyError, TypeError, ValueError, UnicodeError, AttributeError) as exc:
        result = failure_result(InventoryError(f'inventory control failure: {type(exc).__name__}: {exc}'))
    return emit_result(result, args.output, args.protected_inputs, args.inputs_known)


if __name__ == '__main__':
    sys.exit(main())
