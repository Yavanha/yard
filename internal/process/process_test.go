package process

import (
	"strings"
	"testing"

	"yard/internal/config"
)

func TestStartCommandBuildsGuardedScript(t *testing.T) {
	t.Parallel()

	command, err := StartCommand(Service{
		ProjectName: "api",
		Name:        "web",
		RepoDir:     "/home/ubuntu/workspaces/app",
		Workdir:     ".",
		Command:     "pnpm dev --host 0.0.0.0",
	})
	if err != nil {
		t.Fatalf("StartCommand returned error: %v", err)
	}

	if len(command) != 3 {
		t.Fatalf("expected shell command, got %#v", command)
	}
	script := command[2]
	for _, expected := range []string{
		"kill -0 \"$pid\"",
		"already_running",
		"nohup",
		"pid=\"$!\"",
		"$HOME/.yard/processes/api/web",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("expected script to contain %q:\n%s", expected, script)
		}
	}
}

func TestStartCommandRequiresRepoDir(t *testing.T) {
	t.Parallel()

	_, err := StartCommand(Service{
		ProjectName: "api",
		Name:        "web",
		Workdir:     ".",
		Command:     "pnpm dev",
	})
	if err == nil {
		t.Fatal("expected missing repo_dir to fail")
	}
}

func TestParseStatusOutput(t *testing.T) {
	t.Parallel()

	state := ParseStatusOutput("api", config.ServiceConfig{
		Name:    "web",
		Command: "pnpm dev",
		Workdir: ".",
		Port:    3000,
	}, []byte("state=running\npid=1234\nlog=/tmp/log\n"))

	assertEqual(t, state.Project, "api")
	assertEqual(t, state.Service, "web")
	assertEqual(t, state.Status, "running")
	assertEqual(t, state.PID, "1234")
	assertEqual(t, state.Port, 3000)
	assertEqual(t, state.Log, "/tmp/log")
}

func TestStopCommandRejectsUnsafeNames(t *testing.T) {
	t.Parallel()

	_, err := StopCommand("api", "../web")
	if err == nil {
		t.Fatal("expected unsafe service name to fail")
	}
}

func assertEqual(t *testing.T, got any, want any) {
	t.Helper()
	if got != want {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}
