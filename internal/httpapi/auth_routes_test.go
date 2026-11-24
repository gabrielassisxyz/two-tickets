package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gabrielassisxyz/two-tickets/internal/auth"
	"github.com/gabrielassisxyz/two-tickets/internal/config"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
)

// Low entropy on purpose: a random-looking value trips the secret scanner.
const (
	routerAlicePassword = "alicealicealicealice"
	routerBobPassword   = "bobbobbobbobbobbob"
	routerOrigin        = "https://tickets.example.com"
	routerLANOrigin     = "http://192.0.2.10:30080"
)

var (
	routerTestEpoch  = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	routerHashCache  sync.Map
	loopbackNetworks = []netip.Prefix{netip.MustParsePrefix("127.0.0.0/8"), netip.MustParsePrefix("::1/128")}
)

type routerTestClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *routerTestClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *routerTestClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func routerHash(t *testing.T, password string) string {
	t.Helper()
	if cached, ok := routerHashCache.Load(password); ok {
		return cached.(string)
	}
	encoded, err := auth.HashPassword(password, auth.MinimumArgon2idParams)
	if err != nil {
		t.Fatal(err)
	}
	routerHashCache.Store(password, encoded)
	return encoded
}

// authServer is the router over a migrated temporary state.db, the way the
// binary assembles it.
type authServer struct {
	*httptest.Server
	clock   *routerTestClock
	service *auth.Service
}

// startAuthServer serves the router. trusted lists the networks whose
// forwarding headers count; the test client always connects from loopback.
func startAuthServer(t *testing.T, trusted []netip.Prefix) *authServer {
	t.Helper()
	return startAuthServerWith(t, trusted, NewRouter)
}

// startAuthServerWith serves the handler build makes from the same deps.
func startAuthServerWith(t *testing.T, trusted []netip.Prefix, build func(RouterDeps) http.Handler) *authServer {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), store.StateFileName)
	db, err := store.OpenDatabase(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	migrations, err := store.LoadMigrations(store.StateMigrations())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MigrateState(ctx, db, migrations, filepath.Join(filepath.Dir(path), store.RollbackDirName)); err != nil {
		t.Fatal(err)
	}
	clock := &routerTestClock{now: routerTestEpoch}
	accounts := [2]config.Account{
		{ConfigKey: "first", Username: "alice", Email: "alice@example.com", DisplayName: "Alice", PasswordHash: config.NewSecret(routerHash(t, routerAlicePassword))},
		{ConfigKey: "second", Username: "bob", Email: "bob@example.com", DisplayName: "Bob", PasswordHash: config.NewSecret(routerHash(t, routerBobPassword))},
	}
	service, err := auth.NewService(ctx, db, accounts, clock.Now)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(build(RouterDeps{
		Readiness:      []ReadinessCheck{db},
		Auth:           service,
		AllowedOrigins: []string{routerOrigin, routerLANOrigin},
		TrustedProxies: trusted,
	}))
	t.Cleanup(server.Close)
	return &authServer{Server: server, clock: clock, service: service}
}

type authRequest struct {
	method, path string
	body         string
	headers      map[string]string
	token        string
}

func (s *authServer) do(t *testing.T, r authRequest) recordedResponse {
	t.Helper()
	var body io.Reader
	if r.body != "" {
		body = strings.NewReader(r.body)
	}
	req, err := http.NewRequestWithContext(context.Background(), r.method, s.URL+r.path, body)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range r.headers {
		req.Header.Set(k, v)
	}
	if r.token != "" {
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: r.token})
	}
	resp, err := s.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return recordedResponse{status: resp.StatusCode, header: resp.Header, body: raw}
}

func loginBody(identifier, password string) string {
	raw, _ := json.Marshal(map[string]string{"email": identifier, "password": password})
	return string(raw)
}

func withOrigin(extra map[string]string) map[string]string {
	headers := map[string]string{"Origin": routerOrigin, "Content-Type": "application/json"}
	for k, v := range extra {
		headers[k] = v
	}
	return headers
}

func (s *authServer) login(t *testing.T, identifier, password string, headers map[string]string) recordedResponse {
	t.Helper()
	return s.do(t, authRequest{method: http.MethodPost, path: routeLogin, body: loginBody(identifier, password), headers: withOrigin(headers)})
}

