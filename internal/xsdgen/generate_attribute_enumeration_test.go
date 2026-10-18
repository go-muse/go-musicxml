package xsdgen

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateAttributeEnumerations(t *testing.T) {
	t.Parallel()

	xlink := parseSchemaFile(t, "xlink.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="http://www.w3.org/1999/xlink">
	<xs:attribute name="href" type="xs:anyURI"/>
	<xs:attribute name="show">
		<xs:simpleType>
			<xs:restriction base="xs:token">
				<xs:enumeration value="new"/>
				<xs:enumeration value="replace"/>
			</xs:restriction>
		</xs:simpleType>
	</xs:attribute>
</xs:schema>`)
	other := parseSchemaFile(t, "other.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema" targetNamespace="urn:other">
	<xs:attribute name="mode">
		<xs:simpleType>
			<xs:restriction base="xs:token">
				<xs:enumeration value="fast"/>
			</xs:restriction>
		</xs:simpleType>
	</xs:attribute>
</xs:schema>`)
	index, err := NewIndex(&Set{Files: []*SchemaFile{xlink, other}})
	require.NoError(t, err)

	actual, err := GenerateComplexTypes(index, "example")
	require.NoError(t, err)
	assert.Contains(t, string(actual), `// Values of the xlink:show attribute. The fields that carry it are strings.
const (
	// XLinkShowNew is the "new" value of xlink:show.
	XLinkShowNew = "new"
	// XLinkShowReplace is the "replace" value of xlink:show.
	XLinkShowReplace = "replace"
)
`)
	assert.NotContains(t, string(actual), "Href")
	assert.False(t, strings.Contains(string(actual), "Fast"), "only the xlink and xml namespaces get constants")
}
