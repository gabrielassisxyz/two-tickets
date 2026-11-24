package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"net/url"
	"regexp"
	"strings"
)

// Account variables, one set per slot: ACCOUNT_1_USERNAME, ACCOUNT_2_USERNAME...
// Every one of them may instead be read from a mounted file through NAME_FILE.
// Build a full name with AccountEnvName.
const (
	AccountKeySuffix          = "_KEY"
	AccountUsernameSuffix     = "_USERNAME"
	AccountEmailSuffix        = "_EMAIL"
	AccountDisplayNameSuffix  = "_DISPLAY_NAME"
	AccountPasswordHashSuffix = "_PASSWORD_HASH"

	EnvAllowedOrigins    = "ALLOWED_ORIGINS"
	EnvTrustedProxyCIDRs = "TRUSTED_PROXY_CIDRS"

	envAccountPrefix          = "ACCOUNT_"
	accountSlotCount          = 2
	maxAccountIdentifierBytes = 254
)

var (
	accountKeyPattern      = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)
	accountUsernamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
)

// Account is one of the two fixed logins. Username and Email are already
// normalised (trimmed, lower-case), the form a login identifier is compared in.
type Account struct {
	// ConfigKey ties the account to its users row across restarts; the
	// numeric id follows it, so it must never change once data exists.
	ConfigKey    string
	Username     string
	Email        string
	DisplayName  string
	PasswordHash Secret
}

// Auth is the configuration of login and of the request checks around it.
type Auth struct {
	Accounts [accountSlotCount]Account
	// AllowedOrigins are scheme://host[:port] values, normalised the way
	// NormalizeOrigin does, that a mutation's Origin or Referer must match.
	AllowedOrigins []string
	// TrustedProxies are the networks whose Forwarded and X-Forwarded-*
	// headers are believed. Empty means no proxy is trusted.
	TrustedProxies []netip.Prefix
}

// AccountEnvName returns the variable for one field of an account slot
// (1-based): AccountEnvName(1, AccountUsernameSuffix) is ACCOUNT_1_USERNAME.
func AccountEnvName(slot int, suffix string) string {
	return fmt.Sprintf("%s%d%s", envAccountPrefix, slot, suffix)
}

// LoadAuth reads and validates the two accounts, the allowed origins and the
// trusted proxy networks. The password hash is only checked for presence
// here; its argon2id parameters are checked by the auth package at startup.
func LoadAuth(lookup LookupEnv) (Auth, error) {
	var cfg Auth
	var problems []error
	for i := range cfg.Accounts {
		account, err := loadAccount(lookup, i+1)
		if err != nil {
			problems = append(problems, err)
		}
		cfg.Accounts[i] = account
	}
	if len(problems) == 0 {
		if err := requireDistinctAccounts(cfg.Accounts); err != nil {
			problems = append(problems, err)
		}
	}

	origins, err := loadAllowedOrigins(lookup)
	if err != nil {
		problems = append(problems, err)
	}
	cfg.AllowedOrigins = origins

	proxies, err := loadTrustedProxies(lookup)
	if err != nil {
		problems = append(problems, err)
	}
	cfg.TrustedProxies = proxies

	if len(problems) > 0 {
		return Auth{}, errors.Join(problems...)
	}
	return cfg, nil
}

// LogValue names the accounts by key only and never prints a hash.
func (a Auth) LogValue() slog.Value {
	keys := make([]string, 0, len(a.Accounts))
	for _, account := range a.Accounts {
		keys = append(keys, account.ConfigKey)
	}
	proxies := make([]string, 0, len(a.TrustedProxies))
	for _, p := range a.TrustedProxies {
		proxies = append(proxies, p.String())
	}
	return slog.GroupValue(
		slog.Any("account_keys", keys),
		slog.Any("allowed_origins", a.AllowedOrigins),
		slog.Any("trusted_proxies", proxies),
	)
}

// accountField reads one field of a slot; a missing one is reported by name.
func accountField(lookup LookupEnv, slot int, suffix string, missing *[]string) (string, error) {
	name := AccountEnvName(slot, suffix)
	v, err := secretValue(lookup, name)
	if err != nil {
		return "", err
	}
	if v == "" {
		*missing = append(*missing, name)
	}
	return v, nil
}

