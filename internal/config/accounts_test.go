package config

import (
	"bytes"
	"log/slog"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The hash is only checked for presence here, so any non-empty text will do;
// its argon2id parameters are the auth package's test.
const placeholderPasswordHash = "placeholderplaceholder"

func validAuthEnv() map[string]string {
	return map[string]string{
		AccountEnvName(1, AccountKeySuffix):          "first",
		AccountEnvName(1, AccountUsernameSuffix):     "alice",
		AccountEnvName(1, AccountEmailSuffix):        "alice@example.com",
		AccountEnvName(1, AccountDisplayNameSuffix):  "Alice",
		AccountEnvName(1, AccountPasswordHashSuffix): placeholderPasswordHash,
		AccountEnvName(2, AccountKeySuffix):          "second",
		AccountEnvName(2, AccountUsernameSuffix):     "bob",
		AccountEnvName(2, AccountEmailSuffix):        "bob@example.com",
		AccountEnvName(2, AccountDisplayNameSuffix):  "Bob",
		AccountEnvName(2, AccountPasswordHashSuffix): placeholderPasswordHash,
		EnvAllowedOrigins:                            "https://tickets.example.com, http://192.0.2.10:30080,http://192.0.2.11:30080",
		EnvTrustedProxyCIDRs:                         "10.42.0.0/16, fd00::/8",
	}
}

func TestLoadAuthReadsBothAccountsOriginsAndProxies(t *testing.T) {
	cfg, err := LoadAuth(fakeEnv(validAuthEnv()))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Accounts[0].ConfigKey != "first" || cfg.Accounts[1].Username != "bob" || cfg.Accounts[1].DisplayName != "Bob" {
		t.Fatalf("accounts = %+v", cfg.Accounts)
	}
	wantOrigins := []string{"https://tickets.example.com", "http://192.0.2.10:30080", "http://192.0.2.11:30080"}
	if strings.Join(cfg.AllowedOrigins, " ") != strings.Join(wantOrigins, " ") {
		t.Fatalf("origins = %v, want %v", cfg.AllowedOrigins, wantOrigins)
	}
	if len(cfg.TrustedProxies) != 2 || !cfg.TrustedProxies[0].Contains(netip.MustParseAddr("10.42.3.4")) {
		t.Fatalf("proxies = %v", cfg.TrustedProxies)
	}
}

// CRITERION 2: the configured identifiers are stored in the form a login is
// compared in.
func TestLoadAuthNormalisesUsernamesAndEmails(t *testing.T) {
	env := validAuthEnv()
	env[AccountEnvName(1, AccountUsernameSuffix)] = "  Alice "
	env[AccountEnvName(1, AccountEmailSuffix)] = " Alice@Example.COM"
	cfg, err := LoadAuth(fakeEnv(env))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Accounts[0].Username != "alice" || cfg.Accounts[0].Email != "alice@example.com" {
		t.Fatalf("normalised to %q %q", cfg.Accounts[0].Username, cfg.Accounts[0].Email)
	}
}

// CRITERION 2: one username or one email on both accounts fails startup,
// including when they differ only in case or spaces.
func TestLoadAuthRefusesAUsernameOrEmailOnBothAccounts(t *testing.T) {
	cases := map[string]struct {
		suffix, second, wantText string
	}{
		"username":         {AccountUsernameSuffix, "alice", "same username"},
		"username in case": {AccountUsernameSuffix, " ALICE ", "same username"},
		"email":            {AccountEmailSuffix, "alice@example.com", "same email"},
		"email in case":    {AccountEmailSuffix, "Alice@EXAMPLE.com ", "same email"},
		"config key":       {AccountKeySuffix, "first", "same config key"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			env := validAuthEnv()
			env[AccountEnvName(2, tc.suffix)] = tc.second
			_, err := LoadAuth(fakeEnv(env))
			if err == nil || !strings.Contains(err.Error(), tc.wantText) {
				t.Fatalf("error = %v, want one containing %q", err, tc.wantText)
			}
		})
	}
}

