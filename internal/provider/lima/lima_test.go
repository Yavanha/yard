package lima

import (
	"errors"
	"reflect"
	"testing"
)

func TestParseListParsesJSONStream(t *testing.T) {
	t.Parallel()

	instances, err := ParseList([]byte(`{"name":"alpha","status":"Running","vmType":"vz","cpus":4,"memory":6442450944,"disk":53687091200,"sshLocalPort":50000,"sshConfigFile":"/tmp/alpha/ssh.config"}
{"name":"beta","status":"Stopped","vmType":"qemu","cpus":2,"memory":2147483648,"disk":10737418240,"sshLocalPort":50001,"sshConfigFile":"/tmp/beta/ssh.config"}
`))
	if err != nil {
		t.Fatalf("ParseList returned error: %v", err)
	}
	if len(instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(instances))
	}
	assertEqual(t, instances[0].Name, "alpha")
	assertEqual(t, instances[0].Status, "Running")
	assertEqual(t, instances[1].Name, "beta")
}

func TestSSHArgs(t *testing.T) {
	t.Parallel()

	got := SSHArgs(Instance{
		Name:          "alpha",
		SSHConfigFile: "/tmp/alpha/ssh.config",
	}, []string{"echo", "hello"})

	want := []string{
		"-F",
		"/tmp/alpha/ssh.config",
		"-o",
		"ForwardAgent=yes",
		"-o",
		"ControlMaster=no",
		"-o",
		"StrictHostKeyChecking=accept-new",
		"-o",
		"ServerAliveInterval=30",
		"lima-alpha",
		"--",
		"echo",
		"hello",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func TestClientStartStop(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{}
	client := NewClient(runner)

	if err := client.Start("alpha"); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	if err := client.Stop("alpha"); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	want := [][]string{
		{"limactl", "start", "--yes", "alpha"},
		{"limactl", "stop", "--yes", "alpha"},
	}
	if !reflect.DeepEqual(runner.runs, want) {
		t.Fatalf("expected %#v, got %#v", want, runner.runs)
	}
}

func TestClientExecUsesSSH(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		outputs: map[string][]byte{
			"limactl list --format json alpha": []byte(`{"name":"alpha","status":"Running","sshConfigFile":"/tmp/alpha/ssh.config"}`),
		},
	}
	client := NewClient(runner)

	if err := client.Exec("alpha", []string{"uname", "-a"}); err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	if len(runner.runs) != 1 {
		t.Fatalf("expected one run, got %#v", runner.runs)
	}
	assertEqual(t, runner.runs[0][0], "ssh")
	assertEqual(t, runner.runs[0][len(runner.runs[0])-2], "uname")
	assertEqual(t, runner.runs[0][len(runner.runs[0])-1], "-a")
}

func TestClientExecRejectsStoppedVM(t *testing.T) {
	t.Parallel()

	runner := &fakeRunner{
		outputs: map[string][]byte{
			"limactl list --format json alpha": []byte(`{"name":"alpha","status":"Stopped","sshConfigFile":"/tmp/alpha/ssh.config"}`),
		},
	}
	client := NewClient(runner)

	err := client.Exec("alpha", []string{"uname", "-a"})
	if err == nil {
		t.Fatal("expected stopped VM to fail")
	}
}

func TestClientExecRequiresCommand(t *testing.T) {
	t.Parallel()

	client := NewClient(&fakeRunner{})
	err := client.Exec("alpha", nil)
	if err == nil {
		t.Fatal("expected missing command to fail")
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

func assertEqual(t *testing.T, got any, want any) {
	t.Helper()
	if got != want {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}
