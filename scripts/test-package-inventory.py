#!/usr/bin/env python3
"""Exercise inventory integrity under LF/CRLF/mixed checkouts and real mutations.

Only temporary copies are changed. This does not execute production Go code or
establish semantic/resource acceptance.
"""
import argparse
import json
import subprocess
import sys
import tempfile
from pathlib import Path


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path("."))
    parser.add_argument("--manifest", type=Path,
                        default=Path("docs/validation/package-assessment-ownership.json"))
    parser.add_argument("--evidence", type=Path,
                        default=Path("docs/validation/evidence/package-row-evidence.json"))
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    root = args.root.resolve()
    mp = args.manifest if args.manifest.is_absolute() else root / args.manifest
    ep = args.evidence if args.evidence.is_absolute() else root / args.evidence
    manifest = json.loads(mp.read_text(encoding="utf-8"))
    evidence = json.loads(ep.read_text(encoding="utf-8"))
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
            result = json.loads(run.stdout)
            if run.returncode != expected:
                raise AssertionError((name, expected, run.returncode, result, run.stderr))
            results.append({"case": name, "exit_code": run.returncode,
                            "expected_exit_code": expected, "failures": result["failures"]})
    output = {"status": "PASS", "hash_convention": "CRLF-to-LF only; all other bytes remain significant",
              "original_value_preservation": "All fixture edits are in temporary copies; source IDs/statuses/provenance untouched",
              "scope": "Inventory checker controls only, no production/runtime acceptance", "cases": results}
    text = json.dumps(output, ensure_ascii=False, indent=2) + "\n"
    if args.output:
        args.output.write_text(text, encoding="utf-8")
    print(text, end="")


if __name__ == "__main__":
    main()
