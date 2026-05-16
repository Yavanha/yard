package process

import (
	"bufio"
	"errors"
	"fmt"
	"strings"

	"yard/internal/config"
)

type Service struct {
	ProjectName string
	Name        string
	RepoDir     string
	Workdir     string
	Command     string
	Port        int
}

type State struct {
	Project string
	Service string
	Status  string
	PID     string
	Port    int
	Command string
	Workdir string
	Log     string
}

func ServiceFromConfig(projectName string, project config.ProjectConfig, service config.ServiceConfig) Service {
	return Service{
		ProjectName: projectName,
		Name:        service.Name,
		RepoDir:     project.RepoDir,
		Workdir:     service.Workdir,
		Command:     service.Command,
		Port:        service.Port,
	}
}

func StartCommand(service Service) ([]string, error) {
	if err := validateService(service); err != nil {
		return nil, err
	}
	return []string{"sh", "-lc", startScript(service)}, nil
}

func StopCommand(projectName string, serviceName string) ([]string, error) {
	if err := validateName("project", projectName); err != nil {
		return nil, err
	}
	if err := validateName("service", serviceName); err != nil {
		return nil, err
	}
	return []string{"sh", "-lc", stopScript(projectName, serviceName)}, nil
}

func StatusCommand(projectName string, serviceName string) ([]string, error) {
	if err := validateName("project", projectName); err != nil {
		return nil, err
	}
	if err := validateName("service", serviceName); err != nil {
		return nil, err
	}
	return []string{"sh", "-lc", statusScript(projectName, serviceName)}, nil
}

func ParseStatusOutput(projectName string, service config.ServiceConfig, output []byte) State {
	state := State{
		Project: projectName,
		Service: service.Name,
		Status:  "unknown",
		PID:     "-",
		Port:    service.Port,
		Command: service.Command,
		Workdir: service.Workdir,
		Log:     "-",
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), "=")
		if !ok {
			continue
		}
		switch key {
		case "state":
			if value != "" {
				state.Status = value
			}
		case "pid":
			if value != "" {
				state.PID = value
			}
		case "log":
			if value != "" {
				state.Log = value
			}
		}
	}
	return state
}

func StaticState(projectName string, service config.ServiceConfig, status string) State {
	return State{
		Project: projectName,
		Service: service.Name,
		Status:  status,
		PID:     "-",
		Port:    service.Port,
		Command: service.Command,
		Workdir: service.Workdir,
		Log:     "-",
	}
}

func validateService(service Service) error {
	if err := validateName("project", service.ProjectName); err != nil {
		return err
	}
	if err := validateName("service", service.Name); err != nil {
		return err
	}
	if service.RepoDir == "" {
		return errors.New("repo_dir is required to start a process")
	}
	if service.Command == "" {
		return fmt.Errorf("service %s command is required", service.Name)
	}
	if strings.ContainsAny(service.Command, "\r\n") {
		return fmt.Errorf("service %s command must be a single line", service.Name)
	}
	if service.Workdir == "" {
		return errors.New("service workdir is required")
	}
	return nil
}

func validateName(kind string, name string) error {
	if name == "" {
		return fmt.Errorf("%s name is required", kind)
	}
	for _, char := range name {
		if !(char == '_' || char == '-' || char >= '0' && char <= '9' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z') {
			return fmt.Errorf("invalid %s name: %s", kind, name)
		}
	}
	return nil
}

func startScript(service Service) string {
	return strings.Join([]string{
		"set -eu",
		fmt.Sprintf("state_dir=\"$HOME/.yard/processes/%s/%s\"", service.ProjectName, service.Name),
		"mkdir -p \"$state_dir\"",
		"pid_file=\"$state_dir/pid\"",
		"log_file=\"$state_dir/stdout.log\"",
		"if [ -f \"$pid_file\" ]; then",
		"  pid=\"$(cat \"$pid_file\" || true)\"",
		"  case \"$pid\" in ''|*[!0-9]*) pid= ;; esac",
		"  if [ -n \"$pid\" ] && kill -0 \"$pid\" 2>/dev/null; then",
		"    printf 'already_running pid=%s log=%s\\n' \"$pid\" \"$log_file\"",
		"    exit 0",
		"  fi",
		"fi",
		"repo_dir=" + shellQuote(service.RepoDir),
		"workdir=" + shellQuote(service.Workdir),
		"yard_command=" + shellQuote(service.Command),
		"cd \"$repo_dir\"",
		"cd \"$workdir\"",
		"if command -v setsid >/dev/null 2>&1; then",
		"  nohup setsid sh -lc \"$yard_command\" >\"$log_file\" 2>&1 </dev/null &",
		"else",
		"  nohup sh -lc \"$yard_command\" >\"$log_file\" 2>&1 </dev/null &",
		"fi",
		"pid=\"$!\"",
		"printf '%s\\n' \"$pid\" >\"$pid_file\"",
		"printf '%s\\n' \"$yard_command\" >\"$state_dir/command\"",
		"printf '%s\\n' \"$PWD\" >\"$state_dir/workdir\"",
		"printf '%s\\n' \"$log_file\" >\"$state_dir/log\"",
		"printf 'started pid=%s log=%s\\n' \"$pid\" \"$log_file\"",
	}, "\n")
}

func stopScript(projectName string, serviceName string) string {
	return strings.Join([]string{
		"set -u",
		fmt.Sprintf("state_dir=\"$HOME/.yard/processes/%s/%s\"", projectName, serviceName),
		"pid_file=\"$state_dir/pid\"",
		"if [ ! -f \"$pid_file\" ]; then",
		"  echo 'not_running'",
		"  exit 0",
		"fi",
		"pid=\"$(cat \"$pid_file\" || true)\"",
		"case \"$pid\" in",
		"  ''|*[!0-9]*)",
		"    rm -f \"$pid_file\"",
		"    echo 'not_running'",
		"    exit 0",
		"    ;;",
		"esac",
		"if [ -z \"$pid\" ] || ! kill -0 \"$pid\" 2>/dev/null; then",
		"  rm -f \"$pid_file\"",
		"  echo 'not_running'",
		"  exit 0",
		"fi",
		"if ! kill -TERM \"-$pid\" 2>/dev/null; then",
		"  kill -TERM \"$pid\" 2>/dev/null || true",
		"fi",
		"rm -f \"$pid_file\"",
		"printf 'stopped pid=%s\\n' \"$pid\"",
	}, "\n")
}

func statusScript(projectName string, serviceName string) string {
	return strings.Join([]string{
		"set -u",
		fmt.Sprintf("state_dir=\"$HOME/.yard/processes/%s/%s\"", projectName, serviceName),
		"pid_file=\"$state_dir/pid\"",
		"log_file=\"$state_dir/stdout.log\"",
		"state=stopped",
		"pid=",
		"if [ -f \"$pid_file\" ]; then",
		"  pid=\"$(cat \"$pid_file\" || true)\"",
		"  case \"$pid\" in",
		"    ''|*[!0-9]*) state=stale ;;",
		"    *)",
		"      if kill -0 \"$pid\" 2>/dev/null; then",
		"        state=running",
		"      else",
		"        state=stale",
		"      fi",
		"      ;;",
		"  esac",
		"fi",
		"printf 'state=%s\\n' \"$state\"",
		"printf 'pid=%s\\n' \"$pid\"",
		"printf 'log=%s\\n' \"$log_file\"",
	}, "\n")
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
