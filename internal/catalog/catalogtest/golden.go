// Package catalogtest is the shared test kit for the catalog routes: it loads
// the committed goldens, stands up a fake TMDB that answers from the recorded
// fixtures, and compares a response body with a golden under the same rule
// that produced the goldens. Import it from _test.go files only.
package catalogtest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Golden is one committed case under testdata/golden/<family>/<case>.json:
// the request upstream answered, the TMDB fixtures it was answered from, and
// the status and body it produced. Body keeps the golden's raw JSON, with the
// volatile values already replaced by type markers.
type Golden struct {
	Case     string          `json:"case"`
	Family   string          `json:"family"`
	Request  string          `json:"request"`
	Fixtures []string        `json:"fixtures"`
	Status   int             `json:"status"`
	Body     json.RawMessage `json:"body"`
}

// LoadGolden reads testdata/golden/<family>/<name>.json from the repository
// root and fails t if it is missing, malformed, or names a different case.
func LoadGolden(t testing.TB, family, name string) Golden {
	t.Helper()
	path := filepath.Join(repositoryRoot(t), "testdata", "golden", family, name+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("LoadGolden: %v", err)
	}
	var golden Golden
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatalf("LoadGolden %s/%s: %v", family, name, err)
	}
	if golden.Case != name || golden.Family != family {
		t.Fatalf("LoadGolden %s/%s: the file records case %q of family %q", family, name, golden.Case, golden.Family)
	}
	if len(golden.Body) == 0 {
		t.Fatalf("LoadGolden %s/%s: the golden has no body", family, name)
	}
	return golden
}

// repositoryRoot walks up from the working directory, which go test sets to
// the package under test, to the directory holding go.mod.
func repositoryRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("catalogtest: working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("catalogtest: %v", err)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("catalogtest: no go.mod above the working directory")
		}
		dir = parent
	}
}
