package gitrepo

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestTestAccessUsesSSHIdentity(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	client := NewClient(runner)

	err := client.TestAccess("git@github.com:acme/api.git", "/Users/me/.ssh/yard acme")
	if err != nil {
		t.Fatalf("TestAccess returned error: %v", err)
	}

	want := fakeCall{
		env:     []string{"GIT_SSH_COMMAND=ssh -i '/Users/me/.ssh/yard acme' -o IdentitiesOnly=yes -o BatchMode=yes -o StrictHostKeyChecking=accept-new"},
		command: "git",
		args:    []string{"ls-remote", "git@github.com:acme/api.git"},
	}
	if !reflect.DeepEqual(runner.outputs[0], want) {
		t.Fatalf("expected %#v, got %#v", want, runner.outputs[0])
	}
}

func TestCloneUsesSSHIdentity(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	client := NewClient(runner)

	err := client.Clone("git@github.com:acme/api.git", "/Users/me/.ssh/yard_acme", "/Users/me/workspaces/api")
	if err != nil {
		t.Fatalf("Clone returned error: %v", err)
	}

	want := fakeCall{
		env:     []string{"GIT_SSH_COMMAND=ssh -i '/Users/me/.ssh/yard_acme' -o IdentitiesOnly=yes -o BatchMode=yes -o StrictHostKeyChecking=accept-new"},
		command: "git",
		args:    []string{"clone", "git@github.com:acme/api.git", "/Users/me/workspaces/api"},
	}
	if !reflect.DeepEqual(runner.runs[0], want) {
		t.Fatalf("expected %#v, got %#v", want, runner.runs[0])
	}
}

func TestTestAccessIncludesGitOutputOnFailure(t *testing.T) {
	t.Parallel()

	client := NewClient(&fakeRunner{
		outputErr: errors.New("exit status 128"),
		output:    []byte("Permission denied (publickey)."),
	})

	err := client.TestAccess("git@github.com:acme/api.git", "/Users/me/.ssh/key")
	if err == nil {
		t.Fatal("expected access error")
	}
	if !strings.Contains(err.Error(), "Permission denied") {
		t.Fatalf("expected git output in error, got %v", err)
	}
}

func TestShellQuoteEscapesSingleQuotes(t *testing.T) {
	t.Parallel()

	got := shellQuote("/Users/me/.ssh/yard's key")
	want := `'/Users/me/.ssh/yard'\''s key'`
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

type fakeRunner struct {
	outputs   []fakeCall
	runs      []fakeCall
	output    []byte
	outputErr error
	runErr    error
}

type fakeCall struct {
	env     []string
	command string
	args    []string
}

func (r *fakeRunner) Output(env []string, command string, args ...string) ([]byte, error) {
	r.outputs = append(r.outputs, fakeCall{env: env, command: command, args: args})
	return r.output, r.outputErr
}

func (r *fakeRunner) Run(env []string, command string, args ...string) error {
	r.runs = append(r.runs, fakeCall{env: env, command: command, args: args})
	return r.runErr
}
