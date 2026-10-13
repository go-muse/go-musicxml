// Package packageinventory checks the preserved package-assessment inventory.
// It is a maintenance tool, not production validation or execution acceptance.
package packageinventory

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf8"
)

type inventoryError struct{ message string }

func (e inventoryError) Error() string { return e.message }
func fail(format string, args ...any)  { panic(inventoryError{fmt.Sprintf(format, args...)}) }
func must(err error) {
	if err != nil {
		panic(inventoryError{err.Error()})
	}
}
func digest(data []byte) string      { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func canonicalLF(data []byte) []byte { return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n")) }

type node struct {
	value    any
	location string
}

func (n node) object() map[string]any {
	v, ok := n.value.(map[string]any)
	if !ok {
		fail("invalid inventory type: %s: expected object, got %s", n.location, jsonType(n.value))
	}
	return v
}
func (n node) list() []any {
	v, ok := n.value.([]any)
	if !ok {
		fail("invalid inventory type: %s: expected array, got %s", n.location, jsonType(n.value))
	}
	return v
}
func (n node) text() string {
	v, ok := n.value.(string)
	if !ok {
		fail("invalid inventory type: %s: expected string, got %s", n.location, jsonType(n.value))
	}
	return v
}

// Use JSON vocabulary rather than Go's decoder representation in diagnostics.
func jsonType(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case json.Number:
		if !strings.ContainsAny(string(v), ".eE") {
			return "integer"
		}
		return "number"
	default:
		return "non-JSON value"
	}
}

// Inventory integers have Python's arbitrary precision. Validate the token here;
// counts and sizes are compared before any bounded machine-index conversion.
func (n node) integerValue() *big.Int {
	v, ok := n.value.(json.Number)
	if !ok || strings.ContainsAny(string(v), ".eE") {
		fail("invalid inventory type: %s: expected integer, got %s", n.location, jsonType(n.value))
	}
	i, ok := new(big.Int).SetString(string(v), 10)
	if !ok {
		fail("invalid inventory integer: %s: %s", n.location, v)
	}
	return i
}
func (n node) integer() int64 {
	i := n.integerValue()
	if !i.IsInt64() {
		fail("inventory integer out of machine range: %s: %s", n.location, i)
	}
	return i.Int64()
}
func (n node) get(key string) node {
	v, ok := n.object()[key]
	if !ok {
		fail("missing inventory field: %s/%s", n.location, key)
	}
	return node{v, n.location + "/" + key}
}
func (n node) optional(key string, fallback any) node {
	v, ok := n.object()[key]
	if !ok {
		v = fallback
	}
	return node{v, n.location + "/" + key}
}
func (n node) at(i int) node {
	a := n.list()
	if i < 0 || i >= len(a) {
		fail("invalid inventory index: %s/%d", n.location, i)
	}
	return node{a[i], fmt.Sprintf("%s/%d", n.location, i)}
}
func (n node) fields(schema map[string]string) {
	for _, key := range sortedKeys(schema) {
		v := n.get(key)
		switch schema[key] {
		case "object":
			v.object()
		case "array":
			v.list()
		case "string":
			v.text()
		case "integer":
			v.integerValue()
		}
	}
}
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Decode recursively rather than into structs/maps directly: encoding/json's
// default duplicate-key and invalid-string replacement would hide mutations.
func parseJSON(raw []byte, location string) any {
	if !utf8.Valid(raw) {
		fail("cannot parse JSON: %s: invalid UTF-8", location)
	}
	if err := checkSurrogates(raw); err != nil {
		fail("cannot parse JSON: %s: %v", location, err)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var read func() (any, error)
	read = func() (any, error) {
		tok, err := d.Token()
		if err != nil {
			return nil, err
		}
		switch v := tok.(type) {
		case json.Delim:
			switch v {
			case '{':
				m := map[string]any{}
				for d.More() {
					k, err := d.Token()
					if err != nil {
						return nil, err
					}
					key, ok := k.(string)
					if !ok {
						return nil, errors.New("object key is not a string")
					}
					if _, exists := m[key]; exists {
						return nil, fmt.Errorf("duplicate JSON key: %s", key)
					}
					value, err := read()
					if err != nil {
						return nil, err
					}
					m[key] = value
				}
				_, err = d.Token()
				return m, err
			case '[':
				a := []any{}
				for d.More() {
					value, err := read()
					if err != nil {
						return nil, err
					}
					a = append(a, value)
				}
				_, err = d.Token()
				return a, err
			default:
				return nil, errors.New("unexpected closing delimiter")
			}
		default:
			return tok, nil
		}
	}
	v, err := read()
	if err != nil {
		fail("cannot parse JSON: %s: %v", location, err)
	}
	if _, err = d.Token(); err != io.EOF {
		fail("cannot parse JSON: %s: trailing data: %v", location, err)
	}
	return v
}
func checkSurrogates(raw []byte) error {
	in := false
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			in = !in
			continue
		}
		if !in || raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) || raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			continue
		}
		n, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			continue
		}
		i += 4
		if n >= 0xdc00 && n <= 0xdfff {
			return errors.New("unpaired Unicode low surrogate")
		}
		if n >= 0xd800 && n <= 0xdbff {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return errors.New("unpaired Unicode high surrogate")
			}
			low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return errors.New("unpaired Unicode high surrogate")
			}
			i += 6
		}
	}
	return nil
}

