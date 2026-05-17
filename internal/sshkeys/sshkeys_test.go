package sshkeys

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseIdentityList(t *testing.T) {
	t.Parallel()

	keys := parseIdentityList([]byte(`256 SHA256:agent1 api@example.com (ED25519)
2048 SHA256:agent2 /Users/me/.ssh/id_rsa (RSA)
The agent has no identities.
`))

	want := []agentKey{
		{Fingerprint: "SHA256:agent1", Comment: "api@example.com"},
		{Fingerprint: "SHA256:agent2", Comment: "/Users/me/.ssh/id_rsa"},
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("expected %#v, got %#v", want, keys)
	}
}

func TestListMergesAgentAndPublicKeys(t *testing.T) {
	t.Parallel()

	sshDir := filepath.Join(t.TempDir(), ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	apiPublicKey := filepath.Join(sshDir, "id_ed25519.pub")
	workerPublicKey := filepath.Join(sshDir, "worker.pub")
	if err := os.WriteFile(apiPublicKey, []byte("ssh-ed25519 AAAA api@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(workerPublicKey, []byte("ssh-ed25519 BBBB worker@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunner{
		outputs: map[string]fakeOutput{
			"ssh-add -l": {
				content: []byte("256 SHA256:api agent-api-comment (ED25519)\n256 SHA256:agent-only loaded-only (ED25519)\n"),
			},
			"ssh-keygen -lf " + apiPublicKey: {
				content: []byte("256 SHA256:api ignored (ED25519)\n"),
			},
			"ssh-keygen -lf " + workerPublicKey: {
				content: []byte("256 SHA256:worker ignored (ED25519)\n"),
			},
		},
	}

	keys, err := NewDetector(runner, sshDir).List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	want := []KeyCandidate{
		{
			Path:        strings.TrimSuffix(apiPublicKey, ".pub"),
			Comment:     "api@example.com",
			Fingerprint: "SHA256:api",
			InAgent:     true,
		},
		{
			Path:        strings.TrimSuffix(workerPublicKey, ".pub"),
			Comment:     "worker@example.com",
			Fingerprint: "SHA256:worker",
		},
		{
			Comment:     "loaded-only",
			Fingerprint: "SHA256:agent-only",
			InAgent:     true,
		},
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("expected %#v, got %#v", want, keys)
	}
}

func TestListContinuesWhenAgentIsUnavailable(t *testing.T) {
	t.Parallel()

	sshDir := filepath.Join(t.TempDir(), ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	publicKey := filepath.Join(sshDir, "id_ed25519.pub")
	if err := os.WriteFile(publicKey, []byte("ssh-ed25519 AAAA api@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunner{
		outputs: map[string]fakeOutput{
			"ssh-add -l": {
				err: errors.New("agent unavailable"),
			},
			"ssh-keygen -lf " + publicKey: {
				content: []byte("256 SHA256:api api@example.com (ED25519)\n"),
			},
		},
	}

	keys, err := NewDetector(runner, sshDir).List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected one key, got %#v", keys)
	}
	assertEqual(t, keys[0].Fingerprint, "SHA256:api")
	assertEqual(t, keys[0].InAgent, false)
}

func TestListReturnsFingerprintError(t *testing.T) {
	t.Parallel()

	sshDir := filepath.Join(t.TempDir(), ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	publicKey := filepath.Join(sshDir, "broken.pub")
	if err := os.WriteFile(publicKey, []byte("broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunner{
		outputs: map[string]fakeOutput{
			"ssh-add -l": {
				content: []byte("The agent has no identities.\n"),
			},
			"ssh-keygen -lf " + publicKey: {
				err: errors.New("invalid key"),
			},
		},
	}

	_, err := NewDetector(runner, sshDir).List()
	if err == nil {
		t.Fatal("expected fingerprint error")
	}
}

func TestFingerprintForIdentityUsesPublicKey(t *testing.T) {
	t.Parallel()

	sshDir := filepath.Join(t.TempDir(), ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatal(err)
	}

	publicKey := filepath.Join(sshDir, "id_ed25519.pub")
	if err := os.WriteFile(publicKey, []byte("ssh-ed25519 AAAA api@example.com\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	runner := &fakeRunner{
		outputs: map[string]fakeOutput{
			"ssh-keygen -lf " + publicKey: {
				content: []byte("256 SHA256:api api@example.com (ED25519)\n"),
			},
		},
	}

	fingerprint, err := NewDetector(runner, sshDir).FingerprintForIdentity(strings.TrimSuffix(publicKey, ".pub"))
	if err != nil {
		t.Fatalf("FingerprintForIdentity returned error: %v", err)
	}
	assertEqual(t, fingerprint, "SHA256:api")
}

type fakeRunner struct {
	outputs map[string]fakeOutput
}

type fakeOutput struct {
	content []byte
	err     error
}

func (r *fakeRunner) Output(command string, args ...string) ([]byte, error) {
	key := strings.Join(append([]string{command}, args...), " ")
	output, ok := r.outputs[key]
	if !ok {
		return nil, errors.New("unexpected command: " + key)
	}
	return output.content, output.err
}

func assertEqual(t *testing.T, got any, want any) {
	t.Helper()
	if got != want {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}
