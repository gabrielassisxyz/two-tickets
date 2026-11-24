package httpapi

import (
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// stripRebrandManifestUpdate rewrites testdata/contract/api-routes.txt from
// the literals found under internal/. It exists so the manifest stays a
// generated file rather than a hand-kept list.
var stripRebrandManifestUpdate = flag.Bool("update", false, "rewrite the api route manifest")

var stripRebrandParamPattern = regexp.MustCompile(`\{[^}]+\}`)

// stripRebrandCollectRoutes walks the non-test Go files under internal/ and
// collects every string literal containing /api/v1/, reduced to its path
// template: a leading METHOD is dropped and each {name} becomes {}.
func stripRebrandCollectRoutes(t *testing.T, root string) []string {
	t.Helper()
	seen := map[string]bool{}
	fset := token.NewFileSet()
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			raw, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if !strings.Contains(raw, "/api/v1/") {
				return true
			}
			if strings.HasSuffix(raw, "/") {
				return true
			}
			reduced := raw
			if idx := strings.Index(reduced, " "); idx >= 0 {
				reduced = reduced[idx+1:]
			}
			if idx := strings.Index(reduced, "?"); idx >= 0 {
				reduced = reduced[:idx]
			}
			if !strings.Contains(reduced, "/api/v1/") {
				return true
			}
			reduced = stripRebrandParamPattern.ReplaceAllString(reduced, "{}")
			seen[reduced] = true
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	routes := make([]string, 0, len(seen))
	for r := range seen {
		routes = append(routes, r)
	}
	sort.Strings(routes)
	return routes
}

func TestStripRebrandRouteManifestMatchesLiterals(t *testing.T) {
	root := filepath.Join("..", "..", "internal")
	manifestPath := filepath.Join("..", "..", "testdata", "contract", "api-routes.txt")
	got := stripRebrandCollectRoutes(t, root)
	if *stripRebrandManifestUpdate {
		content := strings.Join(got, "\n") + "\n"
		if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(manifestPath, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest %s: %v (run with -update to create it)", manifestPath, err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	want := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			want = append(want, trimmed)
		}
	}
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("route manifest has %d routes, literals have %d:\nmanifest=%v\nliterals=%v", len(want), len(got), want, got)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("route manifest differs at line %d: manifest %q, literals %q\nfull manifest=%v\nfull literals=%v", i+1, want[i], got[i], want, got)
		}
	}
}
