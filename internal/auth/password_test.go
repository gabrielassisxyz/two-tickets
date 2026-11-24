package auth

import (
	"strings"
	"testing"
)

func TestParsePasswordHashRefusesWeakOrForeignHashes(t *testing.T) {
	valid := hashFor(t, alicePassword)
	parts := strings.Split(valid, "$")
	withParams := func(params string) string {
		p := append([]string(nil), parts...)
		p[3] = params
		return strings.Join(p, "$")
	}
	cases := map[string]string{
		"memory below 19 MiB":   withParams("m=19455,t=2,p=1"),
		"one pass":              withParams("m=19456,t=1,p=1"),
		"no lanes":              withParams("m=19456,t=2,p=0"),
		"trailing parameter":    withParams("m=19456,t=2,p=1,x=3"),
		"argon2i":               strings.Replace(valid, "$argon2id$", "$argon2i$", 1),
		"version 16":            strings.Replace(valid, "$v=19$", "$v=16$", 1),
		"short salt":            strings.Join(append(append([]string(nil), parts[:4]...), "c2FsdA", parts[5]), "$"),
		"short key":             strings.Join(append(append([]string(nil), parts[:5]...), "a2V5"), "$"),
		"padded base64 salt":    strings.Join(append(append([]string(nil), parts[:4]...), parts[4]+"==", parts[5]), "$"),
		"bcrypt":                "$2b$12$abcdefghijklmnopqrstuuabcdefghijklmnopqrstuvwxyz01234",
		"plain text":            alicePassword,
		"empty":                 "",
		"missing key component": strings.Join(parts[:5], "$"),
	}
	for name, encoded := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := ParsePasswordHash(encoded); err == nil {
				t.Fatalf("ParsePasswordHash(%q) accepted it", encoded)
			}
		})
	}
	if _, err := ParsePasswordHash(withParams("m=19456,t=2,p=1")); err != nil {
		t.Fatalf("the minimum parameters were refused: %v", err)
	}
}

func TestHashPasswordUsesTheRequestedParametersAndAFreshSalt(t *testing.T) {
	params := Argon2idParams{MemoryKiB: 20 * 1024, Passes: 3, Lanes: 2}
	first, err := HashPassword(alicePassword, params)
	if err != nil {
		t.Fatal(err)
	}
	second, err := HashPassword(alicePassword, params)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("two hashes of one password are identical: the salt is not random")
	}
	parsed, err := ParsePasswordHash(first)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Params != params {
		t.Fatalf("parameters = %+v, want %+v", parsed.Params, params)
	}
	if !strings.HasPrefix(first, "$argon2id$v=19$m=20480,t=3,p=2$") {
		t.Fatalf("encoded = %q", first)
	}
}

func TestVerifyAcceptsOnlyTheHashedPassword(t *testing.T) {
	h, err := ParsePasswordHash(hashFor(t, alicePassword))
	if err != nil {
		t.Fatal(err)
	}
	if !verifyArgon2idPassword(h, alicePassword) {
		t.Fatal("the right password was refused")
	}
	for _, wrong := range []string{"", bobPassword, alicePassword + "x", alicePassword[:len(alicePassword)-1], strings.ToUpper(alicePassword)} {
		if verifyArgon2idPassword(h, wrong) {
			t.Errorf("wrong password %q was accepted", wrong)
		}
	}
}

// The reference argon2 command line tool is what .env.example tells the
// operator to run, so its output must verify here.
func TestHashFromTheArgon2CommandLineVerifies(t *testing.T) {
	// printf %s pw | argon2 saltsaltsaltsalt -id -t 2 -k 19456 -p 1 -e
	fromCLI := "$argon2id$v=19$m=19456,t=2,p=1$c2FsdHNhbHRzYWx0c2FsdA$aOJaI7vUM9/qm6/8sgni9fVmwMtEMUdxlpUqY5SGAGE"
	h, err := ParsePasswordHash(fromCLI)
	if err != nil {
		t.Fatal(err)
	}
	if !verifyArgon2idPassword(h, "pw") || verifyArgon2idPassword(h, "pW") {
		t.Fatal("the argon2 CLI's hash does not verify as expected")
	}
}

func TestDummyHashCostsWhatTheFirstAccountCosts(t *testing.T) {
	svc, _ := newTestService(t, newAuthTestClock())
	if svc.dummyHash.Params != svc.accounts[0].hash.Params {
		t.Fatalf("dummy parameters %+v, account %+v", svc.dummyHash.Params, svc.accounts[0].hash.Params)
	}
	for _, password := range []string{"", alicePassword, bobPassword} {
		if verifyArgon2idPassword(svc.dummyHash, password) {
			t.Fatalf("the dummy hash accepts %q", password)
		}
	}
}