// sessionCookieOf returns the session cookie a response set, parsed, and its
// raw Set-Cookie line.
func sessionCookieOf(t *testing.T, resp recordedResponse) (*http.Cookie, string) {
	t.Helper()
	for _, line := range resp.header.Values("Set-Cookie") {
		cookie, err := http.ParseSetCookie(line)
		if err != nil {
			t.Fatalf("Set-Cookie %q: %v", line, err)
		}
		if cookie.Name == SessionCookieName {
			return cookie, line
		}
	}
	return nil, ""
}

func (s *authServer) mustLogin(t *testing.T, identifier, password string) string {
	t.Helper()
	resp := s.login(t, identifier, password, nil)
	if resp.status != http.StatusOK {
		t.Fatalf("login %q = %d %s", identifier, resp.status, resp.body)
	}
	cookie, _ := sessionCookieOf(t, resp)
	if cookie == nil || cookie.Value == "" {
		t.Fatalf("login set no session cookie: %v", resp.header)
	}
	return cookie.Value
}

// CRITERION 1, over HTTP.
func TestFailedLoginsAreIndistinguishable(t *testing.T) {
	s := startAuthServer(t, nil)
	attempts := map[string]string{
		"wrong password":   loginBody("alice", routerBobPassword),
		"unknown user":     loginBody("mallory", routerAlicePassword),
		"unknown email":    loginBody("mallory@example.com", routerAlicePassword),
		"not JSON":         "email=alice&password=x",
		"truncated JSON":   `{"email":"alice","password":`,
		"JSON array":       `["alice","x"]`,
		"missing password": `{"email":"alice"}`,
		"wrong types":      `{"email":1,"password":true}`,
		"oversized":        `{"email":"alice","password":"` + strings.Repeat("x", 20000) + `"}`,
		"empty body":       "",
	}
	var reference *recordedResponse
	for name, body := range attempts {
		resp := s.do(t, authRequest{method: http.MethodPost, path: routeLogin, body: body, headers: withOrigin(nil)})
		if cookie, _ := sessionCookieOf(t, resp); cookie != nil {
			t.Errorf("%s: a failed login set a session cookie", name)
		}
		if reference == nil {
			reference = &resp
		}
		if resp.status != http.StatusForbidden || string(resp.body) != string(reference.body) {
			t.Errorf("%s: %d %s, want 403 %s", name, resp.status, resp.body, reference.body)
		}
	}
	var body ErrorBody
	if err := json.Unmarshal(reference.body, &body); err != nil || body.Message != loginFailureMessage || body.Errors == nil {
		t.Fatalf("failure body = %s", reference.body)
	}
}

// CRITERION 2, over HTTP.
func TestLoginIdentifierIgnoresCaseAndSurroundingSpaces(t *testing.T) {
	s := startAuthServer(t, nil)
	var ids []int64
	for _, identifier := range []string{"alice", " ALICE  ", "Alice@Example.com", "\talice@example.com "} {
		resp := s.login(t, identifier, routerAlicePassword, nil)
		if resp.status != http.StatusOK {
			t.Fatalf("%q = %d %s", identifier, resp.status, resp.body)
		}
		var user userBody
		if err := json.Unmarshal(resp.body, &user); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, user.ID)
	}
	for _, id := range ids {
		if id != ids[0] {
			t.Fatalf("identifiers logged in different accounts: %v", ids)
		}
	}
}

func forwardedFor(addr string) map[string]string {
	return map[string]string{"X-Forwarded-For": addr}
}

