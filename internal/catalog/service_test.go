package catalog

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/tmdb"
)

const (
	modulePath  = "github.com/gabrielassisxyz/two-tickets"
	httpapiPath = modulePath + "/internal/httpapi"
	tmdbPath    = modulePath + "/internal/tmdb"
)

// httpapi imports catalog to register its routes. The compiler only refuses
// the cycle while that import exists, so the rule is pinned here as well.
func TestCatalogDoesNotDependOnHTTPAPI(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, out)
	}
	deps := strings.Fields(string(out))
	// A listing that misses a dependency catalog certainly has would also
	// miss httpapi, so it proves nothing.
	if !containsPackage(deps, tmdbPath) {
		t.Fatalf("go list -deps does not list %s; the check cannot see imports:\n%s", tmdbPath, out)
	}
	if containsPackage(deps, httpapiPath) {
		t.Fatalf("catalog depends on %s; answer through internal/respond instead", httpapiPath)
	}
}

func containsPackage(deps []string, path string) bool {
	for _, dep := range deps {
		if dep == path {
			return true
		}
	}
	return false
}

func TestNewServiceKeepsTheClientAndDefaultsTheClock(t *testing.T) {
	client := tmdb.NewClient(tmdb.Options{})
	before := time.Now()
	service := NewService(client, nil)
	if service.tmdb != client {
		t.Error("the service does not hold the client it was given")
	}
	if got := service.now(); got.Before(before) || time.Since(got) > time.Minute {
		t.Errorf("default clock reads %v, want the current time", got)
	}

	fixed := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	if got := NewService(client, func() time.Time { return fixed }).now(); !got.Equal(fixed) {
		t.Errorf("injected clock reads %v, want %v", got, fixed)
	}
}
