package xsdgen

import (
	"encoding/xml"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveMusicXMLReferences(t *testing.T) {
	t.Parallel()

	schemaFS := os.DirFS(filepath.Join(
		"..",
		"..",
		"schema",
		"musicxml-4.0",
	))

	set, err := Load(schemaFS, "musicxml.xsd", "catalog.xml")
	require.NoError(t, err)

	index, err := NewIndex(set)
	require.NoError(t, err)

	for _, file := range set.Files {
		reader, err := schemaFS.Open(file.Path)
		require.NoError(t, err)

		decoder := xml.NewDecoder(reader)

		for {
			token, tokenErr := decoder.Token()
			if errors.Is(tokenErr, io.EOF) {
				break
			}
			require.NoError(t, tokenErr)

			start, ok := token.(xml.StartElement)
			if !ok || start.Name.Space != Namespace {
				continue
			}

			for _, attribute := range start.Attr {
				if attribute.Name.Space != "" {
					continue
				}

				switch attribute.Name.Local {
				case "type", "base":
					_, err = index.ResolveType(file, attribute.Value)

				case "ref":
					switch start.Name.Local {
					case "element":
						_, err = index.ResolveElement(file, attribute.Value)

					case "attribute":
						_, err = index.ResolveAttribute(file, attribute.Value)

					case "group":
						_, err = index.ResolveGroup(file, attribute.Value)

					case "attributeGroup":
						_, err = index.ResolveAttributeGroup(
							file,
							attribute.Value,
						)

					default:
						continue
					}

				default:
					continue
				}

				require.NoErrorf(
					t,
					err,
					"%s: <%s> %s=%q",
					file.Path,
					start.Name.Local,
					attribute.Name.Local,
					attribute.Value,
				)
			}
		}

		require.NoError(t, reader.Close())
	}
}