// CRITERION 3, over HTTP behind a trusted proxy.
func TestLoginRateLimitsPerForwardedAddress(t *testing.T) {
	s := startAuthServer(t, loopbackNetworks)
	for i := 1; i <= auth.LoginAttemptsPerIdentifier; i++ {
		if resp := s.login(t, "alice", "wrong", forwardedFor("198.51.100.7")); resp.status != http.StatusForbidden {
			t.Fatalf("attempt %d = %d", i, resp.status)
		}
		s.clock.Advance(time.Minute)
	}
	limited := s.login(t, "alice", routerAlicePassword, forwardedFor("198.51.100.7"))
	if limited.status != http.StatusTooManyRequests {
		t.Fatalf("11th attempt = %d %s, want 429", limited.status, limited.body)
	}
	if retry := limited.header.Get("Retry-After"); retry == "" || retry == "0" {
		t.Errorf("Retry-After = %q", retry)
	}
	assertErrorShape(t, limited)
	// Another forwarded address has its own counter, even for the same account.
	if resp := s.login(t, "alice", routerAlicePassword, forwardedFor("198.51.100.8")); resp.status != http.StatusOK {
		t.Fatalf("another forwarded address = %d %s, want 200", resp.status, resp.body)
	}

	for i := 1; i < auth.LoginAttemptsPerAddress-auth.LoginAttemptsPerIdentifier; i++ {
		if resp := s.login(t, fmt.Sprintf("user%d", i), "wrong", forwardedFor("198.51.100.7")); resp.status != http.StatusForbidden {
			t.Fatalf("spread attempt %d = %d", i, resp.status)
		}
	}
	// 10 on alice and 19 spread: one slot left on the address ceiling.
	if resp := s.login(t, "bob", "wrong", forwardedFor("198.51.100.7")); resp.status != http.StatusForbidden {
		t.Fatalf("30th attempt = %d, want 403", resp.status)
	}
	if resp := s.login(t, "bob", routerBobPassword, forwardedFor("198.51.100.7")); resp.status != http.StatusTooManyRequests {
		t.Fatalf("31st attempt with a fresh identifier = %d, want 429", resp.status)
	}
}

// CRITERION 4: a browser cannot choose the address it is counted under.
func TestForwardedForFromAnUntrustedPeerIsIgnored(t *testing.T) {
	s := startAuthServer(t, nil)
	for i := 1; i <= auth.LoginAttemptsPerIdentifier; i++ {
		if resp := s.login(t, "alice", "wrong", forwardedFor(fmt.Sprintf("198.51.100.%d", i))); resp.status != http.StatusForbidden {
			t.Fatalf("attempt %d = %d", i, resp.status)
		}
	}
	if resp := s.login(t, "alice", "wrong", forwardedFor("203.0.113.99")); resp.status != http.StatusTooManyRequests {
		t.Fatalf("11th attempt under a new forged address = %d, want 429: the forged header was believed", resp.status)
	}
}

func TestClientAddressTakesTheRightmostUntrustedForwardedEntry(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("10.42.0.0/16")}
	cases := []struct {
		name, peer, xff, want string
	}{
		{"untrusted peer, header ignored", "203.0.113.5:4000", "198.51.100.1", "203.0.113.5"},
		{"trusted peer, single entry", "10.42.0.9:4000", "198.51.100.1", "198.51.100.1"},
		{"browser-prefixed entry is skipped", "10.42.0.9:4000", "6.6.6.6, 198.51.100.1", "198.51.100.1"},
		{"trusted hops are skipped", "10.42.0.9:4000", "198.51.100.1, 10.42.0.3", "198.51.100.1"},
		{"two header lines", "10.42.0.9:4000", "", "198.51.100.2"},
		{"garbage falls back to the peer", "10.42.0.9:4000", "not-an-ip", "10.42.0.9"},
		{"no header from a trusted peer", "10.42.0.9:4000", "", "10.42.0.9"},
		{"mapped IPv4 peer", "[::ffff:203.0.113.5]:4000", "198.51.100.1", "203.0.113.5"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, "/", nil)
			r.RemoteAddr = tc.peer
			if tc.xff != "" {
				r.Header.Set("X-Forwarded-For", tc.xff)
			}
			if tc.name == "two header lines" {
				r.Header.Add("X-Forwarded-For", "6.6.6.6")
				r.Header.Add("X-Forwarded-For", "198.51.100.2")
			}
			if got := clientAddress(r, trusted).String(); got != tc.want {
				t.Fatalf("clientAddress = %s, want %s", got, tc.want)
			}
		})
	}
}

// CRITERION 5.
func TestLoginCookieAttributes(t *testing.T) {
	cases := []struct {
		name       string
		trusted    []netip.Prefix
		headers    map[string]string
		wantSecure bool
	}{
		{"plain HTTP", loopbackNetworks, nil, false},
		{"proxy says https", loopbackNetworks, map[string]string{"X-Forwarded-Proto": "https"}, true},
		{"proxy says http", loopbackNetworks, map[string]string{"X-Forwarded-Proto": "http"}, false},
		{"browser says https", nil, map[string]string{"X-Forwarded-Proto": "https"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := startAuthServer(t, tc.trusted)
			resp := s.login(t, "alice", routerAlicePassword, tc.headers)
			if resp.status != http.StatusOK {
				t.Fatalf("login = %d %s", resp.status, resp.body)
			}
			cookie, line := sessionCookieOf(t, resp)
			if cookie == nil {
				t.Fatal("no session cookie")
			}
			if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.Domain != "" {
				t.Errorf("cookie attributes wrong: %s", line)
			}
			if !strings.Contains(line, "Max-Age=15552000") {
				t.Errorf("Max-Age missing or wrong: %s", line)
			}
			if want := routerTestEpoch.Add(180 * 24 * time.Hour); !cookie.Expires.Equal(want) {
				t.Errorf("Expires = %s, want %s", cookie.Expires, want)
			}
			if cookie.Secure != tc.wantSecure {
				t.Errorf("Secure = %v, want %v: %s", cookie.Secure, tc.wantSecure, line)
			}
		})
	}
}

