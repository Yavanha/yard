package runtime

import (
	"errors"
	"reflect"
	"testing"

	"yard/internal/provider/lima"
)

func TestLocalVMExecDelegatesToLimaClient(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		outputs: map[string][]byte{
			"limactl list --format json api-dev": []byte(`{"name":"api-dev","status":"Running","sshConfigFile":"/tmp/api-dev/ssh.config"}`),
		},
	}
	target := NewLocalVM(lima.NewClient(runner), "api-dev")

	if err := target.Exec([]string{"printf", "ok"}); err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	if len(runner.runs) != 1 {
		t.Fatalf("expected one run, got %#v", runner.runs)
	}
	if runner.runs[0][0] != "ssh" {
		t.Fatalf("expected ssh command, got %#v", runner.runs[0])
	}
	if got, want := runner.runs[0][len(runner.runs[0])-2:], []string{"--", "'printf' 'ok'"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected command suffix %#v, got %#v", want, got)
	}
}

func TestLocalVMExecOutputDelegatesToLimaClient(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		outputs: map[string][]byte{
			"limactl list --format json api-dev": []byte(`{"name":"api-dev","status":"Running","sshConfigFile":"/tmp/api-dev/ssh.config"}`),
			"ssh -F /tmp/api-dev/ssh.config -o ForwardAgent=yes -o ControlMaster=no -o StrictHostKeyChecking=accept-new -o ServerAliveInterval=30 lima-api-dev -- 'printf' 'ok'": []byte("ok"),
		},
	}
	target := NewLocalVM(lima.NewClient(runner), "api-dev")

	output, err := target.ExecOutput([]string{"printf", "ok"})
	if err != nil {
		t.Fatalf("ExecOutput returned error: %v", err)
	}
	if string(output) != "ok" {
		t.Fatalf("expected ok, got %q", string(output))
	}
}

type fakeRunner struct {
	outputs map[string][]byte
	runs    [][]string
}

func (r *fakeRunner) Output(command string, args ...string) ([]byte, error) {
	key := command
	for _, arg := range args {
		key += " " + arg
	}
	if output, ok := r.outputs[key]; ok {
		return output, nil
	}
	if r.outputs == nil {
		return nil, nil
	}
	return nil, errors.New("unexpected output command: " + key)
}

func (r *fakeRunner) Run(command string, args ...string) error {
	call := append([]string{command}, args...)
	r.runs = append(r.runs, call)
	return nil
}