func canonicalJSON(value any) []byte {
	var b bytes.Buffer
	appendCanonical(&b, value)
	return b.Bytes()
}
func appendJSONString(b *bytes.Buffer, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(b, `\u%04x`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}
func appendCanonical(b *bytes.Buffer, v any) {
	switch v := v.(type) {
	case nil:
		b.WriteString("null")
	case bool:
		b.WriteString(strconv.FormatBool(v))
	case string:
		appendJSONString(b, v)
	case json.Number:
		b.WriteString(pythonNumber(v))
	case map[string]any:
		b.WriteByte('{')
		for i, k := range sortedKeys(v) {
			if i > 0 {
				b.WriteByte(',')
			}
			appendJSONString(b, k)
			b.WriteByte(':')
			appendCanonical(b, v[k])
		}
		b.WriteByte('}')
	case []any:
		b.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			appendCanonical(b, item)
		}
		b.WriteByte(']')
	default:
		fail("unsupported canonical JSON value: %T", v)
	}
}
func pythonNumber(v json.Number) string {
	s := string(v)
	if !strings.ContainsAny(s, ".eE") {
		n, ok := new(big.Int).SetString(s, 10)
		if !ok {
			fail("invalid JSON number: %s", s)
		}
		return n.String()
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
		fail("invalid finite JSON number: %s", s)
	}
	e := strconv.FormatFloat(f, 'e', -1, 64)
	parts := strings.Split(e, "e")
	exponent, _ := strconv.Atoi(parts[1])
	if exponent >= -4 && exponent < 16 {
		s = strconv.FormatFloat(f, 'f', -1, 64)
		if !strings.Contains(s, ".") {
			s += ".0"
		}
		return s
	}
	return e
}
func prettyJSON(v any) []byte {
	var b bytes.Buffer
	e := json.NewEncoder(&b)
	e.SetEscapeHTML(false)
	e.SetIndent("", "  ")
	if err := e.Encode(v); err != nil {
		fail("cannot encode JSON: %v", err)
	}
	return b.Bytes()
}
func readBytes(path string) []byte { return readBytesNamed(path, path) }
func readBytesNamed(path, displayName string) []byte {
	reportReadError := func(err error) {
		var pathError *os.PathError
		if path != displayName && errors.As(err, &pathError) {
			fail("cannot read input: %s: %s: %v", displayName, pathError.Op, pathError.Err)
		}
		fail("cannot read input: %s: %v", displayName, err)
	}
	// Resolve before native I/O: Windows normalizes '..' lexically before
	// following symlinks, which could otherwise read a different input.
	resolved, err := checkedResolve(path, defaultFileOps())
	if err != nil {
		reportReadError(err)
	}
	b, err := os.ReadFile(resolved)
	if err != nil {
		reportReadError(err)
	}
	return b
}
func readJSON(path string) node { return readJSONNamed(path, path) }
func readJSONNamed(path, displayName string) node {
	return node{parseJSON(readBytesNamed(path, displayName), displayName), displayName}
}
func repositoryPath(root, value string) string {
	if filepath.IsAbs(value) {
		fail("invalid repository path: %s", value)
	}
	for _, p := range strings.Split(filepath.ToSlash(value), "/") {
		if p == ".." {
			fail("invalid repository path: %s", value)
		}
	}
	resolved, err := checkedResolve(joinUnclean(root, value), defaultFileOps())
	if err != nil {
		fail("cannot resolve repository path: %s: %v", value, err)
	}
	return resolved
}

