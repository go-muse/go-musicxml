package xsdgen

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratedNumericXMLCodecs(t *testing.T) {
	t.Parallel()
	file := parseSchemaFile(t, "numeric.xsd", `
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:simpleType name="amount"><xs:restriction base="xs:decimal"/></xs:simpleType>
  <xs:simpleType name="derived-amount"><xs:restriction base="amount"/></xs:simpleType>
  <xs:simpleType name="count"><xs:restriction base="xs:positiveInteger"/></xs:simpleType>
  <xs:simpleType name="derived-count"><xs:restriction base="count"/></xs:simpleType>
  <xs:complexType name="numeric-base">
    <xs:sequence>
      <xs:element name="decimal" type="xs:decimal"/>
      <xs:element name="unsigned" type="xs:unsignedByte"/>
      <xs:element name="decimals" type="xs:decimal" maxOccurs="unbounded"/>
      <xs:element name="counts" type="xs:nonNegativeInteger" maxOccurs="unbounded"/>
      <xs:element name="inline" minOccurs="0"><xs:complexType><xs:simpleContent><xs:extension base="xs:decimal"><xs:attribute name="count" type="xs:unsignedInt"/></xs:extension></xs:simpleContent></xs:complexType></xs:element>
    </xs:sequence>
    <xs:attribute name="amount" type="xs:decimal"/>
    <xs:attribute name="count" type="xs:unsignedShort"/>
  </xs:complexType>
  <xs:complexType name="numeric-derived"><xs:complexContent><xs:extension base="numeric-base"><xs:attribute name="extra" type="xs:string"/></xs:extension></xs:complexContent></xs:complexType>
  <xs:complexType name="numeric-choices"><xs:choice maxOccurs="unbounded">
    <xs:element name="amount" type="xs:decimal"/>
    <xs:element name="count" type="xs:positiveInteger"/>
    <xs:element name="inline"><xs:complexType><xs:simpleContent><xs:extension base="xs:decimal"/></xs:simpleContent></xs:complexType></xs:element>
  </xs:choice></xs:complexType>
  <xs:complexType name="named-content"><xs:simpleContent><xs:extension base="derived-amount"><xs:attribute name="count" type="derived-count"/></xs:extension></xs:simpleContent></xs:complexType>
  <xs:complexType name="float-control"><xs:sequence><xs:element name="double" type="xs:double"/></xs:sequence><xs:attribute name="float" type="xs:float"/></xs:complexType>
  <xs:element name="decimal-root" type="xs:decimal"/>
  <xs:element name="named-root" type="derived-amount"/>
  <xs:element name="unsigned-root" type="xs:nonNegativeInteger"/>
  <xs:element name="referenced-root" type="numeric-derived"/>
  <xs:element name="inline-root"><xs:complexType><xs:sequence><xs:element name="value" type="xs:decimal"/></xs:sequence></xs:complexType></xs:element>
</xs:schema>`)
	index, err := NewIndex(&Set{Files: []*SchemaFile{file}})
	require.NoError(t, err)
	simple, err := GenerateSimpleTypes(index, "musicxml")
	require.NoError(t, err)
	complex, err := GenerateComplexTypes(index, "musicxml")
	require.NoError(t, err)
	elements, err := GenerateElements(index, "musicxml", QName{Local: "decimal-root"}, QName{Local: "named-root"}, QName{Local: "unsigned-root"}, QName{Local: "referenced-root"}, QName{Local: "inline-root"})
	require.NoError(t, err)
	assert.NotContains(t, string(complex), "type xmlFloatControl ")
	assert.NotContains(t, string(simple), ") MarshalText(")
	runtimeSource, err := os.ReadFile(filepath.Join("..", "..", "xml_numeric.go"))
	require.NoError(t, err)
	directory := t.TempDir()
	files := map[string][]byte{
		"go.mod":          []byte("module numeric-test\n\ngo 1.26\n"),
		"simple.go":       simple,
		"complex.go":      complex,
		"elements.go":     elements,
		"xml_numeric.go":  runtimeSource,
		"numeric_test.go": []byte(generatedNumericRuntimeTests),
	}
	for name, contents := range files {
		require.NoError(t, os.WriteFile(filepath.Join(directory, name), contents, 0600))
	}
	command := exec.Command(filepath.Join(runtime.GOROOT(), "bin", "go"), "test", ".")
	command.Dir = directory
	output, err := command.CombinedOutput()
	require.NoError(t, err, string(output))
}

