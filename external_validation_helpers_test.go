package musicxml

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