func loadAccount(lookup LookupEnv, slot int) (Account, error) {
	var missing []string
	var problems []error
	read := func(suffix string) string {
		v, err := accountField(lookup, slot, suffix, &missing)
		if err != nil {
			problems = append(problems, err)
		}
		return v
	}
	account := Account{
		ConfigKey:    read(AccountKeySuffix),
		Username:     NormalizeAccountIdentifier(read(AccountUsernameSuffix)),
		Email:        NormalizeAccountIdentifier(read(AccountEmailSuffix)),
		DisplayName:  read(AccountDisplayNameSuffix),
		PasswordHash: NewSecret(read(AccountPasswordHashSuffix)),
	}
	if len(missing) > 0 {
		problems = append([]error{fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))}, problems...)
	}
	if len(problems) > 0 {
		return Account{}, errors.Join(problems...)
	}
	if err := validateAccount(slot, account); err != nil {
		return Account{}, err
	}
	return account, nil
}

func validateAccount(slot int, a Account) error {
	var problems []error
	if !accountKeyPattern.MatchString(a.ConfigKey) {
		problems = append(problems, fmt.Errorf("%s must match %s", AccountEnvName(slot, AccountKeySuffix), accountKeyPattern))
	}
	// A username can hold no "@", so it can never equal an email and one
	// identifier can never name two accounts.
	if !accountUsernamePattern.MatchString(a.Username) {
		problems = append(problems, fmt.Errorf("%s must match %s once lower-cased", AccountEnvName(slot, AccountUsernameSuffix), accountUsernamePattern))
	}
	if !plausibleEmail(a.Email) {
		problems = append(problems, fmt.Errorf("%s is not an email address", AccountEnvName(slot, AccountEmailSuffix)))
	}
	return errors.Join(problems...)
}

func plausibleEmail(email string) bool {
	local, domain, found := strings.Cut(email, "@")
	return found && local != "" && domain != "" && !strings.Contains(domain, "@") &&
		!strings.ContainsAny(email, " \t\r\n") && len(email) <= maxAccountIdentifierBytes
}

func requireDistinctAccounts(accounts [accountSlotCount]Account) error {
	a, b := accounts[0], accounts[1]
	var problems []error
	if a.ConfigKey == b.ConfigKey {
		problems = append(problems, errors.New("both accounts have the same config key"))
	}
	if a.Username == b.Username {
		problems = append(problems, errors.New("both accounts have the same username"))
	}
	if a.Email == b.Email {
		problems = append(problems, errors.New("both accounts have the same email"))
	}
	return errors.Join(problems...)
}

// NormalizeAccountIdentifier is the one form a username or email is stored
// and compared in, so " Alice@Example.com " and "alice@example.com" are the
// same login.
func NormalizeAccountIdentifier(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func loadAllowedOrigins(lookup LookupEnv) ([]string, error) {
	raw := envValue(lookup, EnvAllowedOrigins)
	if raw == "" {
		return nil, fmt.Errorf("missing required configuration: %s", EnvAllowedOrigins)
	}
	var origins []string
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		origin, err := NormalizeOrigin(item)
		if err != nil {
			return nil, fmt.Errorf("%s: %q: %w", EnvAllowedOrigins, item, err)
		}
		origins = append(origins, origin)
	}
	if len(origins) == 0 {
		return nil, fmt.Errorf("missing required configuration: %s", EnvAllowedOrigins)
	}
	return origins, nil
}

// NormalizeOrigin reduces an origin or a URL to the scheme://host[:port] a
// browser sends in Origin: lower-case, with the scheme's default port dropped.
// Anything carrying a path, query, fragment or credentials is refused, so a
// configured value cannot silently mean less than it says.
func NormalizeOrigin(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("an origin is scheme://host[:port] with nothing after it")
	}
	return originOf(u)
}

// OriginOfURL reduces a full URL, such as a Referer, to its origin.
func OriginOfURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	return originOf(u)
}

func originOf(u *url.URL) (string, error) {
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return "", errors.New("the scheme must be http or https")
	}
	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", errors.New("the host is empty")
	}
	port := u.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port != "" {
		return scheme + "://" + host + ":" + port, nil
	}
	return scheme + "://" + host, nil
}

func loadTrustedProxies(lookup LookupEnv) ([]netip.Prefix, error) {
	raw := envValue(lookup, EnvTrustedProxyCIDRs)
	var prefixes []netip.Prefix
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(item)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", EnvTrustedProxyCIDRs, err)
		}
		prefixes = append(prefixes, prefix.Masked())
	}
	return prefixes, nil
}
