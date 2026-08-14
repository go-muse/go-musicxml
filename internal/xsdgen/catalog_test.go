package xsdgen

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCatalogDuplicateURI(t *testing.T) {
	t.Parallel()

	const input = `
<catalog xmlns="urn:oasis:names:tc:entity:xmlns:xml:catalog">
	<uri name="https://schemas.example/schema.xsd" uri="first.xsd"/>
	<group>
		<uri name="https://schemas.example/schema.xsd" uri="second.xsd"/>
	</group>
</catalog>`

	catalog, err := parseCatalog(strings.NewReader(input))

	assert.Nil(t, catalog)
	assert.ErrorIs(t, err, ErrDuplicateCatalogURI)
}
