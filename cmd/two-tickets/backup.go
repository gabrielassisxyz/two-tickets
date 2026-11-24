package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/gabrielassisxyz/two-tickets/internal/config"
	"github.com/gabrielassisxyz/two-tickets/internal/store"
)

const commandBackup = "backup"

// runBackup publishes an online copy of the live state.db as the backup
// artifact. The source is always DATA_DIR/state.db; only the target is a
// flag, defaulting to the backup path of the /data volume the api runs with.
func runBackup(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet(commandBackup, flag.ContinueOnError)
	flags.SetOutput(stderr)
	target := flags.String("target", store.DefaultBackupTarget, "where to write the backup artifact")
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return exitUsage
	}
	if flags.NArg() > 0 {
		_, _ = fmt.Fprintf(stderr, "two-tickets backup: unexpected argument %q\n", flags.Arg(0))
		return exitUsage
	}
	dataDir, ok := os.LookupEnv(config.EnvDataDir)
	if !ok || strings.TrimSpace(dataDir) == "" {
		_, _ = fmt.Fprintf(stderr, "two-tickets backup: %s is not set\n", config.EnvDataDir)
		return exitFailure
	}
	if !filepath.IsAbs(dataDir) {
		_, _ = fmt.Fprintf(stderr, "two-tickets backup: %s must be an absolute path\n", config.EnvDataDir)
		return exitFailure
	}
	source := filepath.Join(dataDir, store.StateFileName)
	if err := store.BackupStateDB(context.Background(), source, *target); err != nil {
		_, _ = fmt.Fprintf(stderr, "two-tickets backup: %v\n", err)
		return exitFailure
	}
	return 0
}
