package musicxml

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-muse/go-musicxml/validation"
)

func TestZZUsage(t *testing.T) {
	var files []string
	for _, dir := range []string{"testdata/official", "testdata/corpus"} {
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && (strings.HasSuffix(path, ".xml") || strings.HasSuffix(path, ".musicxml")) {
				files = append(files, path)
			}
			return err
		})
	}
	big := validation.Budgets{MaxRuleSteps: 1 << 40, MaxBufferedBytes: 1 << 34}
	for i := 0; i < len(files); i += 3 {
		data, _ := os.ReadFile(files[i])
		var peak, steps int64
		prev := validationSetFinishObserver(func(stage string, a *validation.Account) {
			if stage == "finished" {
				peak, _ = a.Usage(validation.BudgetBufferedBytes)
				steps, _ = a.Usage(validation.BudgetRuleSteps)
			}
		})
		InspectXML(context.Background(), bytes.NewReader(data), SourceOptions{Profile: ProfileSourceLine, Budgets: big})
		l2 := fmt.Sprint(peak, steps)
		if doc, err := Decode(bytes.NewReader(data)); err == nil {
			AssessModel(context.Background(), doc, ModelOptions{Profile: ProfileModelSemantics, Budgets: big})
		}
		validationSetFinishObserver(prev)
		fmt.Println(files[i], l2, peak, steps)
	}
}
