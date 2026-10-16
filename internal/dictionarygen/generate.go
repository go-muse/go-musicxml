// Package dictionarygen prepares the five existing pinned dictionary assets from
// committed local inputs. It performs no network access or upstream refresh.
package dictionarygen

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/go-muse/go-musicxml/validation"
)

type input struct {
	path     string
	manifest validation.DictionaryManifestID
}

var inputs = []input{
	{"smufl-1.4/glyphnames.json", validation.DictionaryManifestSMuFL14Canonical},
	{"smufl-1.4/ranges.json", validation.DictionaryManifestSMuFL14Ranges},
	{"musicxml-4.0/sounds.xml", validation.DictionaryManifestStandardSounds40},
	{"ipa-positive/runtime-positive-bundle.json", validation.DictionaryManifestIPAPositive2015Unicode13},
	{"ipa-representation/runtime-representation-bundle.json", validation.DictionaryManifestIPARepresentation2015Unicode13},
	{"NOTICES.txt", ""},
	{"ipa-positive/NOTICES.txt", ""},
	{"ipa-positive/LICENSE-UNICODE-V3.txt", ""},
	{"ipa-representation/NOTICES.txt", ""},
	{"ipa-representation/LICENSE-UNICODE-V3.txt", ""},
}

// Generate verifies all five committed components against the existing compiled
// manifests, then copies their exact bytes and accompanying notices into the
// runtime asset directory. Invalid or missing inputs leave all outputs untouched.
// Source line endings and derived-bundle identities are preserved verbatim.
// root is the repository root; this maintenance operation does not accept URIs.
func Generate(root string) error {
	// Finish every input check before creating a directory or changing an asset.
	// The accepted hashes already identify reviewed source syntax; parsing or
	// reserializing here would change the pinned original bytes and their meaning.
	raw := make([][]byte, len(inputs))
	for i, in := range inputs {
		// Notices are small committed text; cap them separately from exact pins.
		limit := int64(65536)
		var pin validation.DictionaryComponentPin
		if in.manifest != "" {
			manifest, ok := validation.LookupDictionaryManifest(in.manifest)
			if !ok {
				return fmt.Errorf("dictionary input %s: unsupported manifest", in.path)
			}
			pin = manifest.Component
			limit = pin.Bytes
		}
		data, err := readInput(filepath.Join(root, "testdata", "dictionaries", filepath.FromSlash(in.path)), limit, in.manifest != "")
		if err != nil {
			return fmt.Errorf("dictionary input %s: %w", in.path, err)
		}
		if in.manifest == "" && len(bytes.TrimSpace(data)) == 0 {
			return fmt.Errorf("dictionary input %s: attribution and license notices must not be empty", in.path)
		}
		if in.manifest != "" && fmt.Sprintf("%x", sha256.Sum256(data)) != pin.SHA256 {
			return fmt.Errorf("dictionary input %s: bytes do not match pinned manifest %s", in.path, in.manifest)
		}
		raw[i] = data
	}
	for i, in := range inputs {
		path := filepath.Join(root, "internal", "dictionarydata", "data", filepath.FromSlash(in.path))
		old, err := os.ReadFile(path)
		if err == nil && bytes.Equal(old, raw[i]) {
			continue
		}
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("dictionary asset %s: %w", in.path, err)
		}
		if err := replace(path, raw[i]); err != nil {
			return fmt.Errorf("dictionary asset %s: %w", in.path, err)
		}
	}
	return nil
}

// Check the on-disk length before allocating, then bound the read as well in
// case a file changes after Stat. Only committed regular files are inputs.
func readInput(path string, limit int64, exact bool) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit || exact && info.Size() != limit {
		return nil, fmt.Errorf("expected a regular file of the bounded pinned size")
	}
	raw, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > limit || exact && int64(len(raw)) != limit {
		return nil, fmt.Errorf("input length changed or exceeds its bound")
	}
	return raw, nil
}

func replace(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".dictionary-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err := f.Chmod(0644); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(raw); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
