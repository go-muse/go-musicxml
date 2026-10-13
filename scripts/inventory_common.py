"""Shared safe I/O for development-only package inventory tools (Python 3.9+)."""
import hashlib
import json
import os
from pathlib import Path
import tempfile


class InventoryError(Exception):
    """An input or I/O failure that must be reported in the JSON result."""


def digest(data):
    return hashlib.sha256(data).hexdigest()


def canonical_lf(data):
    """Normalize checkout CRLF only; lone CR and all other bytes are significant."""
    return data.replace(b'\r\n', b'\n')


def canonical(value):
    return digest(json.dumps(value, ensure_ascii=False, sort_keys=True,
                             separators=(',', ':')).encode('utf-8'))


def require(value, kind, location):
    # bool is a subclass of int, but not a valid record index or denominator.
    if type(value) is not kind:
        raise InventoryError(f'invalid inventory type: {location}: expected {kind.__name__}, got {type(value).__name__}')
    return value


def field(value, key, kind, location):
    require(value, dict, location)
    if key not in value:
        raise InventoryError(f'missing inventory field: {location}/{key}')
    return require(value[key], kind, f'{location}/{key}')


def read_bytes(path):
    try:
        return path.read_bytes()
    except OSError as exc:
        raise InventoryError(f'cannot read input: {path}: {exc}') from exc


def read_json(path):
    try:
        def no_duplicates(pairs):
            value = {}
            for key, item in pairs:
                if key in value:
                    raise ValueError(f'duplicate JSON key: {key}')
                value[key] = item
            return value
        return json.loads(read_bytes(path).decode('utf-8'), object_pairs_hook=no_duplicates)
    except (ValueError, UnicodeError) as exc:
        raise InventoryError(f'cannot parse JSON: {path}: {exc}') from exc


def repository_path(root, value):
    require(value, str, 'repository path')
    path = Path(value)
    if path.is_absolute() or '..' in path.parts:
        raise InventoryError(f'invalid repository path: {value}')
    return root / path


def reject_alias(output, protected):
    """Reject lexical, symlink and hard-link aliases before creating a tempfile."""
    output = Path(output)
    try:
        resolved = output.resolve()
        for path in protected:
            path = Path(path)
            if resolved == path.resolve() or (output.exists() and path.exists() and output.samefile(path)):
                raise InventoryError(f'output aliases protected input: {output} ({path})')
    except (OSError, RuntimeError) as exc:
        raise InventoryError(f'cannot validate output path: {output}: {exc}') from exc


def atomic_write_json(path, value):
    """Replace a result only after a same-directory temporary file closes cleanly."""
    path = Path(path)
    tmp = None
    try:
        text = json.dumps(value, ensure_ascii=False, indent=2) + '\n'
        with tempfile.NamedTemporaryFile(mode='w', encoding='utf-8', newline='\n',
                                         prefix='.' + path.name + '.', suffix='.tmp',
                                         dir=path.parent, delete=False) as stream:
            tmp = stream.name
            stream.write(text)
            stream.flush()
            os.fsync(stream.fileno())
        os.replace(tmp, path)
        tmp = None
    except (OSError, ValueError) as exc:
        message = f'cannot write JSON atomically: {path}: {exc}'
        if tmp is not None:
            try:
                os.unlink(tmp)
            except OSError as cleanup:
                message += f'; cannot clean temporary file {tmp}: {cleanup}'
        raise InventoryError(message) from exc


def input_paths(root, manifest, evidence, manifest_path, evidence_path, archive_dir=None):
    """Collect every protected file input, even when later schema checks fail."""
    paths = [manifest_path, evidence_path]
    paths.extend(Path(__file__).parent / name for name in (
        'inventory_common.py', 'verify-package-inventory.py',
        'refresh-package-inventory.py', 'test-package-inventory.py'))
    if isinstance(manifest, dict) and isinstance(manifest.get('source_files'), dict):
        paths.extend(repository_path(root, name) for name in manifest['source_files'])
    if isinstance(evidence, dict) and isinstance(evidence.get('tests'), dict):
        for record in evidence['tests'].values():
            if isinstance(record, dict) and isinstance(record.get('file'), str):
                paths.append(repository_path(root, record['file']))
    if archive_dir is not None:
        if isinstance(manifest, dict) and isinstance(manifest.get('external_archive'), list):
            for record in manifest['external_archive']:
                if isinstance(record, dict) and isinstance(record.get('artifact'), str):
                    paths.append(repository_path(archive_dir, record['artifact']))
        if isinstance(evidence, dict) and isinstance(evidence.get('historical_tests'), dict):
            for record in evidence['historical_tests'].values():
                if isinstance(record, dict) and isinstance(record.get('artifact'), str):
                    paths.append(repository_path(archive_dir, record['artifact']))
    return paths


def inputs_known(manifest, evidence):
    """Malformed routing tables cannot provide a complete safe output denylist."""
    return (isinstance(manifest, dict) and isinstance(manifest.get('source_files'), dict)
            and isinstance(evidence, dict) and isinstance(evidence.get('tests'), dict)
            and all(isinstance(record, dict) and isinstance(record.get('file'), str)
                    for record in evidence['tests'].values()))


def failure_result(exc):
    return {'result': 'FAIL', 'failures': [str(exc)], 'warnings': [],
            'semantic_resource_execution_acceptance': 'NOT CLAIMED'}


def emit_result(result, output=None, protected=(), inputs_known=True):
    if output is not None:
        try:
            reject_alias(output, protected)
            if not inputs_known:
                raise InventoryError('cannot write result: malformed inputs prevent complete output-alias protection; JSON is available on stdout')
            atomic_write_json(output, result)
        except InventoryError as exc:
            result['result'] = 'FAIL'
            if 'status' in result:
                result['status'] = 'FAIL'
            result.setdefault('failures', []).append(str(exc))
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return int(bool(result.get('failures')))
