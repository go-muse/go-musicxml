#!/usr/bin/env python3
"""Regression tests for safe, test-hash-only inventory maintenance (stdlib only)."""
import copy
import contextlib
import io
import importlib.util
import json
import os
import re
from pathlib import Path
import subprocess
import sys
sys.dont_write_bytecode = True
import tempfile
import unittest
from unittest import mock

SCRIPTS = Path(__file__).resolve().parent
ROOT = SCRIPTS.parent
MANIFEST = Path('docs/validation/package-assessment-ownership.json')
EVIDENCE = Path('docs/validation/evidence/package-row-evidence.json')


def module(name):
    spec = importlib.util.spec_from_file_location(name.replace('-', '_'), SCRIPTS / (name + '.py'))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


class MaintenanceTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.manifest = json.loads((ROOT / MANIFEST).read_text())
        cls.evidence = json.loads((ROOT / EVIDENCE).read_text())
        paths = set(cls.manifest['source_files'])
        paths.update(record['reconciliation'] for record in cls.manifest.get('reviewed_source_corrections', []))
        paths.update(r['file'] for r in cls.evidence['tests'].values())
        cls.originals = {p: (ROOT / p).read_bytes() for p in paths}

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(prefix='inventory-maintenance-')
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        for path, data in self.originals.items():
            dest = self.root / path
            dest.parent.mkdir(parents=True, exist_ok=True)
            dest.write_bytes(data)
        self.m = copy.deepcopy(self.manifest)
        self.e = copy.deepcopy(self.evidence)
        self.save()

    def save(self):
        for path, value in [(MANIFEST, self.m), (EVIDENCE, self.e)]:
            (self.root / path).write_text(json.dumps(value, ensure_ascii=False, indent=2) + '\n')

    def run_tool(self, script='verify-package-inventory', *args, expected=0, env=None):
        run = subprocess.run([sys.executable, str(SCRIPTS / (script + '.py')), '--root', str(self.root), *args],
                             text=True, encoding='utf-8', capture_output=True, env=env)
        try:
            result = json.loads(run.stdout)
        except ValueError:
            self.fail(f'{script}: expected JSON, exit={run.returncode}; stdout={run.stdout!r}; stderr={run.stderr!r}')
        self.assertEqual(run.returncode, expected, (result, run.stderr))
        self.assertNotIn('Traceback', run.stderr)
        return result

    def refresh(self, *args, expected=0):
        return self.run_tool('refresh-package-inventory', *args, expected=expected)

    def mutate_test(self):
        path = self.root / 'link_intent_test.go'
        path.write_bytes(path.read_bytes() + b'// Maintenance regression.\n')

    def test_dry_run_then_write_only_current_hashes_and_idempotence(self):
        self.mutate_test()
        old_m = (self.root / MANIFEST).read_bytes()
        old_e = (self.root / EVIDENCE).read_bytes()
        dry = self.refresh()
        self.assertEqual(dry['mode'], 'dry-run')
        self.assertEqual(dry['changed_test_files'], ['link_intent_test.go'])
        self.assertEqual((self.root / EVIDENCE).read_bytes(), old_e)
        self.assertEqual((self.root / MANIFEST).read_bytes(), old_m)
        self.refresh('--write')
        actual = json.loads((self.root / EVIDENCE).read_text())
        before = copy.deepcopy(self.e)
        for ref in before['tests']:
            before['tests'][ref]['file_sha256'] = actual['tests'][ref]['file_sha256']
        self.assertEqual(actual, before, 'refresh must preserve every non-current-hash value')
        self.assertEqual((self.root / MANIFEST).read_bytes(), old_m)
        self.run_tool('verify-package-inventory', '--strict-evidence')
        written = (self.root / EVIDENCE).read_bytes()
        self.assertEqual(self.refresh('--write')['changed_test_files'], [])
        self.assertEqual((self.root / EVIDENCE).read_bytes(), written)

    def test_crlf_and_mixed_are_noop_but_lone_cr_is_not(self):
        for name, data in self.originals.items():
            (self.root / name).write_bytes(data.replace(b'\r\n', b'\n').replace(b'\n', b'\r\n'))
        self.assertEqual(self.refresh()['changed_test_files'], [])
        p = self.root / 'link_intent_test.go'
        p.write_bytes(p.read_bytes().replace(b'\r\n', b'\n', 1))
        self.assertEqual(self.refresh()['changed_test_files'], [])
        p.write_bytes(p.read_bytes() + b'\r')
        self.assertEqual(self.refresh()['changed_test_files'], ['link_intent_test.go'])

    def test_source_whitespace_status_and_owner_changes_are_blocked(self):
        path = 'docs/validation/plan/context-dispositions.json'
        for mutation in ('whitespace', 'status', 'owner'):
            with self.subTest(mutation=mutation):
                source = json.loads(self.originals[path])
                if mutation == 'whitespace':
                    data = self.originals[path] + b' '
                else:
                    source['checks'][0][mutation] = 'unreviewed change'
                    data = json.dumps(source).encode()
                (self.root / path).write_bytes(data)
                before = (self.root / EVIDENCE).read_bytes()
                result = self.refresh('--write', expected=1)
                self.assertTrue(any('source' in f for f in result['failures']), result)
                self.assertEqual((self.root / EVIDENCE).read_bytes(), before)

    def test_blanket_source_refresh_is_not_supported(self):
        run = subprocess.run([sys.executable, str(SCRIPTS / 'refresh-package-inventory.py'), '--refresh-sources'], capture_output=True)
        self.assertNotEqual(run.returncode, 0)

    def test_duplicate_issue_routes_and_declared_selected_counts_are_rejected(self):
        self.m['selected']['issue_gates'][1] = copy.deepcopy(self.m['selected']['issue_gates'][0])
        self.save()
        self.refresh('--write', expected=1)
        self.m = copy.deepcopy(self.manifest)
        self.m['counts']['selected_engineering'] += 1
        self.save()
        self.refresh('--write', expected=1)

    def test_malformed_nested_types_and_missing_metadata_are_json_failures(self):
        mutations = [lambda: self.m['preserved_collections']['requirements'].update(pointer=12),
                     lambda: self.m['preserved_collections']['requirements']['records'][0].update(original_status_fields=[]),
                     lambda: self.m['selected']['requirements'][0].update(owner=[]),
                     lambda: self.e.update(rows=[]),
                     lambda: self.e['hashing'].update(test_file_sha256=12),
                     lambda: self.e['tests'][next(iter(self.e['tests']))].pop('file_sha256')]
        for change in mutations:
            with self.subTest(change=change):
                self.m, self.e = copy.deepcopy(self.manifest), copy.deepcopy(self.evidence)
                change()
                self.save()
                self.refresh('--write', expected=1)

    def test_source_record_hash_is_not_refreshed(self):
        self.m['preserved_collections']['requirements']['records'][0]['sha256'] = '0' * 64
        self.save()
        self.assertTrue(self.refresh('--write', expected=1)['failures'])

    def test_preserved_status_and_denominators_cannot_be_promoted(self):
        cases = [lambda: self.m['preserved_collections']['requirements']['records'][0]['original_status_fields'].update(status='implemented'),
                 lambda: self.m['preserved_collections']['requirements'].update(count=96),
                 lambda: self.m['selected']['requirements'].pop(),
                 lambda: self.m['preserved_collections']['requirements']['records'].pop(),
                 lambda: self.m['counts'].update(requirements=96)]
        for change in cases:
            with self.subTest(change=change):
                self.m = copy.deepcopy(self.manifest)
                change()
                self.save()
                self.refresh('--write', expected=1)

    def test_missing_source_and_test_files_are_json_failures(self):
        for p in ['link_intent_test.go', next(iter(self.m['source_files']))]:
            with self.subTest(path=p):
                (self.root / p).unlink()
                result = self.refresh('--write', expected=1)
                self.assertIn(p, '\n'.join(result['failures']))
                (self.root / p).write_bytes(self.originals[p])

    def test_missing_and_invented_test_routes_are_blocked(self):
        ref = next(iter(self.e['tests']))
        record = self.e['tests'].pop(ref)
        self.save()
        self.refresh('--write', expected=1)
        self.e['tests'][ref] = record
        record['function'] = 'TestInventedMissing'
        self.save()
        self.refresh('--write', expected=1)

    def test_route_key_must_match_file_and_function(self):
        key = next(iter(self.e['tests']))
        self.e['tests']['invented_test.go::TestInvented'] = self.e['tests'].pop(key)
        self.save()
        self.run_tool(expected=1)

    def test_auxiliary_test_routes_and_original_ids_are_validated(self):
        for change in ('test', 'id'):
            with self.subTest(change=change):
                self.e = copy.deepcopy(self.evidence)
                if change == 'test':
                    self.e['auxiliary_xsd_axes'][0]['axes']['positive']['tests'].append('unknown.go::TestUnknown')
                else:
                    self.e['auxiliary_xsd_axes'][0]['id'] = 'unknown-instance'
                self.save()
                self.refresh('--write', expected=1)

    def test_undeclared_source_route_is_rejected(self):
        self.m['source_files'].pop('docs/validation/plan/context-dispositions.json')
        self.save()
        self.refresh('--write', expected=1)

    def test_malformed_pointers_produce_specific_json_errors(self):
        for pointer in ['#/absent', '#/requirements/nope', '#/requirements/999999', '#/requirements/-1',
                        '#/requirements/00', '#/requirements/0/id/child', '#/bad~2escape', 'requirements']:
            with self.subTest(pointer=pointer):
                self.m['preserved_collections']['requirements']['pointer'] = pointer
                self.save()
                result = self.run_tool(expected=1)
                self.assertIn('unresolvable pointer:', '\n'.join(result['failures']))
                self.assertIn('implementation-plan.json', '\n'.join(result['failures']))

    def test_malformed_json_and_shapes_have_useful_errors(self):
        cases = [(MANIFEST, '{'), (EVIDENCE, '[]'),
                 (Path(next(iter(self.m['source_files']))), 'not JSON')]
        for p, data in cases:
            with self.subTest(path=p):
                old = (self.root / p).read_bytes()
                (self.root / p).write_text(data)
                result = self.run_tool(expected=1)
                self.assertIn(str(p), '\n'.join(result['failures']))
                (self.root / p).write_bytes(old)
        for value in [True, '0', -1, None]:
            with self.subTest(index=value):
                self.m['preserved_collections']['requirements']['records'][0]['index'] = value
                self.save()
                self.run_tool(expected=1)

    def test_missing_git_is_a_warning_not_traceback(self):
        result = self.run_tool(env=dict(os.environ, PATH=''))
        self.assertEqual(result['observed_current_head'], 'unavailable')
        self.assertTrue(any('git' in w.lower() for w in result['warnings']), result)

    def test_output_cannot_alias_any_input(self):
        paths = [MANIFEST, EVIDENCE, Path('link_intent_test.go'), Path(next(iter(self.m['source_files'])))]
        for p in paths:
            for script in ['verify-package-inventory', 'refresh-package-inventory', 'test-package-inventory']:
                with self.subTest(path=p, script=script):
                    before = (self.root / p).read_bytes()
                    result = self.run_tool(script, '--output', str(self.root / p), expected=1)
                    self.assertIn('protected input', '\n'.join(result['failures']))
                    self.assertEqual((self.root / p).read_bytes(), before)

    def test_symlink_and_hardlink_output_aliases_are_rejected(self):
        for kind in ['symlink', 'hardlink']:
            with self.subTest(kind=kind):
                alias = self.root / (kind + '.json')
                target = self.root / MANIFEST
                if kind == 'symlink':
                    alias.symlink_to(target)
                else:
                    os.link(target, alias)
                before = target.read_bytes()
                for script in ['verify-package-inventory', 'refresh-package-inventory', 'test-package-inventory']:
                    self.run_tool(script, '--output', str(alias), expected=1)
                    self.assertEqual(target.read_bytes(), before)

    def test_cyclic_root_returns_json_failure_without_touching_output(self):
        root = self.root / 'cyclic-root'
        root.symlink_to(root)
        output = self.root / 'existing-output.json'
        output.write_text('preserve existing output')
        for script in ['verify-package-inventory', 'refresh-package-inventory', 'test-package-inventory']:
            for report_args in [[], ['--output', str(output)]]:
                with self.subTest(script=script, report_args=report_args):
                    result = self.run_tool(script, '--root', str(root), *report_args, expected=1)
                    message = '\n'.join(result['failures'])
                    self.assertIn('cannot resolve inventory root', message)
                    self.assertIn(str(root), message)
                    self.assertEqual(output.read_text(), 'preserve existing output')

    def test_cyclic_output_symlink_returns_json_failure(self):
        alias = self.root / 'cycle.json'
        alias.symlink_to(alias)
        for script in ['verify-package-inventory', 'refresh-package-inventory', 'test-package-inventory']:
            with self.subTest(script=script):
                result = self.run_tool(script, '--output', str(alias), expected=1)
                self.assertIn('cannot validate output path', '\n'.join(result['failures']))

    def test_evidence_alias_of_source_cannot_be_written(self):
        target = self.root / 'link_intent_test.go'
        target.unlink()
        os.link(self.root / EVIDENCE, target)
        before = target.read_bytes()
        self.refresh('--write', expected=1)
        self.assertEqual(target.read_bytes(), before)

    def test_output_replace_failure_returns_json_and_leaves_no_temp(self):
        dest = self.root / 'directory-output'
        dest.mkdir()
        result = self.run_tool('verify-package-inventory', '--output', str(dest), expected=1)
        self.assertIn('write', '\n'.join(result['failures']))
        self.assertEqual(list(self.root.glob('.directory-output.*')), [])

    def test_missing_output_directory_returns_json_without_clobbering_inputs(self):
        before = (self.root / EVIDENCE).read_bytes()
        for script in ['verify-package-inventory', 'refresh-package-inventory', 'test-package-inventory']:
            with self.subTest(script=script):
                result = self.run_tool(script, '--output', str(self.root / 'absent/report.json'), expected=1)
                self.assertIn('cannot write JSON atomically', '\n'.join(result['failures']))
                self.assertNotEqual(result.get('status'), 'PASS')
                self.assertEqual((self.root / EVIDENCE).read_bytes(), before)

    def test_refresh_write_failure_does_not_claim_success(self):
        refresher = module('refresh-package-inventory')
        self.mutate_test()
        before = (self.root / EVIDENCE).read_bytes()
        stdout = io.StringIO()
        with mock.patch.object(sys, 'argv', ['refresh', '--root', str(self.root), '--write']), \
             mock.patch.object(refresher, 'atomic_write_json', side_effect=refresher.InventoryError('injected write failure')), \
             contextlib.redirect_stdout(stdout):
            code = refresher.main()
        result = json.loads(stdout.getvalue())
        self.assertEqual(code, 1)
        self.assertFalse(result['written'])
        self.assertIn('injected write failure', result['failures'])
        self.assertEqual((self.root / EVIDENCE).read_bytes(), before)

    def test_atomic_write_failures_preserve_original_and_report_cleanup(self):
        common = module('inventory_common')
        dest = self.root / 'result.json'
        dest.write_text('unchanged')
        for operation in ['replace', 'fsync']:
            with self.subTest(operation=operation), mock.patch.object(common.os, operation, side_effect=OSError(operation + ' failed')):
                with self.assertRaises(common.InventoryError) as exc:
                    common.atomic_write_json(dest, {'new': True})
                self.assertIn(operation + ' failed', str(exc.exception))
                self.assertEqual(dest.read_text(), 'unchanged')
                self.assertEqual(list(self.root.glob('.result.json.*')), [])
        with mock.patch.object(common.os, 'replace', side_effect=OSError('replace failed')), \
             mock.patch.object(common.os, 'unlink', side_effect=OSError('cleanup failed')):
            with self.assertRaises(common.InventoryError) as exc:
                common.atomic_write_json(dest, {'new': True})
            self.assertIn('cleanup failed', str(exc.exception))
            self.assertIn('replace failed', str(exc.exception))
        self.assertEqual(dest.read_text(), 'unchanged')

    def test_atomic_partial_write_flush_and_close_failures_clean_up(self):
        common = module('inventory_common')
        dest = self.root / 'result.json'
        dest.write_text('unchanged')
        original = common.tempfile.NamedTemporaryFile
        for operation in ['write', 'flush', 'close']:
            class FaultyStream:
                def __init__(self, *args, **kwargs):
                    self.stream = original(*args, **kwargs)
                    self.name = self.stream.name

                def __enter__(self):
                    return self

                def __exit__(self, *args):
                    self.stream.__exit__(*args)
                    if operation == 'close':
                        raise OSError('injected close failure')

                def write(self, text):
                    if operation == 'write':
                        self.stream.write(text[:3])
                        raise OSError('injected write failure')
                    return self.stream.write(text)

                def flush(self):
                    if operation == 'flush':
                        raise OSError('injected flush failure')
                    return self.stream.flush()

                def fileno(self):
                    return self.stream.fileno()

            with self.subTest(operation=operation), mock.patch.object(common.tempfile, 'NamedTemporaryFile', FaultyStream):
                with self.assertRaises(common.InventoryError) as exc:
                    common.atomic_write_json(dest, {'new': True})
                self.assertIn('injected ' + operation + ' failure', str(exc.exception))
                self.assertEqual(dest.read_text(), 'unchanged')
                self.assertEqual(list(self.root.glob('.result.json.*')), [])

    def test_refresh_checks_output_alias_before_evidence_write(self):
        self.mutate_test()
        before = (self.root / EVIDENCE).read_bytes()
        for target in [MANIFEST, EVIDENCE, Path('link_intent_test.go')]:
            with self.subTest(target=target):
                self.refresh('--write', '--output', str(self.root / target), expected=1)
                self.assertEqual((self.root / EVIDENCE).read_bytes(), before)

    def test_parse_failure_does_not_overwrite_undiscovered_input(self):
        (self.root / MANIFEST).write_text('{')
        target = self.root / 'link_intent_test.go'
        for script in ['verify-package-inventory', 'refresh-package-inventory', 'test-package-inventory']:
            with self.subTest(script=script):
                before = target.read_bytes()
                self.run_tool(script, '--output', str(target), expected=1)
                self.assertEqual(target.read_bytes(), before)

    def test_unreferenced_test_record_is_rejected(self):
        self.e['tests']['unused.go::TestUnused'] = {
            'file': 'unused.go', 'function': 'TestUnused', 'file_sha256': '0' * 64}
        (self.root / 'unused.go').write_text('package musicxml\nfunc TestUnused(t *testing.T) {}\n')
        self.save()
        self.refresh('--write', expected=1)

    def test_harness_parse_failure_includes_exit_and_stderr(self):
        harness = module('test-package-inventory')
        bad = subprocess.CompletedProcess(['verifier'], 23, '', 'specific tool failure')
        with self.assertRaises(harness.InventoryError) as exc:
            harness.parse_verifier_result(bad, 'probe', 0)
        self.assertIn('specific tool failure', str(exc.exception))
        self.assertIn('23', str(exc.exception))

    def test_existing_linux_jobs_require_maintenance_without_new_go_stage(self):
        expected = {'ci.yml': {'quality', 'test', 'race', 'xsd-conformance', 'fuzz'},
                    'release.yml': {'verify', 'publish'}}
        for name, jobs in expected.items():
            text = (ROOT / '.github/workflows' / name).read_text()
            body = text.split('jobs:\n', 1)[1]
            self.assertEqual(set(re.findall(r'^  ([a-z-]+):$', body, re.M)), jobs)
            steps = re.split(r'(?=^      - name:)', body, flags=re.M)
            matching = [step for step in steps if 'python3 scripts/test-package-inventory-maintenance.py' in step]
            self.assertEqual(len(matching), 1, name)
            self.assertIn("if: runner.os == 'Linux'", matching[0])
            self.assertIn('python3 scripts/verify-package-inventory.py --strict-evidence', matching[0])
            self.assertIn('python3 scripts/test-package-inventory.py', matching[0])
        stages = (SCRIPTS / 'check-all.sh').read_text()
        self.assertIn('all_stages=(mod generate format test vet race fuzz)', stages)

    def test_reviewed_reconciliation_output_is_protected(self):
        path = self.root / self.m['reviewed_source_corrections'][0]['reconciliation']
        before = path.read_bytes()
        for script in ['verify-package-inventory', 'refresh-package-inventory', 'test-package-inventory']:
            with self.subTest(script=script):
                path.write_bytes(before)
                result = self.run_tool(script, '--output', str(path), expected=1)
                self.assertIn('protected input', '\n'.join(result['failures']))
                self.assertEqual(path.read_bytes(), before)

    def test_malformed_reviewed_routes_cannot_write_evidence_or_output(self):
        output = self.root / 'result.json'
        output.write_text('existing result')
        for routes in [None, {}, ['not a record'], [{}], [{'reconciliation': 7}]]:
            with self.subTest(routes=routes):
                self.m['reviewed_source_corrections'] = routes
                self.save()
                self.mutate_test()
                before = (self.root / EVIDENCE).read_bytes()
                result = self.refresh('--write', '--output', str(output), expected=1)
                self.assertFalse(result['written'])
                self.assertEqual((self.root / EVIDENCE).read_bytes(), before)
                self.assertEqual(output.read_text(), 'existing result')

    def test_reviewed_auxiliary_schema_diff_is_narrow_and_reproducible(self):
        common = module('inventory_common')
        review = json.loads((ROOT / 'docs/validation/evidence/auxiliary-fixture-reference-review.json').read_text())
        self.assertEqual(review['review_comment_id'], 4255983792)
        source = common.canonical_lf((ROOT / review['source']).read_bytes())
        self.assertEqual(common.digest(source), review['after_sha256'])
        self.assertEqual(len(source), review['after_bytes'])
        original = json.loads(source)
        records = {record['instance_id']: record for record in review['records']}
        self.assertEqual(len(records), 36)
        self.assertEqual(set(records), {row['instance_id'] for row in original['xsd_instances']})
        for row in original['xsd_instances']:
            record = records[row['instance_id']]
            self.assertEqual(row['research_id'], record['research_id'])
            self.assertEqual(common.canonical(row), record['after_record_sha256'])
            self.assertEqual(record['changes'], [
                {'field': 'fixture_axes.missing_fact', 'before': ['TestAuxiliarySourceUnavailableEvidence'], 'after_field_absent': True},
                {'field': 'missing_fact_test', 'before_field_absent': True, 'after': 'TestAuxiliarySourceUnavailableEvidence'},
            ])
            self.assertEqual(row.pop('missing_fact_test'), 'TestAuxiliarySourceUnavailableEvidence')
            self.assertNotIn('missing_fact', row['fixture_axes'])
            row['fixture_axes']['missing_fact'] = ['TestAuxiliarySourceUnavailableEvidence']
            self.assertEqual(common.canonical(row), record['before_record_sha256'])
        # Reverse only the reviewed field relocation and recover the exact old LF
        # source bytes. This proves every other value, status and owner survived.
        restored = (json.dumps(original, ensure_ascii=False, indent=2) + '\n').encode()
        self.assertEqual(common.digest(restored), review['before_sha256'])
        self.assertEqual(len(restored), review['before_bytes'])
        self.assertEqual(self.m['source_files'][review['source']],
                         {'sha256': review['after_sha256'], 'bytes': review['after_bytes']})

    def test_historical_observation_does_not_compare_current_head(self):
        verifier = module('verify-package-inventory')
        args = verifier.argument_parser().parse_args(['--root', str(self.root)])
        with mock.patch.object(verifier.subprocess, 'check_output', side_effect=['published-head\n', '']):
            result = verifier.verify(args)
        self.assertEqual(result['failures'], [])
        self.assertEqual(result['observed_current_head'], 'published-head')
        self.assertEqual(result['historical_observation']['head'], self.m['snapshot']['head_at_end'])
        self.assertIn('historical', result['historical_observation']['interpretation'])
        self.assertFalse(any('Integration differs' in warning for warning in result['warnings']))

    def test_auxiliary_fixture_routes_match_typed_original(self):
        for mutation in ['unknown fixture', 'missing fixture', 'wrong axis', 'function as fixture', 'wrong function']:
            with self.subTest(mutation=mutation):
                self.e = copy.deepcopy(self.evidence)
                axes = self.e['auxiliary_xsd_axes'][0]['axes']
                if mutation == 'unknown fixture':
                    axes['positive']['case_labels'] = ['unknown fixture']
                elif mutation == 'missing fixture':
                    axes['positive']['case_labels'] = []
                elif mutation == 'wrong axis':
                    axes['positive']['case_labels'] = axes['negative']['case_labels']
                elif mutation == 'function as fixture':
                    axes['missing_fact']['case_labels'] = ['TestAuxiliarySourceUnavailableEvidence']
                else:
                    axes['missing_fact']['tests'] = axes['positive']['tests']
                self.save()
                result = self.run_tool('verify-package-inventory', expected=1)
                self.assertTrue(any('auxiliary fixture route' in failure for failure in result['failures']), result)

    def test_historical_hashes_are_never_refreshed(self):
        key = next(iter(self.e['historical_tests']))
        self.e['historical_tests'][key]['file_sha256'] = 'f' * 64
        self.save()
        self.mutate_test()
        self.refresh('--write')
        actual = json.loads((self.root / EVIDENCE).read_text())
        for field in ['historical_tests', 'execution_results', 'snapshot', 'final_acceptance']:
            self.assertEqual(actual[field], self.e[field])


if __name__ == '__main__':
    unittest.main(verbosity=2)
