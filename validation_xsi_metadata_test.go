package musicxml

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validationXSIDefaultCases() []validationSourceCase {
	var cases []validationSourceCase
	for _, test := range []struct {
		element, nominated string
		invalid            bool
	}{
		{"blocked", "Derived", true}, {"blocked", "Base", false}, {"clear", "Derived", false},
		{"type-blocked", "BlockedChild", true}, {"type-blocked", "Blocked", false},
		{"final", "FinalInherited", false}, {"final", "FinalRestricted", false}, {"simple", "FinalText", false}, {"simple", "RestrictedText", false},
	} {
		var issues []string
		if test.invalid {
			issues = []string{"/" + test.element + "/@{" + validationXSINamespace + "}type:xsi:type"}
		}
		cases = append(cases, validationSourceCase{name: test.element + "/" + test.nominated, source: wrapValidationElement(test.element, ` n:type="`+test.nominated+`"`, ""), issues: issues, oracleComparable: true})
	}
	return cases
}

func TestXSITypeGeneratedMetadata(t *testing.T) {
	t.Parallel()
	types := validationXSIDefaultsGenerated.Types
	assert.Equal(t, validationDerivationExtension, types[validationQName{Local: "Blocked"}].Complex.Block)
	assert.Zero(t, types[validationQName{Local: "Base"}].Complex.Block)
	assert.Zero(t, types[validationQName{Local: "Base"}].Complex.Final)
	assert.Equal(t, validationDerivationExtension, types[validationQName{Local: "FinalInherited"}].Complex.Final)
	assert.Equal(t, validationDerivationList|validationDerivationUnion, types[validationQName{Local: "FinalText"}].Simple.Final)
	assert.Equal(t, validationDerivationExtension, validationXSIDefaultsGenerated.Elements[validationQName{Local: "blocked"}].Final)
	assert.Zero(t, validationXSIDefaultsGenerated.Elements[validationQName{Local: "clear"}].Final)
	assert.True(t, validationXSIGenerated.Types[validationQName{Space: "urn:xsi-contract", Local: "Abstract"}].Complex.Abstract)
	for _, test := range validationXSIDefaultCases() {
		t.Run(test.name, func(t *testing.T) {
			assertValidationIssues(t, validateAttributeSource(t, &validationXSIDefaultsGenerated, test.source), test.issues)
		})
	}
}

func TestXSITypeSelectedAssembly(t *testing.T) {
	t.Parallel()
	makeSchema := func(base string) *validationSchemaSet {
		return &validationSchemaSet{Types: map[validationQName]*validationTypeSchema{{Local: "shared"}: {Simple: &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: base}}}}}, Elements: map[validationQName]*validationElementSchema{{Local: "value"}: {Name: validationQName{Local: "value"}, Type: validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "string"}}}}}
	}
	for _, test := range []struct {
		base  string
		valid bool
	}{{"string", true}, {"integer", false}} {
		t.Run(test.base, func(t *testing.T) {
			c := validateAttributeSource(t, makeSchema(test.base), wrapValidationElement("value", ` n:type="shared"`, "1"))
			assert.Equal(t, test.valid, len(c.issues) == 0)
		})
	}
	// A type existing in the other assembly is still unknown in this one.
	other := makeSchema("string")
	delete(other.Types, validationQName{Local: "shared"})
	c := validateAttributeSource(t, other, wrapValidationElement("value", ` n:type="shared"`, "1"))
	require.Len(t, c.issues, 1)
	assert.Equal(t, "xsi:type", c.issues[0].Constraint)
}

func TestXSITypeCacheBounds(t *testing.T) {
	for _, depth := range []int{8, 64, 256} {
		t.Run(fmt.Sprint(depth), func(t *testing.T) {
			set := &validationSchemaSet{Types: map[validationQName]*validationTypeSchema{}}
			base := validationQName{Space: validationXSDNamespace, Local: "string"}
			for index := range depth {
				name := validationQName{Local: fmt.Sprintf("type%d", index)}
				set.Types[name] = &validationTypeSchema{Simple: &validationSimpleSchema{Form: validationSimpleRestriction, Base: &validationSimpleMember{Name: base}}}
				base = name
			}
			c := newTestValidationContext(set)
			declared := &validationElementSchema{Name: validationQName{Local: "value"}, Type: validationTypeRef{Name: validationQName{Space: validationXSDNamespace, Local: "string"}}}
			node, err := parseValidationDocument([]byte(wrapValidationElement("value", ` n:type="`+base.Local+`"`, "text")))
			require.NoError(t, err)
			c.validateElement(node, declared, "/value")
			require.NoError(t, c.result())
			metadata, paths, semantics := len(c.typeMetadata), len(c.derivations), len(c.simpleSemantics)
			assert.LessOrEqual(t, metadata, depth+3)
			assert.LessOrEqual(t, paths, depth)
			assert.Equal(t, depth, semantics)
			allocations := testing.AllocsPerRun(20, func() { c.validateElement(node, declared, "/value") })
			require.NoError(t, c.result())
			assert.Equal(t, metadata, len(c.typeMetadata))
			assert.Equal(t, paths, len(c.derivations))
			assert.Equal(t, semantics, len(c.simpleSemantics))
			assert.Less(t, allocations, float64(8*depth+32))
			t.Logf("depth=%d metadata=%d derivation-pairs=%d semantics=%d repeated-allocations=%.0f", depth, metadata, paths, semantics, allocations)
		})
	}
}
