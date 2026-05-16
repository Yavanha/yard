package lima

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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

type Client struct {
	Runner Runner
}

type Instance struct {
	Name          string `json:"name"`
	Hostname      string `json:"hostname"`
	Status        string `json:"status"`
	Dir           string `json:"dir"`
	VMType        string `json:"vmType"`
	Arch          string `json:"arch"`
	CPUs          int    `json:"cpus"`
	Memory        int64  `json:"memory"`
	Disk          int64  `json:"disk"`
	SSHLocalPort  int    `json:"sshLocalPort"`
	SSHConfigFile string `json:"sshConfigFile"`
}

func NewClient(runner Runner) Client {
	if runner == nil {
		runner = ExecRunner{
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}
	}
	return Client{Runner: runner}
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

func (c Client) List() ([]Instance, error) {
	output, err := c.Runner.Output("limactl", "list", "--format", "json")
	if err != nil {
		return nil, err
	}
	return ParseList(output)
}

func (c Client) Status(name string) (Instance, error) {
	output, err := c.Runner.Output("limactl", "list", "--format", "json", name)
	if err != nil {
		return Instance{}, err
	}
	instances, err := ParseList(output)
	if err != nil {
		return Instance{}, err
	}
	if len(instances) == 0 {
		return Instance{}, fmt.Errorf("VM not found: %s", name)
	}
	return instances[0], nil
}

func (c Client) Start(name string) error {
	return c.Runner.Run("limactl", "start", "--yes", name)
}

func (c Client) Stop(name string) error {
	return c.Runner.Run("limactl", "stop", "--yes", name)
}

func (c Client) Exec(name string, command []string) error {
	if len(command) == 0 {
		return errors.New("exec requires a command")
	}

	instance, err := c.Status(name)
	if err != nil {
		return err
	}
	if instance.SSHConfigFile == "" {
		return fmt.Errorf("VM has no SSH config file: %s", name)
	}
	if !strings.EqualFold(instance.Status, "Running") {
		return fmt.Errorf("VM is not running: %s. Run: yard vm start %s", name, name)
	}

	return c.Runner.Run("ssh", SSHArgs(instance, command)...)
}

func ParseList(content []byte) ([]Instance, error) {
	decoder := json.NewDecoder(bytes.NewReader(content))
	instances := []Instance{}

	for {
		var instance Instance
		if err := decoder.Decode(&instance); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		instances = append(instances, instance)
	}
	return instances, nil
}

func SSHArgs(instance Instance, command []string) []string {
	args := []string{
		"-F",
		instance.SSHConfigFile,
		"-o",
		"ForwardAgent=yes",
		"-o",
		"ControlMaster=no",
		"-o",
		"StrictHostKeyChecking=accept-new",
		"-o",
		"ServerAliveInterval=30",
		"lima-" + instance.Name,
		"--",
	}
	return append(args, command...)
}
