package lima

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"yard/internal/config"
)

const ubuntuImageTemplate = "template:_images/ubuntu-24.04"

var sizePattern = regexp.MustCompile(`^(\d+(?:\.\d+)?)([KMGTP]?)B?$`)

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

type SetupResult struct {
	VMName  string
	Created bool
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

func (c Client) Exists(name string) (bool, error) {
	instances, err := c.List()
	if err != nil {
		return false, err
	}
	for _, instance := range instances {
		if instance.Name == name {
			return true, nil
		}
	}
	return false, nil
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

func (c Client) Delete(name string) error {
	return c.Runner.Run("limactl", "delete", "--yes", name)
}

func (c Client) Setup(project config.ProjectConfig) (SetupResult, error) {
	exists, err := c.Exists(project.VMName)
	if err != nil {
		return SetupResult{}, err
	}
	if exists {
		return SetupResult{VMName: project.VMName, Created: false}, nil
	}

	content, err := RenderConfig(project)
	if err != nil {
		return SetupResult{}, err
	}

	tempDir, err := os.MkdirTemp("", "yard-lima-")
	if err != nil {
		return SetupResult{}, err
	}
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, project.VMName+".yaml")
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		return SetupResult{}, err
	}

	if err := c.Runner.Run("limactl", "start", "--yes", "--name", project.VMName, configPath); err != nil {
		return SetupResult{}, err
	}
	return SetupResult{VMName: project.VMName, Created: true}, nil
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

func (c Client) ExecOutput(name string, command []string) ([]byte, error) {
	if len(command) == 0 {
		return nil, errors.New("exec requires a command")
	}

	instance, err := c.Status(name)
	if err != nil {
		return nil, err
	}
	if instance.SSHConfigFile == "" {
		return nil, fmt.Errorf("VM has no SSH config file: %s", name)
	}
	if !strings.EqualFold(instance.Status, "Running") {
		return nil, fmt.Errorf("VM is not running: %s. Run: yard vm start %s", name, name)
	}

	return c.Runner.Output("ssh", SSHArgs(instance, command)...)
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
	return append(args, shellCommand(command))
}

func RenderConfig(project config.ProjectConfig) (string, error) {
	memory, err := FormatSizeForLima(project.Resources.Memory)
	if err != nil {
		return "", err
	}
	disk, err := FormatSizeForLima(project.Resources.Disk)
	if err != nil {
		return "", err
	}

	ports := append([]config.PortMapping(nil), project.Ports...)
	sort.Slice(ports, func(left int, right int) bool {
		if ports[left].Port == ports[right].Port {
			return ports[left].Name < ports[right].Name
		}
		return ports[left].Port < ports[right].Port
	})

	var builder strings.Builder
	builder.WriteString("minimumLimaVersion: 2.0.0\n")
	builder.WriteString("base:\n")
	builder.WriteString("  - " + ubuntuImageTemplate + "\n")
	builder.WriteString("vmType: " + quoteYAML(project.VM.Type) + "\n")
	builder.WriteString("arch: \"default\"\n")
	builder.WriteString(fmt.Sprintf("cpus: %d\n", project.Resources.CPUs))
	builder.WriteString("memory: " + quoteYAML(memory) + "\n")
	builder.WriteString("disk: " + quoteYAML(disk) + "\n")
	builder.WriteString("mounts: []\n")
	builder.WriteString("containerd:\n")
	builder.WriteString("  system: false\n")
	builder.WriteString("  user: false\n")
	builder.WriteString("ssh:\n")
	builder.WriteString("  forwardAgent: true\n")
	builder.WriteString("  loadDotSSHPubKeys: true\n")
	builder.WriteString("user:\n")
	builder.WriteString("  name: " + quoteYAML(project.VMUser) + "\n")
	builder.WriteString("  home: " + quoteYAML("/home/"+project.VMUser) + "\n")
	builder.WriteString("portForwards:\n")
	if len(ports) == 0 {
		builder.WriteString("  []\n")
	} else {
		for _, port := range ports {
			builder.WriteString(fmt.Sprintf("  - guestPort: %d\n", port.Port))
			builder.WriteString(fmt.Sprintf("    hostPort: %d\n", port.Port))
			builder.WriteString("    hostIP: \"127.0.0.1\"\n")
		}
	}
	return builder.String(), nil
}

func FormatSizeForLima(value string) (string, error) {
	match := sizePattern.FindStringSubmatch(strings.ToUpper(strings.TrimSpace(value)))
	if match == nil {
		return "", fmt.Errorf("invalid size: %s", value)
	}

	amount, err := strconv.ParseFloat(match[1], 64)
	if err != nil {
		return "", err
	}

	multiplier, ok := map[string]float64{
		"":  1 / (1024 * 1024),
		"K": 1.0 / 1024,
		"M": 1,
		"G": 1024,
		"T": 1024 * 1024,
		"P": 1024 * 1024 * 1024,
	}[match[2]]
	if !ok {
		return "", fmt.Errorf("invalid size unit: %s", match[2])
	}

	mib := amount * multiplier
	gib := mib / 1024
	if gib == float64(int64(gib)) {
		return fmt.Sprintf("%dGiB", int64(gib)), nil
	}
	return fmt.Sprintf("%.2fGiB", gib), nil
}

func quoteYAML(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func shellCommand(command []string) string {
	quoted := make([]string, 0, len(command))
	for _, arg := range command {
		quoted = append(quoted, shellQuote(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
