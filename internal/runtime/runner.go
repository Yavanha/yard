package runtime

import (
	"io"
	"os"
	"os/exec"
)

type Runner interface {
	Output(command string, args ...string) ([]byte, error)
	Run(command string, args ...string) error
}

type ExecRunner struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func (r ExecRunner) Output(command string, args ...string) ([]byte, error) {
	cmd := exec.Command(command, args...)
	cmd.Stderr = r.stderr()
	return cmd.Output()
}

func (r ExecRunner) Run(command string, args ...string) error {
	cmd := exec.Command(command, args...)
	cmd.Stdin = r.stdin()
	cmd.Stdout = r.stdout()
	cmd.Stderr = r.stderr()
	return cmd.Run()
}

func (r ExecRunner) stdin() io.Reader {
	if r.Stdin != nil {
		return r.Stdin
	}
	return os.Stdin
}

func (r ExecRunner) stdout() io.Writer {
	if r.Stdout != nil {
		return r.Stdout
	}
	return os.Stdout
}

func (r ExecRunner) stderr() io.Writer {
	if r.Stderr != nil {
		return r.Stderr
	}
	return os.Stderr
}
