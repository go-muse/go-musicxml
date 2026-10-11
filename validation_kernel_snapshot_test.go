package musicxml

import (
	"github.com/go-muse/go-musicxml/internal/xsd"
	"testing"
)

func TestGeneratedSchemaSnapshotsCompile(t *testing.T) {
	for name, data := range map[string]validationSchemaSet{"score": scoreValidationSchema, "opus": opusValidationSchema} {
		t.Run(name, func(t *testing.T) {
			c, e := xsd.Compile(data, xsd.CompileLimits{})
			if e != nil {
				t.Fatal(e)
			}
			if c.Digest() == "" || c.LogicalBytes() == 0 || len(c.Roots()) == 0 {
				t.Fatal("missing immutable schema metadata")
			}
		})
	}
}