// CRITERION 6, over HTTP: polling auth/me does not resend the cookie.
func TestSessionCookieIsRenewedOnlyAfter24Hours(t *testing.T) {
	s := startAuthServer(t, nil)
	token := s.mustLogin(t, "alice", routerAlicePassword)
	for range 3 {
		s.clock.Advance(15 * time.Minute)
		resp := s.do(t, authRequest{method: http.MethodGet, path: routeMe, token: token})
		if resp.status != http.StatusOK {
			t.Fatalf("auth/me = %d", resp.status)
		}
		if cookie, line := sessionCookieOf(t, resp); cookie != nil {
			t.Fatalf("auth/me within the first hour resent the cookie: %s", line)
		}
	}
	s.clock.Advance(25 * time.Hour)
	resp := s.do(t, authRequest{method: http.MethodGet, path: routeMe, token: token})
	cookie, line := sessionCookieOf(t, resp)
	if resp.status != http.StatusOK || cookie == nil {
		t.Fatalf("auth/me after 25 hours = %d with no renewed cookie", resp.status)
	}
	if want := s.clock.Now().Add(auth.SessionLifetime); !cookie.Expires.Equal(want) || cookie.Value != token {
		t.Fatalf("renewed cookie = %s, want the same token expiring %s", line, want)
	}
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || !strings.Contains(line, "Max-Age=15552000") {
		t.Fatalf("the renewed cookie lost an attribute: %s", line)
	}
}

func assertClearingCookie(t *testing.T, resp recordedResponse) {
	t.Helper()
	cookie, line := sessionCookieOf(t, resp)
	if cookie == nil {
		t.Fatal("logout did not clear the cookie")
	}
	if cookie.Value != "" || !strings.Contains(line, "Max-Age=0") || cookie.Path != "/" ||
		!cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Domain != "" {
		t.Fatalf("clearing cookie = %s", line)
	}
}

// CRITERION 7.
func TestLogoutAlwaysAnswersOKAndRevokesTheSession(t *testing.T) {
	s := startAuthServer(t, nil)
	token := s.mustLogin(t, "alice", routerAlicePassword)
	bobToken := s.mustLogin(t, "bob", routerBobPassword)
	for _, round := range []struct {
		name  string
		token string
	}{{"live session", token}, {"session already gone", token}, {"no cookie", ""}, {"garbage cookie", "garbage"}} {
		resp := s.do(t, authRequest{method: http.MethodPost, path: routeLogout, headers: withOrigin(nil), token: round.token})
		if resp.status != http.StatusOK || strings.TrimSpace(string(resp.body)) != `{"status":"ok"}` {
			t.Fatalf("%s: logout = %d %s", round.name, resp.status, resp.body)
		}
		assertClearingCookie(t, resp)
	}
	if resp := s.do(t, authRequest{method: http.MethodGet, path: routeMe, token: token}); resp.status != http.StatusUnauthorized {
		t.Fatalf("replayed token after logout = %d, want 401", resp.status)
	}
	if resp := s.do(t, authRequest{method: http.MethodGet, path: routeMe, token: bobToken}); resp.status != http.StatusOK {
		t.Fatalf("the partner's session after the logout = %d, want 200", resp.status)
	}
}

func TestLogoutClearingCookieKeepsSecureBehindHTTPS(t *testing.T) {
	s := startAuthServer(t, loopbackNetworks)
	resp := s.do(t, authRequest{method: http.MethodPost, path: routeLogout, headers: withOrigin(map[string]string{"X-Forwarded-Proto": "https"})})
	cookie, line := sessionCookieOf(t, resp)
	if cookie == nil || !cookie.Secure {
		t.Fatalf("clearing cookie behind HTTPS is not Secure: %s", line)
	}
}

