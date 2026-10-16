package dictionarygen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Inspect the production command's full import graph, without test imports.
// Proxy environment variables cannot establish whether Generate can dial a
// network or launch an external downloader; these packages provide those paths.
func checkOfflineDependencies(dir, target string) error {
	cmd := exec.Command("go", "list", "-buildvcs=false", "-deps", "-f", "{{.ImportPath}}", target)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "GOPROXY=off", "GOSUMDB=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("list generator dependencies: %w\n%s", err, output)
	}
	var forbidden []string
	for _, path := range strings.Fields(string(output)) {
		switch path {
		case "net", "net/http", "crypto/tls", "os/exec":
			forbidden = append(forbidden, path)
		}
	}
	if len(forbidden) != 0 {
		return fmt.Errorf("offline generator has network or process dependencies: %s", strings.Join(forbidden, ", "))
	}
	return nil
}

func TestGeneratorProductionDependenciesRemainOffline(t *testing.T) {
	if err := checkOfflineDependencies(".", "./cmd/dictionarygen"); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratorOfflineGuardRejectsTransitiveNetworkAndExec(t *testing.T) {
	for _, dependency := range []string{"net", "net/http", "crypto/tls", "os/exec"} {
		t.Run(dependency, func(t *testing.T) {
			root := t.TempDir()
			// The root imports only a local helper. The forbidden dependency is
			// indirect, so inspecting just the command's imports would miss it.
			writeTestFile(t, filepath.Join(root, "go.mod"), []byte("module example.com/offline-probe\n\ngo 1.21\n"))
			writeTestFile(t, filepath.Join(root, "main.go"), []byte("package main\nimport _ \"example.com/offline-probe/helper\"\nfunc main() {}\n"))
			writeTestFile(t, filepath.Join(root, "helper", "helper.go"), []byte(fmt.Sprintf("package helper\nimport _ %q\n", dependency)))
			err := checkOfflineDependencies(root, ".")
			if err == nil || !strings.Contains(err.Error(), "offline generator has network or process dependencies:") || !strings.Contains(err.Error(), dependency) {
				t.Fatalf("transitive %s was not rejected by the dependency guard: %v", dependency, err)
			}
			t.Log(err)
		})
	}
}
