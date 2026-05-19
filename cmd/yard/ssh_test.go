package main

import (
	"bytes"
	"strings"
	"testing"

	"yard/internal/sshkeys"
)

func TestParseSSHKeysArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseArgs([]string{"ssh", "keys"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	assertEqual(t, parsed.command, "ssh")
	assertEqual(t, parsed.subcommand, "keys")
}

func TestParseSSHHostKeyArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseArgs([]string{"ssh", "host-key", "dev.example.com", "--port", "2222"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	assertEqual(t, parsed.command, "ssh")
	assertEqual(t, parsed.subcommand, "host-key")
	assertEqual(t, parsed.positionals[0], "dev.example.com")
	assertEqual(t, parsed.servicePort, 2222)
}

func TestRunSSHKeysWritesTable(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := runSSHKeysWithLister(args{}, fakeSSHKeyLister{
		keys: []sshkeys.KeyCandidate{{
			Path:        "/Users/me/.ssh/id_ed25519",
			Comment:     "api@example.com",
			Fingerprint: "SHA256:abc123",
			InAgent:     true,
		}},
	}, &output)
	if err != nil {
		t.Fatalf("runSSHKeysWithLister returned error: %v", err)
	}

	got := output.String()
	for _, expected := range []string{"AGENT", "FINGERPRINT", "PATH", "COMMENT", "yes", "SHA256:abc123", "/Users/me/.ssh/id_ed25519"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected output to contain %q:\n%s", expected, got)
		}
	}
}

func TestRunSSHKeysRejectsArguments(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := runSSHKeysWithLister(args{positionals: []string{"extra"}}, fakeSSHKeyLister{}, &output)
	if err == nil {
		t.Fatal("expected extra argument to fail")
	}
}

func TestRunSSHHostKeyWritesTable(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := runSSHHostKeyWithScanner(args{
		positionals: []string{"dev.example.com"},
		servicePort: 2222,
	}, &fakeRemoteHostKeyScanner{
		fingerprints: []string{"SHA256:host123"},
	}, &output)
	if err != nil {
		t.Fatalf("runSSHHostKeyWithScanner returned error: %v", err)
	}

	got := output.String()
	for _, expected := range []string{"HOST", "PORT", "FINGERPRINT", "dev.example.com", "2222", "SHA256:host123"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected output to contain %q:\n%s", expected, got)
		}
	}
}

func TestRunSSHHostKeyUsesDefaultPort(t *testing.T) {
	t.Parallel()

	scanner := &fakeRemoteHostKeyScanner{
		fingerprints: []string{"SHA256:host123"},
	}
	err := runSSHHostKeyWithScanner(args{
		positionals: []string{"dev.example.com"},
	}, scanner, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("runSSHHostKeyWithScanner returned error: %v", err)
	}
	assertEqual(t, scanner.port, 22)
}

func TestRunSSHHostKeyRejectsInvalidArgs(t *testing.T) {
	t.Parallel()

	err := runSSHHostKeyWithScanner(args{}, &fakeRemoteHostKeyScanner{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected missing host to fail")
	}
}

type fakeSSHKeyLister struct {
	keys []sshkeys.KeyCandidate
	err  error
}

func (l fakeSSHKeyLister) List() ([]sshkeys.KeyCandidate, error) {
	if l.err != nil {
		return nil, l.err
	}
	return l.keys, nil
}

var _ sshKeyLister = fakeSSHKeyLister{}

type fakeRemoteHostKeyScanner struct {
	host         string
	port         int
	fingerprints []string
	err          error
}

func (scanner *fakeRemoteHostKeyScanner) Scan(host string, port int) ([]string, error) {
	scanner.host = host
	scanner.port = port
	if scanner.err != nil {
		return nil, scanner.err
	}
	return scanner.fingerprints, nil
}

var _ remoteHostKeyScanner = (*fakeRemoteHostKeyScanner)(nil)
