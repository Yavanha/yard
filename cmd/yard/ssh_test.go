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
