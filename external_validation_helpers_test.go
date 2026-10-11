package musicxml

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type validationSourceCase struct {
	name, source, schema string
	issues               []string
	oracleComparable     bool
	assessmentIncomplete bool
}

// wrapValidationElement assembles an original-source fixture, not serialized
// XML. Keep raw attribute/content fragments unchanged, including their spacing,
// references and markup; callers supply any leading attribute whitespace.
func wrapValidationElement(element, attributes, content string) string {
	return "<" + element + ` xmlns:n="` + validationXSINamespace + `"` + attributes + ">" + content + "</" + element + ">"
}

func validationSchemaFor(name string) *validationSchemaSet {
	switch name {
	case "musicxml.xsd":
		return &scoreValidationSchema
	case "opus.xsd":
		return &opusValidationSchema
	default:
		return &validationNilGenerated
	}
}

// newTestValidationContext gives each fixture independent validation state.
func newTestValidationContext(schema *validationSchemaSet) *validationContext {
	return &validationContext{
		schema:      schema,
		effective:   make(map[*validationComplexSchema]*validationEffectiveComplex),
		identifiers: make(map[string]string),
	}
}

func snapshotValidationSchema(t *testing.T, set *validationSchemaSet) map[string]string {
	t.Helper()
	result := make(map[string]string)
	for name, schema := range set.Types {
		encoded, err := json.Marshal(schema)
		require.NoError(t, err)
		result["type:"+name.Space+"\x00"+name.Local] = string(encoded)
	}
	for name, schema := range set.Elements {
		encoded, err := json.Marshal(schema)
		require.NoError(t, err)
		result["element:"+name.Space+"\x00"+name.Local] = string(encoded)
	}
	return result
}

func assertValidationIssues(t *testing.T, context *validationContext, want []string) {
	t.Helper()
	require.NoError(t, context.assessmentErr, "assessment is incomplete, not a successful or invalid assessment")
	var issues []string
	for _, issue := range context.issues {
		issues = append(issues, issue.Path+":"+issue.Constraint)
	}
	assert.ElementsMatch(t, want, issues, "diagnostics: %v", context.issues)
}

func assertXMLLintOutcome(t *testing.T, xmllint, schemaFile, catalog, source string, wantValid bool) {
	t.Helper()
	command := exec.Command(xmllint, "--nonet", "--noout", "--schema", schemaFile, "-")
	command.Env = append(os.Environ(), "XML_CATALOG_FILES="+catalog)
	command.Stdin = strings.NewReader(source)
	output, err := command.CombinedOutput()
	if wantValid {
		assert.NoErrorf(t, err, "independent XSD validation: %s", output)
		return
	}
	var exitError *exec.ExitError
	require.ErrorAsf(t, err, &exitError, "independent XSD validation unexpectedly accepted source: %s", output)
	assert.Equalf(t, 3, exitError.ExitCode(), "expected schema-invalid exit, got: %s", output)
}

func TestWrapValidationElement(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, element, attributes, content, want string }{
		{"empty", "empty", "", "", `<empty xmlns:n="http://www.w3.org/2001/XMLSchema-instance"></empty>`},
		{"raw attributes", "value", ` required='1' n:nil="&#x9;false&#xA;"`, "", `<value xmlns:n="http://www.w3.org/2001/XMLSchema-instance" required='1' n:nil="&#x9;false&#xA;"></value>`},
		{"raw content", "complex", ` id="own"`, " \t\r\n&#xA0;<!--comment--><?note text?><![CDATA[ ]]><child/>", "<complex xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\" id=\"own\"> \t\r\n&#xA0;<!--comment--><?note text?><![CDATA[ ]]><child/></complex>"},
		{"raw spacing", "value", "\t a=\"1\"\n b='2' ", "\r\n", "<value xmlns:n=\"http://www.w3.org/2001/XMLSchema-instance\"\t a=\"1\"\n b='2' >\r\n</value>"},
	} {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, wrapValidationElement(test.element, test.attributes, test.content))
		})
	}
}

func TestNewTestValidationContext(t *testing.T) {
	t.Parallel()
	schema := &validationSchemaSet{}
	first, second := newTestValidationContext(schema), newTestValidationContext(schema)
	assert.Same(t, schema, first.schema)
	assert.Same(t, schema, second.schema)
	require.NotNil(t, first.effective)
	require.NotNil(t, second.effective)
	require.NotNil(t, first.identifiers)
	require.NotNil(t, second.identifiers)
	first.effective[&validationComplexSchema{}] = &validationEffectiveComplex{}
	first.identifiers["seen"] = "/first"
	assert.Empty(t, second.effective)
	assert.Empty(t, second.identifiers)
}

func TestSnapshotValidationSchemaNamespaces(t *testing.T) {
	t.Parallel()
	schema := &validationSchemaSet{
		Types: map[validationQName]*validationTypeSchema{
			{Space: "urn:first", Local: "same"}:  {Simple: &validationSimpleSchema{WhiteSpace: "preserve"}},
			{Space: "urn:second", Local: "same"}: {Simple: &validationSimpleSchema{WhiteSpace: "collapse"}},
		},
		Elements: map[validationQName]*validationElementSchema{
			{Space: "urn:first", Local: "same"}:  {Nillable: true},
			{Space: "urn:second", Local: "same"}: {Nillable: false},
		},
	}
	before := snapshotValidationSchema(t, schema)
	require.Len(t, before, 4)
	assert.Equal(t, before, snapshotValidationSchema(t, schema))
	for _, namespace := range []string{"urn:first", "urn:second"} {
		name := validationQName{Space: namespace, Local: "same"}
		key := "type:" + namespace + "\x00same"
		require.Contains(t, before, key)
		schema.Types[name].Simple.Patterns = []string{"changed"}
		after := snapshotValidationSchema(t, schema)
		assert.NotEqual(t, before[key], after[key])
		schema.Types[name].Simple.Patterns = nil
		key = "element:" + namespace + "\x00same"
		require.Contains(t, before, key)
		schema.Elements[name].Abstract = true
		after = snapshotValidationSchema(t, schema)
		assert.NotEqual(t, before[key], after[key])
		schema.Elements[name].Abstract = false
	}
	assert.Equal(t, before, snapshotValidationSchema(t, schema))
}

// Include text explicitly: strings.Builder has no exported JSON fields.
func snapshotValidationNode(t *testing.T, node *validationNode) string {
	t.Helper()
	var texts []string
	var visit func(*validationNode)
	visit = func(node *validationNode) {
		texts = append(texts, node.Text.String())
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(node)
	encoded, err := json.Marshal(struct {
		Node  *validationNode
		Texts []string
	}{node, texts})
	require.NoError(t, err)
	return string(encoded)
}

func TestSnapshotValidationNodeText(t *testing.T) {
	t.Parallel()
	node, err := parseValidationDocument([]byte(`<root>before<child>child text</child>after</root>`))
	require.NoError(t, err)
	before := snapshotValidationNode(t, node)
	node.Children[0].Text.WriteString("changed")
	assert.NotEqual(t, before, snapshotValidationNode(t, node))
	before = snapshotValidationNode(t, node)
	node.Text.WriteString("changed")
	assert.NotEqual(t, before, snapshotValidationNode(t, node))
}
