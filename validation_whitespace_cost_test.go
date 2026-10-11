package musicxml

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Exercise metadata sharing rather than generated schema size: each added
// restriction has one schema literal. Reassessing the complete base for that
// literal used to double the work at every level. Allocation growth is a
// deterministic regression signal here, with a generous allowance for linear
// or quadratic traversal; this is not a public validation-resource budget.
func TestWhitespaceEnumerationLayeredWork(t *testing.T) {
	// Keep this test nonparallel: AllocsPerRun observes process-wide allocations.
	builtin := func(name string) *validationSimpleMember {
		return &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: name}}
	}
	for _, kind := range []string{"atomic", "list", "union", "nested union", "list of union", "union of list"} {
		t.Run(kind, func(t *testing.T) {
			measure := func(depth int) float64 {
				base := builtin("string")
				switch kind {
				case "list":
					base = &validationSimpleMember{Inline: &validationSimpleSchema{Form: validationSimpleList, Item: builtin("token")}}
				case "union", "nested union":
					base = &validationSimpleMember{Inline: &validationSimpleSchema{Form: validationSimpleUnion, Members: []*validationSimpleMember{builtin("token"), builtin("string")}}}
				}
				if kind == "list of union" {
					base = &validationSimpleMember{Inline: &validationSimpleSchema{Form: validationSimpleList, Item: &validationSimpleMember{Inline: &validationSimpleSchema{Form: validationSimpleUnion, Members: []*validationSimpleMember{builtin("integer"), builtin("token")}}}}}
				}
				if kind == "union of list" {
					base = &validationSimpleMember{Inline: &validationSimpleSchema{Form: validationSimpleUnion, Members: []*validationSimpleMember{{Inline: &validationSimpleSchema{Form: validationSimpleList, Item: builtin("token")}}, builtin("string")}}}
				}
				var schema *validationSimpleSchema
				for range depth {
					schema = &validationSimpleSchema{Form: validationSimpleRestriction, Base: base, Enumerations: []string{"a"}}
					base = &validationSimpleMember{Inline: schema}
					if kind == "nested union" {
						base = &validationSimpleMember{Inline: &validationSimpleSchema{Form: validationSimpleUnion, Members: []*validationSimpleMember{base, builtin("string")}}}
					}
				}
				start := time.Now()
				allocations := testing.AllocsPerRun(3, func() {
					context := newTestValidationContext(&validationSchemaSet{})
					if failure := context.ValidateSimple(schema, "a"); failure != nil {
						t.Fatal(failure)
					}
				})
				t.Logf("depth=%d allocations=%.0f elapsed=%v", depth, allocations, time.Since(start))
				return allocations
			}
			shallow, deep := measure(8), measure(16)
			assert.LessOrEqual(t, deep, 8*shallow, "schema literal normalization must not recursively double base assessment")
		})
	}
}

func TestWhitespaceEnumerationLiteralViews(t *testing.T) {
	t.Parallel()
	builtin := func(name string) *validationSimpleMember {
		return &validationSimpleMember{Name: validationQName{Space: validationXSDNamespace, Local: name}}
	}
	union := func(first, second string) *validationSimpleMember {
		return &validationSimpleMember{Inline: &validationSimpleSchema{Form: validationSimpleUnion, Members: []*validationSimpleMember{builtin(first), builtin(second)}}}
	}
	context := newTestValidationContext(&validationSchemaSet{})
	for _, test := range []struct {
		name               string
		base               *validationSimpleMember
		literal            string
		accepted, rejected []string
	}{
		{"token first", union("token", "string"), " a ", []string{"a", " a ", "\ta\n"}, []string{"b", ""}},
		{"string first", union("string", "token"), " a ", []string{" a "}, []string{"a", "\ta\n", ""}},
		{"empty normalized", union("token", "string"), " \t ", []string{"", " ", "\n\t"}, []string{"a"}},
		{"padded preserved", union("string", "token"), " \t ", []string{" \t "}, []string{"", " "}},
		// Deliberately invalid metadata exercises cached failure without adding
		// schema-component validity checking to ordinary validation.
		{"invalid literal", union("integer", "boolean"), "invalid", nil, []string{"0", "1", "true"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			schema := &validationSimpleSchema{Form: validationSimpleRestriction, Base: test.base, Enumerations: []string{test.literal}}
			for range 3 {
				for _, value := range test.accepted {
					assert.Nil(t, context.ValidateSimple(schema, value), "%q", value)
				}
				for _, value := range test.rejected {
					failure := context.ValidateSimple(schema, value)
					if assert.NotNil(t, failure, "%q", value) {
						assert.Equal(t, "enumeration", failure.Constraint)
					}
				}
			}
			assert.Equal(t, []string{test.literal}, schema.Enumerations)
		})
	}
}

func TestWhitespaceEnumerationCacheBound(t *testing.T) {
	t.Parallel()
	context := newTestValidationContext(&validationSchemaSet{})
	schema := &validationSimpleSchema{Form: validationSimpleRestriction,
		Base: &validationSimpleMember{Inline: &validationSimpleSchema{Form: validationSimpleUnion,
			Members: []*validationSimpleMember{{Name: validationQName{Space: validationXSDNamespace, Local: "token"}}, {Name: validationQName{Space: validationXSDNamespace, Local: "string"}}}}},
		Enumerations: []string{" a ", " b ", " \t "}}
	for _, value := range []string{"not-listed", " a ", "a", "", " ", "b", " b "} {
		context.ValidateSimple(schema, value)
		assert.Len(t, context.EnumerationValues, len(schema.Enumerations))
	}
	// Unboundedly varied instance spellings must not grow the literal cache.
	for length := 1; length <= 100; length++ {
		context.ValidateSimple(schema, strings.Repeat("x", length))
		assert.Len(t, context.EnumerationValues, len(schema.Enumerations))
	}
	cached := context.EnumerationValues[validationEnumerationKey{Schema: schema, Index: 2}]
	assert.True(t, cached.Valid)
	assert.Empty(t, cached.Value)
}

func TestReplaceWhitespaceAllocations(t *testing.T) {
	// Keep this test nonparallel: AllocsPerRun observes process-wide allocations.
	for _, test := range []struct {
		name, input, want string
		maxAllocs         float64
	}{
		{"unchanged", "a b\u00a0c", "a b\u00a0c", 0},
		{"replace", "\ta\r\nb\u00a0c\t", " a  b\u00a0c ", 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			var got string
			allocations := testing.AllocsPerRun(100, func() {
				got = normalizeValidationWhitespace("normalizedString", test.input)
			})
			assert.Equal(t, test.want, got)
			// Bound reusable-replacer overhead, not public validation resources.
			assert.LessOrEqual(t, allocations, test.maxAllocs)
		})
	}
}
