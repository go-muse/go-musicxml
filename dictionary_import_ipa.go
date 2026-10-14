package musicxml

import (
	"bytes"

	"github.com/go-muse/go-musicxml/validation"
)

func decodeDictionaryIPA(c *dictionaryJSON) (*validation.IPAPositiveDictionaryData, error) {
	// The exact verified derived bundle has 1196 bytes through its records-array
	// opener. Its identity and partial-scope metadata are covered by the raw hash;
	// only finite typed records are materialized. This is no general JSON parser
	// and never fetches the publication/UCD URLs described in the source ledger.
	const prefixBytes = 1196
	if len(c.raw) < prefixBytes || !bytes.HasPrefix(c.raw, []byte("{\n  \"schema_version\": 1,\n")) || !bytes.HasSuffix(c.raw[:prefixBytes], []byte("\"records\": [")) {
		return nil, dictionaryImportError("unexpected certified IPA bundle prefix")
	}
	c.at = prefixBytes
	out := &validation.IPAPositiveDictionaryData{Symbols: make([]validation.IPASymbolEncoding, 0, 170), Examples: make([]validation.IPAAttestedExample, 0, 18), Equivalences: make([]validation.IPACanonicalEquivalent, 0, 9)}
	first := true
	for c.next(']', &first) {
		c.want('{')
		object := true
		mask := uint32(0)
		kind := 0
		var fields [3]string
		for c.next('}', &object) {
			switch string(c.str(11)) {
			case "record_type":
				c.field(&mask, 1)
				switch string(c.str(21)) {
				case "symbol":
					kind = 1
				case "example-sequence":
					kind = 2
				case "canonical-equivalence":
					kind = 3
				default:
					c.fail("unsupported IPA record kind")
				}
			case "fields":
				c.field(&mask, 2)
				c.want('[')
				for i := range fields {
					if i > 0 {
						c.want(',')
					}
					fields[i] = c.owned(24)
				}
				c.want(']')
			default:
				c.fail("unknown IPA record field")
			}
		}
		if mask != 3 {
			c.fail("incomplete IPA record")
		}
		if c.err != nil {
			break
		}
		switch kind {
		case 1:
			if len(out.Symbols) == cap(out.Symbols) {
				c.fail("excessive IPA symbols")
				break
			}
			out.Symbols = append(out.Symbols, validation.IPASymbolEncoding{Encoding: fields[0], Category: fields[1], RowID: fields[2]})
		case 2:
			if len(out.Examples) == cap(out.Examples) {
				c.fail("excessive IPA examples")
				break
			}
			out.Examples = append(out.Examples, validation.IPAAttestedExample{Encoding: fields[0], Category: fields[1], RowID: fields[2]})
		case 3:
			if len(out.Equivalences) == cap(out.Equivalences) {
				c.fail("excessive IPA equivalences")
				break
			}
			out.Equivalences = append(out.Equivalences, validation.IPACanonicalEquivalent{Encoding: fields[0], EquivalentToEncoding: fields[1], RowID: fields[2]})
		}
	}
	c.want('}')
	c.eof()
	if len(out.Symbols) != 170 || len(out.Examples) != 18 || len(out.Equivalences) != 9 || c.copied != 4233 {
		c.fail("incomplete IPA record counts")
	}
	return out, c.err
}