// Joining an input path must not erase a symlink/.. pair before the filesystem
// resolves it. Repository route values reject '..', but a caller-supplied root
// (notably --archive-dir) and explicit manifest/evidence paths may contain it.
func joinUnclean(base, value string) string {
	if base == "" {
		return value
	}
	if filepath.Separator == '\\' {
		baseVolume, valueVolume := filepath.VolumeName(base), filepath.VolumeName(value)
		if valueVolume != "" && !strings.EqualFold(baseVolume, valueVolume) {
			return value
		}
		if valueVolume != "" {
			value = value[len(valueVolume):]
		}
		if len(value) > 0 && os.IsPathSeparator(value[0]) {
			return baseVolume + value
		}
	}
	// Only a drive-letter volume is drive-relative. An exact UNC share is a
	// directory and still needs a separator before its child name.
	if volume := filepath.VolumeName(base); len(volume) == 2 && volume[1] == ':' && base == volume {
		return base + value
	}
	return base + string(filepath.Separator) + value
}

type atomicFile interface {
	io.Writer
	Flush() error
	Sync() error
	Close() error
	Name() string
}
type bufferedFile struct {
	*os.File
	writer *bufio.Writer
}

func (f *bufferedFile) Write(p []byte) (int, error) { return f.writer.Write(p) }
func (f *bufferedFile) Flush() error                { return f.writer.Flush() }

type fileOps struct {
	stat       func(string) (os.FileInfo, error)
	createTemp func(string, string) (atomicFile, error)
	rename     func(string, string) error
	remove     func(string) error
}

func defaultFileOps() fileOps {
	return fileOps{os.Stat, func(dir, pattern string) (atomicFile, error) {
		f, err := os.CreateTemp(dir, pattern)
		if err != nil {
			return nil, err
		}
		return &bufferedFile{f, bufio.NewWriter(f)}, nil
	}, os.Rename, os.Remove}
}