// CRITERION 8.
func TestMutationsRequireAConfiguredOrigin(t *testing.T) {
	s := startAuthServer(t, nil)
	token := s.mustLogin(t, "alice", routerAlicePassword)
	refused := map[string]map[string]string{
		"no Origin or Referer":        {},
		"foreign Origin":              {"Origin": "https://evil.example"},
		"null Origin":                 {"Origin": "null"},
		"lookalike Origin":            {"Origin": "https://tickets.example.com.evil.example"},
		"wrong scheme":                {"Origin": "http://tickets.example.com"},
		"wrong port":                  {"Origin": "http://192.0.2.10:8080"},
		"foreign Referer":             {"Referer": "https://evil.example/https://tickets.example.com/"},
		"foreign Origin, own Referer": {"Origin": "https://evil.example", "Referer": routerOrigin + "/login"},
	}
	for name, headers := range refused {
		for _, path := range []string{routeLogout, routeLogin, "/api/v1/anything"} {
			resp := s.do(t, authRequest{method: http.MethodPost, path: path, headers: headers, token: token, body: loginBody("alice", routerAlicePassword)})
			if resp.status != http.StatusForbidden {
				t.Errorf("%s: POST %s = %d, want 403", name, path, resp.status)
				continue
			}
			assertErrorShape(t, resp)
			if cookie, _ := sessionCookieOf(t, resp); cookie != nil {
				t.Errorf("%s: POST %s reached the handler and set a cookie", name, path)
			}
		}
		for _, method := range []string{http.MethodPut, http.MethodPatch, http.MethodDelete} {
			if resp := s.do(t, authRequest{method: method, path: "/api/v1/anything", headers: headers, token: token}); resp.status != http.StatusForbidden {
				t.Errorf("%s: %s = %d, want 403", name, method, resp.status)
			}
		}
	}
	if resp := s.do(t, authRequest{method: http.MethodGet, path: routeMe, token: token}); resp.status != http.StatusOK {
		t.Fatalf("GET without Origin = %d, want 200: GETs are not origin-checked", resp.status)
	}
	if resp := s.do(t, authRequest{method: http.MethodGet, path: routeMe, token: token, headers: map[string]string{"Origin": "https://evil.example"}}); resp.status != http.StatusOK {
		t.Fatalf("GET with a foreign Origin = %d, want 200", resp.status)
	}
	accepted := map[string]map[string]string{
		"canonical Origin":   {"Origin": routerOrigin},
		"LAN Origin":         {"Origin": routerLANOrigin},
		"Origin in caps":     {"Origin": "HTTPS://TICKETS.EXAMPLE.COM"},
		"Referer, no Origin": {"Referer": routerLANOrigin + "/login?x=1"},
	}
	for name, headers := range accepted {
		resp := s.do(t, authRequest{method: http.MethodPost, path: routeLogin, headers: headers, body: loginBody("alice", routerAlicePassword)})
		if resp.status != http.StatusOK {
			t.Errorf("%s: login = %d %s, want 200", name, resp.status, resp.body)
		}
	}
}

func TestOnlyThePublicSurfaceAnswersWithoutASession(t *testing.T) {
	s := startAuthServer(t, nil)
	for _, r := range []authRequest{
		{method: http.MethodGet, path: routeMe},
		{method: http.MethodGet, path: routeMe, token: "garbage"},
		{method: http.MethodGet, path: "/api/v1/user/1/watchlist"},
		{method: http.MethodGet, path: "/no-such-route"},
		{method: http.MethodPost, path: "/api/v1/settings/public", headers: withOrigin(nil)},
		{method: http.MethodGet, path: routeLogin},
	} {
		resp := s.do(t, r)
		if resp.status != http.StatusUnauthorized {
			t.Errorf("%s %s (token %q) = %d, want 401", r.method, r.path, r.token, resp.status)
			continue
		}
		assertErrorShape(t, resp)
	}
	// The public routes pass the gate; nothing is mounted there yet, so 404.
	for _, path := range []string{"/api/v1/settings/public", "/api/v1/backdrops"} {
		if resp := s.do(t, authRequest{method: http.MethodGet, path: path}); resp.status != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404 past the gate", path, resp.status)
		}
	}
	token := s.mustLogin(t, "alice", routerAlicePassword)
	resp := s.do(t, authRequest{method: http.MethodGet, path: "/api/v1/no-such-route", token: token})
	if resp.status != http.StatusNotFound {
		t.Fatalf("an unknown route with a session = %d, want 404", resp.status)
	}
	assertErrorShape(t, resp)
}

