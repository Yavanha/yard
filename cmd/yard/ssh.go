package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"yard/internal/sshkeys"
)

type sshKeyLister interface {
	List() ([]sshkeys.KeyCandidate, error)
}

func runSSH(parsed args) error {
	switch parsed.subcommand {
	case "keys":
		return runSSHKeys(parsed)
	default:
		if parsed.subcommand == "" {
			return errors.New("ssh requires a subcommand: keys")
		}
		return fmt.Errorf("unknown ssh subcommand: %s", parsed.subcommand)
	}
}

func runSSHKeys(parsed args) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	detector := sshkeys.NewDetector(nil, filepath.Join(home, ".ssh"))
	return runSSHKeysWithLister(parsed, detector, os.Stdout)
}

func runSSHKeysWithLister(parsed args, lister sshKeyLister, output io.Writer) error {
	if len(parsed.positionals) != 0 {
		return errors.New("usage: ssh keys")
	}

	keys, err := lister.List()
	if err != nil {
		return err
	}
	return writeSSHKeyRows(output, keys)
}

func writeSSHKeyRows(output io.Writer, keys []sshkeys.KeyCandidate) error {
	writer := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "AGENT\tFINGERPRINT\tPATH\tCOMMENT")
	for _, key := range keys {
		fmt.Fprintf(
			writer,
			"%s\t%s\t%s\t%s\n",
			formatBool(key.InAgent),
			key.Fingerprint,
			formatEmpty(key.Path),
			formatEmpty(key.Comment),
		)
	}
	return writer.Flush()
}

func formatBool(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func formatEmpty(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
