package xsdgen

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
)

var (
	ErrDuplicateCatalogURI = errors.New("xsdgen: duplicate catalog URI")
	ErrTrailingCatalogData = errors.New("xsdgen: content after catalog")
)

type catalog struct {
	entries map[string]string
}

type catalogDocument struct {
	XMLName xml.Name       `xml:"urn:oasis:names:tc:entity:xmlns:xml:catalog catalog"`
	XMLBase string         `xml:"http://www.w3.org/XML/1998/namespace base,attr"`
	URIs    []catalogURI   `xml:"urn:oasis:names:tc:entity:xmlns:xml:catalog uri"`
	Groups  []catalogGroup `xml:"urn:oasis:names:tc:entity:xmlns:xml:catalog group"`
}

type catalogGroup struct {
	XMLBase string         `xml:"http://www.w3.org/XML/1998/namespace base,attr"`
	URIs    []catalogURI   `xml:"urn:oasis:names:tc:entity:xmlns:xml:catalog uri"`
	Groups  []catalogGroup `xml:"urn:oasis:names:tc:entity:xmlns:xml:catalog group"`
}

type catalogURI struct {
	Name    string `xml:"name,attr"`
	URI     string `xml:"uri,attr"`
	XMLBase string `xml:"http://www.w3.org/XML/1998/namespace base,attr"`
}

func parseCatalog(reader io.Reader) (*catalog, error) {
	var document catalogDocument

	decoder := xml.NewDecoder(reader)
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("xsdgen: decode catalog: %w", err)
	}

	if err := readCatalogTail(decoder); err != nil {
		return nil, err
	}

	result := &catalog{
		entries: make(map[string]string),
	}

	base := cleanCatalogPath(document.XMLBase)
	if err := result.addEntries(base, document.URIs, document.Groups); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *catalog) addEntries(
	base string,
	uris []catalogURI,
	groups []catalogGroup,
) error {
	for _, value := range uris {
		entryBase := path.Join(base, cleanCatalogPath(value.XMLBase))
		target := path.Join(entryBase, cleanCatalogPath(value.URI))

		existing, found := c.entries[value.Name]
		if found && existing != target {
			return fmt.Errorf(
				"%w: %q maps to both %q and %q",
				ErrDuplicateCatalogURI,
				value.Name,
				existing,
				target,
			)
		}

		c.entries[value.Name] = target
	}

	for _, group := range groups {
		groupBase := path.Join(base, cleanCatalogPath(group.XMLBase))
		if err := c.addEntries(groupBase, group.URIs, group.Groups); err != nil {
			return err
		}
	}

	return nil
}

func (c *catalog) resolve(location string) (string, bool) {
	target, found := c.entries[location]
	return target, found
}

func cleanCatalogPath(value string) string {
	if value == "" {
		return "."
	}

	return value
}

func readCatalogTail(decoder *xml.Decoder) error {
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("xsdgen: read catalog tail: %w", err)
		}

		switch value := token.(type) {
		case xml.CharData:
			if len(bytes.TrimSpace(value)) == 0 {
				continue
			}
		case xml.Comment, xml.ProcInst:
			continue
		}

		return fmt.Errorf("%w: %T", ErrTrailingCatalogData, token)
	}
}
