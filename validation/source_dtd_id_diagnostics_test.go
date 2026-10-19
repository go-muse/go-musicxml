package validation

import (
	"context"
	"errors"
	"testing"
)

func TestSourceDTDIDDeclarationDiagnosticNamesExactField(t *testing.T) {
	for _, field := range []string{"element", "attribute"} {
		for _, lexical := range []string{"", "0bad", "p:0bad", "0bad:local", "a:b:c", "\xff"} {
			t.Run(field+"/"+lexical, func(t *testing.T) {
				s, key := dtdIDSession(t)
				f := dtdIDAdmittedAttribute(t, s, key, "a", "value")
				if field == "element" {
					f.Declaration.Element = lexical
				} else {
					f.Declaration.Attribute = lexical
				}
				err := s.sourceDTDIDAttribute(context.Background(), f)
				var configuration *ConfigurationError
				if !errors.As(err, &configuration) || configuration.Field != "source DTD ID declaration "+field {
					t.Fatalf("wrong declaration field diagnostic: %v", err)
				}
				s.cleanup()
				if used, _ := s.account.Usage(BudgetBufferedBytes); used != 0 {
					t.Fatalf("cleanup retained %d", used)
				}
			})
		}
	}
}
