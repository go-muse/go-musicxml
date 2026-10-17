package validation

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"hash"
)

type dictionaryEncoder struct {
	ctx     context.Context
	account *Account
	hash    hash.Hash
	bytes   int64
	err     error
	// Each digest/fingerprint invocation owns its encoder and scratch. Hash
	// Write inherits io.Writer's no-retention contract, so sequential writes
	// can reuse these buffers within the existing fixed scratch admission.
	countScratch [4]byte
	textScratch  [256]byte
}

func (w *dictionaryEncoder) write(data []byte) {
	if w.err != nil {
		return
	}
	if w.err = dictionaryStep(w.ctx, w.account, int64(len(data))+1); w.err != nil {
		return
	}
	_, _ = w.hash.Write(data)
	w.bytes += int64(len(data))
}
func (w *dictionaryEncoder) count(n uint32) {
	binary.BigEndian.PutUint32(w.countScratch[:], n)
	w.write(w.countScratch[:])
}
func (w *dictionaryEncoder) text(s string) {
	w.count(uint32(len(s)))
	for len(s) > 0 && w.err == nil {
		n := copy(w.textScratch[:], s)
		w.write(w.textScratch[:n])
		s = s[n:]
	}
}
func (w *dictionaryEncoder) row(kind string, fields ...string) {
	w.text(kind)
	w.count(uint32(len(fields)))
	for _, s := range fields {
		w.text(s)
	}
}
func dictionaryCodepointString(cp uint32) string {
	const digits = "0123456789ABCDEF"
	var buf [8]byte
	at := len(buf)
	for cp > 0 || len(buf)-at < 4 {
		at--
		buf[at] = digits[cp&15]
		cp >>= 4
	}
	at -= 2
	buf[at] = 'U'
	buf[at+1] = '+'
	return string(buf[at:])
}

func dictionaryDataDigest(ctx context.Context, a *Account, e *dictionaryEntry) ([32]byte, int64, error) {
	h := sha256.New()
	w := dictionaryEncoder{ctx: ctx, account: a, hash: h}
	w.write([]byte("go-musicxml.dictionary\x00"))
	w.count(1)
	m := e.manifest
	for _, s := range []string{string(m.Kind), m.Edition, m.ComponentName, m.TransformRevision, m.PolicyRevision} {
		w.text(s)
	}
	// The additive representation format seals its minimal compiled policy with
	// its typed rows. Every earlier manifest retains its original encoding.
	if m.ID == DictionaryManifestIPARepresentation2015Unicode13 {
		w.text(ipaRepresentationPolicySHA256)
	}
	w.count(uint32(m.Rows))
	switch m.ID {
	case DictionaryManifestSMuFL14Canonical:
		for _, g := range e.smufl.Glyphs {
			w.row("glyph", g.Name, dictionaryCodepointString(g.Codepoint))
			if w.err != nil {
				break
			}
		}
	case DictionaryManifestSMuFL14Ranges:
		for _, r := range e.smufl.Ranges {
			w.row("range", r.ID, dictionaryCodepointString(r.Start), dictionaryCodepointString(r.End))
			if w.err != nil {
				break
			}
		}
		for _, r := range e.smufl.Ranges {
			for _, name := range r.Names {
				w.row("range-member", r.ID, name)
				if w.err != nil {
					break
				}
			}
			if w.err != nil {
				break
			}
		}
	case DictionaryManifestStandardSounds40:
		for _, id := range e.sounds.IDs {
			w.row("sound", id)
			if w.err != nil {
				break
			}
		}
	case DictionaryManifestIPAPositive2015Unicode13, DictionaryManifestIPARepresentation2015Unicode13:
		for _, r := range e.ipa.Equivalences {
			w.row("canonical-equivalence", r.Encoding, r.EquivalentToEncoding, r.RowID)
			if w.err != nil {
				break
			}
		}
		for _, r := range e.ipa.Examples {
			w.row("example-sequence", r.Encoding, r.Category, r.RowID)
			if w.err != nil {
				break
			}
		}
		for _, r := range e.ipa.Symbols {
			w.row("symbol", r.Encoding, r.Category, r.RowID)
			if w.err != nil {
				break
			}
		}
	default:
		w.err = dictionaryConfiguration("unsupported encoding manifest")
	}
	var digest [32]byte
	if w.err != nil {
		return digest, w.bytes, w.err
	}
	h.Sum(digest[:0])
	return digest, w.bytes, ctx.Err()
}
