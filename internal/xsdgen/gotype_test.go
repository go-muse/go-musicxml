package xsdgen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltinGoType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		builtin    string
		expression string
		kind       GoTypeKind
	}{
		{
			name:       "string",
			builtin:    "token",
			expression: "string",
			kind:       GoTypeString,
		},
		{
			name:       "boolean",
			builtin:    "boolean",
			expression: "bool",
			kind:       GoTypeBoolean,
		},
		{
			name:       "decimal",
			builtin:    "decimal",
			expression: "float64",
			kind:       GoTypeFloat,
		},
		{
			name:       "signed integer",
			builtin:    "int",
			expression: "int32",
			kind:       GoTypeSignedInteger,
		},
		{
			name:       "unsigned integer",
			builtin:    "positiveInteger",
			expression: "uint64",
			kind:       GoTypeUnsignedInteger,
		},
		{
			name:       "date",
			builtin:    "date",
			expression: "string",
			kind:       GoTypeString,
		},
		{
			name:       "binary",
			builtin:    "base64Binary",
			expression: "[]byte",
			kind:       GoTypeBytes,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual, err := BuiltinGoType(builtinDeclaration(
				test.builtin,
			))

			require.NoError(t, err)
			assert.Equal(t, test.expression, actual.Expression)
			assert.Equal(t, test.kind, actual.Kind)
		})
	}
}

func TestBuiltinGoTypeCoversIndexBuiltins(t *testing.T) {
	t.Parallel()

	for _, name := range builtinSimpleTypeNames {
		actual, err := BuiltinGoType(builtinDeclaration(name))

		require.NoError(t, err, name)
		assert.NotEmpty(t, actual.Expression, name)
		assert.NotEmpty(t, actual.Kind, name)
	}
}

func TestBuiltinGoTypeError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		declaration *Declaration
	}{
		{
			name: "nil declaration",
		},
		{
			name: "user type",
			declaration: &Declaration{
				Name: QName{Local: "value"},
				Kind: DeclarationSimpleType,
			},
		},
		{
			name: "unknown built-in",
			declaration: &Declaration{
				Name: QName{
					Namespace: Namespace,
					Local:     "unknown",
				},
				Kind: DeclarationBuiltinSimpleType,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual, err := BuiltinGoType(test.declaration)

			assert.Zero(t, actual)
			assert.ErrorIs(t, err, ErrUnsupportedBuiltinType)
		})
	}
}

func builtinDeclaration(name string) *Declaration {
	return &Declaration{
		Name: QName{
			Namespace: Namespace,
			Local:     name,
			Prefix:    "xs",
		},
		Kind: DeclarationBuiltinSimpleType,
	}
}
