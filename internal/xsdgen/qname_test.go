package xsdgen

import (
	"encoding/xml"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNamespaceBindingsResolveQName(t *testing.T) {
	t.Parallel()

	bindings := NamespaceBindings{
		"":      "urn:default",
		"xs":    Namespace,
		"xlink": "http://www.w3.org/1999/xlink",
	}

	tests := []struct {
		name    string
		value   string
		want    QName
		wantErr error
	}{
		{
			name:  "qualified",
			value: "xs:string",
			want: QName{
				Namespace: Namespace,
				Local:     "string",
				Prefix:    "xs",
			},
		},
		{
			name:  "default namespace",
			value: "custom-type",
			want: QName{
				Namespace: "urn:default",
				Local:     "custom-type",
			},
		},
		{
			name:  "implicit XML namespace",
			value: "xml:lang",
			want: QName{
				Namespace: XMLNamespace,
				Local:     "lang",
				Prefix:    "xml",
			},
		},
		{
			name:    "undeclared prefix",
			value:   "other:value",
			wantErr: ErrUndeclaredPrefix,
		},
		{
			name:    "empty",
			wantErr: ErrInvalidQName,
		},
		{
			name:    "empty local name",
			value:   "xs:",
			wantErr: ErrInvalidQName,
		},
		{
			name:    "multiple colons",
			value:   "xs:string:value",
			wantErr: ErrInvalidQName,
		},
		{
			name:    "whitespace",
			value:   "xs: string",
			wantErr: ErrInvalidQName,
		},
		{
			name:    "invalid NCName",
			value:   "1value",
			wantErr: ErrInvalidQName,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual, err := bindings.ResolveQName(test.value)

			if test.wantErr != nil {
				assert.ErrorIs(t, err, test.wantErr)
				assert.Equal(t, QName{}, actual)

				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.want, actual)
			assert.Equal(t, test.value, actual.String())
			assert.Equal(
				t,
				xml.Name{
					Space: test.want.Namespace,
					Local: test.want.Local,
				},
				actual.XMLName(),
			)
		})
	}
}

func TestNamespaceBindingsResolveQNames(t *testing.T) {
	t.Parallel()

	actual, err := NamespaceBindings{
		"xs": Namespace,
	}.ResolveQNames("xs:decimal custom-type")

	assert.NoError(t, err)
	assert.Equal(
		t,
		[]QName{
			{
				Namespace: Namespace,
				Local:     "decimal",
				Prefix:    "xs",
			},
			{
				Local: "custom-type",
			},
		},
		actual,
	)
}