func TestAuthMeAnswersTheSessionsAccount(t *testing.T) {
	s := startAuthServer(t, nil)
	users := s.service.Users()
	for i, login := range [][2]string{{"alice", routerAlicePassword}, {"bob", routerBobPassword}} {
		token := s.mustLogin(t, login[0], login[1])
		resp := s.do(t, authRequest{method: http.MethodGet, path: routeMe, token: token})
		var got userBody
		if err := json.Unmarshal(resp.body, &got); err != nil {
			t.Fatal(err)
		}
		want := userBody{ID: users[i].ID, Username: users[i].Username, Email: users[i].Email, DisplayName: users[i].DisplayName}
		if resp.status != http.StatusOK || got != want {
			t.Fatalf("auth/me as %s = %d %+v, want %+v", login[0], resp.status, got, want)
		}
		if resp.header.Get("Cache-Control") != "no-store" {
			t.Errorf("auth/me is cacheable: %q", resp.header.Get("Cache-Control"))
		}
	}
}

// Security: the token before a login and the token after it differ, and the
// one before stops working.
func TestLoginOverAnExistingSessionIssuesANewToken(t *testing.T) {
	s := startAuthServer(t, nil)
	before := s.mustLogin(t, "alice", routerAlicePassword)
	resp := s.do(t, authRequest{method: http.MethodPost, path: routeLogin, body: loginBody("alice", routerAlicePassword), headers: withOrigin(nil), token: before})
	after, _ := sessionCookieOf(t, resp)
	if resp.status != http.StatusOK || after == nil || after.Value == before {
		t.Fatalf("login over a session = %d, cookie %v: the token did not change", resp.status, after)
	}
	if resp := s.do(t, authRequest{method: http.MethodGet, path: routeMe, token: before}); resp.status != http.StatusUnauthorized {
		t.Fatalf("the pre-login token = %d, want 401", resp.status)
	}
}

func TestAuthRoutesRefuseOtherMethods(t *testing.T) {
	s := startAuthServer(t, nil)
	token := s.mustLogin(t, "alice", routerAlicePassword)
	for _, r := range []authRequest{
		{method: http.MethodPut, path: routeLogout, headers: withOrigin(nil), token: token},
		{method: http.MethodPost, path: routeMe, headers: withOrigin(nil), token: token},
	} {
		if resp := s.do(t, r); resp.status != http.StatusMethodNotAllowed {
			t.Errorf("%s %s = %d, want 405", r.method, r.path, resp.status)
		}
	}
}

// stubCatalogRoutes stands in for a catalog route family: catalog itself
// registers no route yet.
type stubCatalogRoutes struct{}

const stubCatalogPath = "/api/v1/stub-catalog"

func (stubCatalogRoutes) Register(mux *http.ServeMux) {
	mux.HandleFunc(stubCatalogPath, func(w http.ResponseWriter, _ *http.Request) {
		writeJSONBody(w, map[string]string{"stub": "catalog"})
	})
}

func TestCatalogRoutesAreBehindTheSessionGateAndTheOriginCheck(t *testing.T) {
	s := startAuthServerWith(t, nil, func(deps RouterDeps) http.Handler {
		return newRouter(deps, stubCatalogRoutes{})
	})
	if resp := s.do(t, authRequest{method: http.MethodGet, path: stubCatalogPath}); resp.status != http.StatusUnauthorized {
		t.Errorf("without a session = %d %s, want 401", resp.status, resp.body)
	}
	token := s.mustLogin(t, "alice", routerAlicePassword)
	resp := s.do(t, authRequest{method: http.MethodGet, path: stubCatalogPath, token: token})
	if resp.status != http.StatusOK || string(resp.body) != `{"stub":"catalog"}`+"\n" {
		t.Errorf("with a session = %d %q, want the handler's answer", resp.status, resp.body)
	}
	foreign := s.do(t, authRequest{method: http.MethodPost, path: stubCatalogPath, token: token, headers: map[string]string{"Origin": "https://elsewhere.example"}})
	if foreign.status != http.StatusForbidden {
		t.Errorf("mutation from a foreign origin = %d, want 403", foreign.status)
	}
}