func TestLoadAuthNamesEveryMissingVariable(t *testing.T) {
	for _, name := range []string{
		AccountEnvName(1, AccountKeySuffix), AccountEnvName(1, AccountPasswordHashSuffix),
		AccountEnvName(2, AccountUsernameSuffix), AccountEnvName(2, AccountEmailSuffix),
		AccountEnvName(2, AccountDisplayNameSuffix), EnvAllowedOrigins,
	} {
		t.Run(name, func(t *testing.T) {
			env := validAuthEnv()
			delete(env, name)
			_, err := LoadAuth(fakeEnv(env))
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("error = %v, want one naming %s", err, name)
			}
		})
	}
	_, err := LoadAuth(fakeEnv(map[string]string{}))
	if err == nil {
		t.Fatal("an empty environment loaded")
	}
	for slot := 1; slot <= 2; slot++ {
		if !strings.Contains(err.Error(), AccountEnvName(slot, AccountPasswordHashSuffix)) {
			t.Errorf("the empty-environment error does not name slot %d: %v", slot, err)
		}
	}
}

func TestLoadAuthRefusesMalformedValues(t *testing.T) {
	cases := map[string]struct{ name, value string }{
		"username with @":     {AccountEnvName(1, AccountUsernameSuffix), "al@ice"},
		"username with colon": {AccountEnvName(1, AccountUsernameSuffix), "pending:first"},
		"username with space": {AccountEnvName(1, AccountUsernameSuffix), "al ice"},
		"email without @":     {AccountEnvName(2, AccountEmailSuffix), "bob.example.com"},
		"email with two @":    {AccountEnvName(2, AccountEmailSuffix), "bob@ex@ample.com"},
		"email with space":    {AccountEnvName(2, AccountEmailSuffix), "bob @example.com"},
		"config key upper":    {AccountEnvName(1, AccountKeySuffix), "First"},
		"origin with path":    {EnvAllowedOrigins, "https://tickets.example.com/app"},
		"origin no scheme":    {EnvAllowedOrigins, "tickets.example.com"},
		"origin ftp":          {EnvAllowedOrigins, "ftp://tickets.example.com"},
		"origin only commas":  {EnvAllowedOrigins, " , ,"},
		"proxy not a CIDR":    {EnvTrustedProxyCIDRs, "10.42.0.0"},
	}
	for label, tc := range cases {
		t.Run(label, func(t *testing.T) {
			env := validAuthEnv()
			env[tc.name] = tc.value
			if _, err := LoadAuth(fakeEnv(env)); err == nil {
				t.Fatalf("%s=%q was accepted", tc.name, tc.value)
			}
		})
	}
}

func TestLoadAuthReadsAccountsFromMountedFiles(t *testing.T) {
	dir := t.TempDir()
	hashFile := filepath.Join(dir, "hash")
	if err := os.WriteFile(hashFile, []byte(placeholderPasswordHash+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	env := validAuthEnv()
	delete(env, AccountEnvName(2, AccountPasswordHashSuffix))
	env[AccountEnvName(2, AccountPasswordHashSuffix)+"_FILE"] = hashFile
	cfg, err := LoadAuth(fakeEnv(env))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Accounts[1].PasswordHash.Reveal() != placeholderPasswordHash {
		t.Fatal("the hash file was not read")
	}
}

func TestAuthLogValueNeverPrintsAHash(t *testing.T) {
	cfg, err := LoadAuth(fakeEnv(validAuthEnv()))
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	slog.New(slog.NewJSONHandler(&out, nil)).Info("x", "auth", cfg, "account", cfg.Accounts[0])
	if strings.Contains(out.String(), placeholderPasswordHash) {
		t.Fatalf("the log line prints a hash: %s", out.String())
	}
	if !strings.Contains(out.String(), "first") {
		t.Fatalf("the log line does not name the accounts: %s", out.String())
	}
}

func TestNormalizeOrigin(t *testing.T) {
	cases := map[string]string{
		"https://Tickets.Example.com":     "https://tickets.example.com",
		"https://tickets.example.com:443": "https://tickets.example.com",
		"https://tickets.example.com/":    "https://tickets.example.com",
		"http://192.0.2.10:80":            "http://192.0.2.10",
		"http://192.0.2.10:30080":         "http://192.0.2.10:30080",
		"HTTP://[2001:DB8::1]:8080":       "http://[2001:db8::1]:8080",
	}
	for in, want := range cases {
		got, err := NormalizeOrigin(in)
		if err != nil || got != want {
			t.Errorf("NormalizeOrigin(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"null", "", "https://", "https://user@tickets.example.com", "https://tickets.example.com?x=1", "https://tickets.example.com/#f"} {
		if got, err := NormalizeOrigin(bad); err == nil {
			t.Errorf("NormalizeOrigin(%q) = %q, want an error", bad, got)
		}
	}
	if got, err := OriginOfURL("https://Tickets.example.com:443/login?next=/x"); err != nil || got != "https://tickets.example.com" {
		t.Errorf("OriginOfURL = %q, %v", got, err)
	}
}
