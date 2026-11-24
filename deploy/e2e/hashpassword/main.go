// Command hashpassword prints the argon2id PHC string of the password read
// from stdin, at the minimum cost the api accepts. The end-to-end check uses
// it to configure a throwaway account without committing a hash, and without
// depending on an argon2 binary the CI runner does not have.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gabrielassisxyz/two-tickets/internal/auth"
)

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hashpassword:", err)
		os.Exit(1)
	}
	password := strings.TrimRight(string(raw), "\n")
	if password == "" {
		fmt.Fprintln(os.Stderr, "hashpassword: empty password on stdin")
		os.Exit(1)
	}
	encoded, err := auth.HashPassword(password, auth.MinimumArgon2idParams)
	if err != nil {
		fmt.Fprintln(os.Stderr, "hashpassword:", err)
		os.Exit(1)
	}
	fmt.Println(encoded)
}
