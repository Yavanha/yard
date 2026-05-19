package runtime

import (
	"reflect"
	"testing"

	"yard/internal/registry"
)

func TestRemoteSSHArgs(t *testing.T) {
	t.Parallel()

	got := RemoteSSHArgs(registry.RemoteServer{
		Host:         "dev.example.com",
		User:         "ubuntu",
		Port:         2222,
		IdentityFile: "/tmp/ssh/remote",
	}, []string{"printf", "ok"})

	want := []string{
		"-p",
		"2222",
		"-o",
		"BatchMode=yes",
		"-o",
		"ForwardAgent=yes",
		"-o",
		"ControlMaster=no",
		"-o",
		"StrictHostKeyChecking=accept-new",
		"-o",
		"ServerAliveInterval=30",
		"-o",
		"ConnectTimeout=5",
		"-i",
		"/tmp/ssh/remote",
		"ubuntu@dev.example.com",
		"--",
		"printf",
		"ok",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestRemoteSSHUsesDefaultPort(t *testing.T) {
	t.Parallel()

	got := RemoteSSHArgs(registry.RemoteServer{
		Host: "dev.example.com",
		User: "ubuntu",
	}, []string{"true"})

	if got[1] != "22" {
		t.Fatalf("expected default port 22, got %#v", got)
	}
}

func TestRemoteSSHExecUsesSSH(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	target := NewRemoteSSH(runner, registry.RemoteServer{
		Host: "dev.example.com",
		User: "ubuntu",
		Port: 2222,
	})

	if err := target.Exec([]string{"true"}); err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if len(runner.runs) != 1 {
		t.Fatalf("expected one run, got %#v", runner.runs)
	}
	if got, want := runner.runs[0][0], "ssh"; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
	if got, want := runner.runs[0][len(runner.runs[0])-1], "true"; got != want {
		t.Fatalf("expected command suffix %q, got %#v", want, runner.runs[0])
	}
}

func TestRemoteSSHExecChecksHostKeyFingerprint(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		outputs: map[string][]byte{
			"ssh-keyscan -p 2222 -T 5 dev.example.com": []byte("dev.example.com ssh-ed25519 YWJj\n"),
		},
	}
	target := NewRemoteSSH(runner, registry.RemoteServer{
		Host:               "dev.example.com",
		User:               "ubuntu",
		Port:               2222,
		HostKeyFingerprint: "SHA256:ungWv48Bz+pBQUDeXa4iI7ADYaOWF3qctBD/YfIAFa0",
	})

	if err := target.Exec([]string{"true"}); err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if len(runner.runs) != 1 {
		t.Fatalf("expected one ssh run, got %#v", runner.runs)
	}
}

func TestRemoteSSHExecRejectsHostKeyMismatch(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		outputs: map[string][]byte{
			"ssh-keyscan -p 2222 -T 5 dev.example.com": []byte("dev.example.com ssh-ed25519 YWJj\n"),
		},
	}
	target := NewRemoteSSH(runner, registry.RemoteServer{
		Host:               "dev.example.com",
		User:               "ubuntu",
		Port:               2222,
		HostKeyFingerprint: "SHA256:expected",
	})

	err := target.Exec([]string{"true"})
	if err == nil {
		t.Fatal("expected host key mismatch to fail")
	}
	if len(runner.runs) != 0 {
		t.Fatalf("expected ssh not to run, got %#v", runner.runs)
	}
}

func TestRemoteHostKeyFingerprints(t *testing.T) {
	t.Parallel()

	fingerprints, err := RemoteHostKeyFingerprints([]byte("# comment\ndev.example.com ssh-ed25519 YWJj\n"))
	if err != nil {
		t.Fatalf("RemoteHostKeyFingerprints returned error: %v", err)
	}
	if got, want := fingerprints[0], "SHA256:ungWv48Bz+pBQUDeXa4iI7ADYaOWF3qctBD/YfIAFa0"; got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRemoteSSHValidatesHost(t *testing.T) {
	t.Parallel()

	target := NewRemoteSSH(&fakeRunner{}, registry.RemoteServer{User: "ubuntu"})
	if err := target.Exec([]string{"true"}); err == nil {
		t.Fatal("expected missing host to fail")
	}
}