const generatedNumericRuntimeTests = `package musicxml

import (
  "encoding/json"
  "encoding/xml"
  "reflect"
  "strings"
  "testing"
)

func TestNumericCodecs(t *testing.T) {
  const input = "<numeric-derived amount=\"0.0000001\" count=\"+2\" extra=\"kept\"><decimal>1000000</decimal><unsigned>+255</unsigned><decimals>1000000</decimals><decimals>0.0000001</decimals><counts>+1</counts><counts>-0</counts><inline count=\"+3\">0.0000001</inline></numeric-derived>"
  var value NumericDerived
  if err := xml.Unmarshal([]byte(input), &value); err != nil { t.Fatal(err) }
  if value.Decimal != 1e6 || value.Unsigned != 255 || len(value.Counts) != 2 || value.Counts[1] != 0 || value.Inline.Value != 1e-7 || *value.Inline.Count != 3 || *value.Extra != "kept" { t.Fatalf("wrong model: %#v", value) }
  encoded, err := xml.Marshal(value)
  if err != nil { t.Fatal(err) }
  if strings.Contains(string(encoded), "e+06") || strings.Contains(string(encoded), "e-07") { t.Fatal(string(encoded)) }
  var again NumericDerived
  if err := xml.Unmarshal(encoded, &again); err != nil { t.Fatal(err) }
  if !reflect.DeepEqual(value, again) { t.Fatalf("round trip: %#v", again) }
  var choices NumericChoices
  if err := xml.Unmarshal([]byte("<choices><amount>1000000</amount><count>+2</count><inline>0.0000001</inline></choices>"), &choices); err != nil { t.Fatal(err) }
  encoded, err = xml.Marshal(choices)
  if err != nil { t.Fatal(err) }
  if !strings.Contains(string(encoded), "<amount>1000000</amount><count>2</count><inline>0.0000001</inline>") { t.Fatal(string(encoded)) }
  var named NamedContent
  if err := xml.Unmarshal([]byte("<named-content count=\"+4\">0.0000001</named-content>"), &named); err != nil { t.Fatal(err) }
  encoded, err = xml.Marshal(named)
  if err != nil || string(encoded) != "<NamedContent count=\"4\">0.0000001</NamedContent>" { t.Fatalf("%s: %v", encoded, err) }
  for _, test := range []struct{ value any; want string }{
    {DecimalRoot{Value: 1e6}, "<decimal-root>1000000</decimal-root>"},
    {NamedRoot{Value: 1e-7}, "<named-root>0.0000001</named-root>"},
    {UnsignedRoot{Value: 1}, "<unsigned-root>1</unsigned-root>"},
    {InlineRoot{Value: 1e6}, "<inline-root><value>1000000</value></inline-root>"},
  } {
    encoded, err = xml.Marshal(test.value)
    if err != nil || string(encoded) != test.want { t.Fatalf("%s: %v; want %s", encoded, err, test.want) }
  }
  var root ReferencedRoot
  if err := xml.Unmarshal([]byte(strings.ReplaceAll(input, "numeric-derived", "referenced-root")), &root); err != nil { t.Fatal(err) }
  encoded, err = xml.Marshal(root)
  if err != nil || !strings.HasPrefix(string(encoded), "<referenced-root ") || !strings.Contains(string(encoded), "extra=\"kept\"") { t.Fatalf("%s: %v", encoded, err) }
  decimal, err := json.Marshal(DerivedAmount(1000000))
  if err != nil || string(decimal) != "1000000" { t.Fatalf("JSON changed: %s: %v", decimal, err) }
  var count DerivedCount
  if err := xml.Unmarshal([]byte("<count>+3</count>"), &count); err != nil || count != 3 { t.Fatalf("count %d: %v", count, err) }
  var float FloatControl
  if err := xml.Unmarshal([]byte("<float-control float=\"1e3\"><double>1e6</double></float-control>"), &float); err != nil { t.Fatal(err) }
  encoded, err = xml.Marshal(float)
  if err != nil || !strings.Contains(string(encoded), "1e+06") { t.Fatalf("double changed: %s: %v", encoded, err) }
}
`
