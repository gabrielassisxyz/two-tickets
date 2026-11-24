package httpapi

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// contractDir holds the response bodies the frontend depends on, shared with
// the web tests that render against them.
var contractDir = filepath.Join("..", "..", "testdata", "contract")

// assertMatchesContractGolden compares body to the committed golden as JSON
// values, so key order and indentation do not matter but every key and value
// does.
func assertMatchesContractGolden(t *testing.T, body []byte, golden string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(contractDir, golden))
	if err != nil {
		t.Fatal(err)
	}
	var want, got any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatalf("%s: %v", golden, err)
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("response is not JSON: %v: %s", err, body)
	}
	if !reflect.DeepEqual(got, want) {
		var pretty bytes.Buffer
		_ = json.Indent(&pretty, body, "", "  ")
		t.Fatalf("response differs from %s:\n%s", golden, pretty.String())
	}
}

func readUpstreamSettingsFields(t *testing.T) []string {
	t.Helper()
	file, err := os.Open(filepath.Join(contractDir, "settings-public.fields"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	var fields []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			fields = append(fields, line)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return fields
}

func TestPublicSettingsSetsEveryUpstreamField(t *testing.T) {
	s := startAuthServer(t, nil)
	resp := s.do(t, authRequest{method: http.MethodGet, path: routePublicSettings})
	if resp.status != http.StatusOK {
		t.Fatalf("GET %s without a session = %d, want 200", routePublicSettings, resp.status)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(resp.body, &body); err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(body))
	for key := range body {
		got = append(got, key)
	}
	want := readUpstreamSettingsFields(t)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("settings/public keys differ from the upstream field list:\n got %v\nwant %v", got, want)
	}
}

func TestPublicSettingsMatchesTheGolden(t *testing.T) {
	s := startAuthServer(t, nil)
	resp := s.do(t, authRequest{method: http.MethodGet, path: routePublicSettings})
	if resp.status != http.StatusOK {
		t.Fatalf("GET %s = %d", routePublicSettings, resp.status)
	}
	assertMatchesContractGolden(t, resp.body, "settings-public.json")
}
