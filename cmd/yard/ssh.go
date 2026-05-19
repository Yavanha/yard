package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"yard/internal/registry"
	yardruntime "yard/internal/runtime"
	"yard/internal/sshkeys"
)

type sshKeyLister interface {
	List() ([]sshkeys.KeyCandidate, error)
}

type remoteHostKeyScanner interface {
	Scan(host string, port int) ([]string, error)
}

type execRemoteHostKeyScanner struct {
	runner yardruntime.Runner
}

func runSSH(parsed args) error {
	switch parsed.subcommand {
	case "keys":
		return runSSHKeys(parsed)
	case "host-key":
		return runSSHHostKey(parsed)
	default:
		if parsed.subcommand == "" {
			return errors.New("ssh requires a subcommand: keys or host-key")
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

func runSSHHostKey(parsed args) error {
	return runSSHHostKeyWithScanner(parsed, execRemoteHostKeyScanner{}, os.Stdout)
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

func runSSHHostKeyWithScanner(parsed args, scanner remoteHostKeyScanner, output io.Writer) error {
	if len(parsed.positionals) != 1 {
		return errors.New("usage: ssh host-key <host> [--port <port>]")
	}
	port := parsed.servicePort
	if port == 0 {
		port = registry.DefaultRemotePort
	}
	fingerprints, err := scanner.Scan(parsed.positionals[0], port)
	if err != nil {
		return err
	}
	return writeRemoteHostKeyRows(output, parsed.positionals[0], port, fingerprints)
}

func (scanner execRemoteHostKeyScanner) Scan(host string, port int) ([]string, error) {
	runner := scanner.runner
	if runner == nil {
		runner = yardruntime.ExecRunner{Stderr: io.Discard}
	}
	remote := registry.RemoteServer{Host: host, Port: port}
	output, err := runner.Output("ssh-keyscan", yardruntime.RemoteHostKeyScanArgs(remote)...)
	if err != nil {
		return nil, fmt.Errorf("scan remote host key: %w", err)
	}
	return yardruntime.RemoteHostKeyFingerprints(output)
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

func writeRemoteHostKeyRows(output io.Writer, host string, port int, fingerprints []string) error {
	writer := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "HOST\tPORT\tFINGERPRINT")
	for _, fingerprint := range fingerprints {
		fmt.Fprintf(writer, "%s\t%d\t%s\n", host, port, fingerprint)
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