// Unlike EvalSymlinks, missing final paths and dangling links are supported.
// Resolve components before '..' so a symlinked parent cannot hide an alias.
func checkedResolve(path string, ops fileOps) (string, error) {
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		volume := filepath.VolumeName(path)
		switch {
		case len(volume) == 2 && volume[1] == ':':
			// Resolve only the drive's current directory. Abs(path) would erase
			// symlink/.. in the remainder before the filesystem follows it.
			anchor, err := filepath.Abs(volume + ".")
			if err != nil {
				return "", err
			}
			path = joinUnclean(anchor, path[len(volume):])
		case filepath.Separator == '\\' && len(path) > 0 && os.IsPathSeparator(path[0]):
			// Windows \child is relative to the current drive's root, not cwd.
			path = filepath.VolumeName(cwd) + path
		default:
			path = joinUnclean(cwd, path)
		}
	}
	volume := filepath.VolumeName(path)
	path = strings.TrimPrefix(path, volume)
	parts := strings.Split(filepath.ToSlash(path), "/")
	resolved := volume + string(filepath.Separator)
	links := 0
	for len(parts) > 0 {
		part := parts[0]
		parts = parts[1:]
		if part == "" || part == "." {
			continue
		}
		if part == ".." {
			resolved = filepath.Dir(resolved)
			continue
		}
		candidate := filepath.Join(resolved, part)
		info, err := os.Lstat(candidate)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				resolved = candidate
				continue
			}
			return "", err
		}
		if info.Mode()&os.ModeSymlink == 0 {
			if !info.IsDir() && len(parts) > 0 {
				return "", &os.PathError{Op: "resolve", Path: candidate, Err: syscall.ENOTDIR}
			}
			resolved = candidate
			continue
		}
		links++
		if links > 255 {
			return "", fmt.Errorf("too many symbolic links: %s", path)
		}
		target, err := os.Readlink(candidate)
		if err != nil {
			return "", err
		}
		if filepath.IsAbs(target) {
			volume = filepath.VolumeName(target)
			resolved = volume + string(filepath.Separator)
			target = strings.TrimPrefix(target, volume)
		}
		parts = append(strings.Split(filepath.ToSlash(target), "/"), parts...)
	}
	if _, err := ops.stat(resolved); err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	return resolved, nil
}
func resolveRoot(root string, ops fileOps) (string, error) {
	s, err := checkedResolve(root, ops)
	if err != nil {
		return "", fmt.Errorf("cannot resolve inventory root: %s: %w", root, err)
	}
	return s, nil
}
func rejectAlias(output string, protected []string, ops fileOps) error {
	resolved, err := checkedResolve(output, ops)
	if err != nil {
		return fmt.Errorf("cannot validate output path: %s: %w", output, err)
	}
	outInfo, err := ops.stat(resolved)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("cannot validate output path: %s: %w", output, err)
	}
	for _, path := range protected {
		p, err := checkedResolve(path, ops)
		if err != nil {
			return fmt.Errorf("cannot validate output path: %s: %w", output, err)
		}
		info, err := ops.stat(p)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cannot validate output path: %s: %w", output, err)
		}
		samePath := resolved == p || (filepath.Separator == '\\' && strings.EqualFold(resolved, p))
		if samePath || (outInfo != nil && info != nil && os.SameFile(outInfo, info)) {
			return fmt.Errorf("output aliases protected input: %s (%s)", output, path)
		}
	}
	return nil
}
func atomicWrite(path string, data []byte, ops fileOps) (err error) {
	// Split retains symlink/.. components; Dir would clean them and could put
	// the temporary file in a different directory or filesystem than the target.
	dir, name := filepath.Split(path)
	if dir == "" {
		dir = "."
	}
	// Resolve only the parent before native tempfile/rename calls can clean it.
	// Keep the final name: replacing a final symlink must not write its referent.
	dir, err = checkedResolve(dir, ops)
	if err != nil {
		return fmt.Errorf("cannot write JSON atomically: %s: %w", path, err)
	}
	destination := filepath.Join(dir, name)
	f, err := ops.createTemp(dir, "."+name+".*.tmp")
	if err != nil {
		return fmt.Errorf("cannot write JSON atomically: %s: %w", path, err)
	}
	tmp := f.Name()
	closed := false
	defer func() {
		if !closed {
			closeErr := f.Close()
			if err == nil {
				err = closeErr
			}
		}
		if err != nil {
			if cleanup := ops.remove(tmp); cleanup != nil {
				err = fmt.Errorf("%w; cannot clean temporary file %s: %v", err, tmp, cleanup)
			}
			err = fmt.Errorf("cannot write JSON atomically: %s: %w", path, err)
		}
	}()
	n, err := f.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	if err = f.Flush(); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	err = f.Close()
	closed = true
	if err != nil {
		return err
	}
	return ops.rename(tmp, destination)
}
