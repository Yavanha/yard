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

func TestRemoteSSHValidatesHost(t *testing.T) {
	t.Parallel()

	target := NewRemoteSSH(&fakeRunner{}, registry.RemoteServer{User: "ubuntu"})
	if err := target.Exec([]string{"true"}); err == nil {
		t.Fatal("expected missing host to fail")
	}
}
