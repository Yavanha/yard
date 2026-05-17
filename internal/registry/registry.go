package registry

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	DefaultRuntimeType = "local-vm"
	RuntimeTypeLocalVM = "local-vm"
	RuntimeTypeRemote  = "remote-server"
	DefaultRemotePort  = 22
	DefaultVMMode      = "shared"
	DefaultVMName      = "yard-shared"
)

type Registry struct {
	CurrentProject string
	Projects       map[string]Project
}

type Project struct {
	Path    string
	Config  string
	Git     Git
	Runtime RuntimeTarget
	Remote  RemoteServer
	VM      VM
}

type Git struct {
	IdentityFile string
	Fingerprint  string
}

type RuntimeTarget struct {
	Type string
}

type RemoteServer struct {
	Host         string
	User         string
	Port         int
	Workdir      string
	IdentityFile string
}

type VM struct {
	Mode string
	Name string
}

func DefaultPath() (string, error) {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configHome = filepath.Join(home, ".config")
	}
	return filepath.Join(configHome, "yard", "config.yaml"), nil
}

func Load(path string) (Registry, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return New(), nil
	}
	if err != nil {
		return Registry{}, err
	}
	return Parse(content)
}

func Save(path string, reg Registry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, Marshal(reg), 0o600)
}

func New() Registry {
	return Registry{
		Projects: map[string]Project{},
	}
}

func (reg Registry) Add(name string, project Project) (Registry, error) {
	if err := validateProjectName(name); err != nil {
		return Registry{}, err
	}
	if project.Path == "" {
		return Registry{}, errors.New("project path is required")
	}

	if reg.Projects == nil {
		reg.Projects = map[string]Project{}
	}

	normalized, err := normalizeProject(project)
	if err != nil {
		return Registry{}, err
	}

	reg.Projects[name] = normalized
	if reg.CurrentProject == "" {
		reg.CurrentProject = name
	}
	return reg, nil
}

func (reg Registry) Use(name string) (Registry, error) {
	if _, ok := reg.Projects[name]; !ok {
		return Registry{}, fmt.Errorf("unknown project: %s", name)
	}
	reg.CurrentProject = name
	return reg, nil
}

func (reg Registry) Remove(name string) (Registry, error) {
	if _, ok := reg.Projects[name]; !ok {
		return Registry{}, fmt.Errorf("unknown project: %s", name)
	}
	delete(reg.Projects, name)
	if reg.CurrentProject == name {
		reg.CurrentProject = ""
	}
	return reg, nil
}

func (reg Registry) Resolve(name string) (string, Project, error) {
	if name == "" {
		if reg.CurrentProject == "" {
			return "", Project{}, errors.New("no current project configured. Run: yard use <name>")
		}
		name = reg.CurrentProject
	}

	project, ok := reg.Projects[name]
	if !ok {
		return "", Project{}, fmt.Errorf("unknown project: %s", name)
	}
	return name, project, nil
}

func (reg Registry) ProjectNames() []string {
	names := make([]string, 0, len(reg.Projects))
	for name := range reg.Projects {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func Parse(content []byte) (Registry, error) {
	reg := New()
	scanner := bufio.NewScanner(bytes.NewReader(content))
	var currentProject string
	inProjects := false
	inGit := false
	inRemote := false
	inRuntime := false
	inVM := false
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := stripInlineComment(scanner.Text())
		if strings.TrimSpace(line) == "" {
			continue
		}

		switch {
		case !strings.HasPrefix(line, " "):
			inGit = false
			inRemote = false
			inRuntime = false
			inVM = false
			key, value, ok := splitKeyValue(line)
			if !ok {
				return Registry{}, unsupportedLineError(lineNumber, line)
			}
			switch key {
			case "current_project":
				reg.CurrentProject = value
				inProjects = false
			case "projects":
				if value != "" {
					return Registry{}, unsupportedLineError(lineNumber, line)
				}
				inProjects = true
			default:
				return Registry{}, unsupportedLineError(lineNumber, line)
			}

		case strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "    "):
			if !inProjects {
				return Registry{}, unsupportedLineError(lineNumber, line)
			}
			key, value, ok := splitKeyValue(strings.TrimPrefix(line, "  "))
			if !ok || value != "" {
				return Registry{}, unsupportedLineError(lineNumber, line)
			}
			if err := validateProjectName(key); err != nil {
				return Registry{}, err
			}
			currentProject = key
			inGit = false
			inRemote = false
			inRuntime = false
			inVM = false
			reg.Projects[currentProject] = Project{}

		case strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "      "):
			if currentProject == "" {
				return Registry{}, unsupportedLineError(lineNumber, line)
			}
			key, value, ok := splitKeyValue(strings.TrimPrefix(line, "    "))
			if !ok {
				return Registry{}, unsupportedLineError(lineNumber, line)
			}
			project := reg.Projects[currentProject]
			switch key {
			case "path":
				project.Path = value
				inGit = false
				inRemote = false
				inRuntime = false
				inVM = false
			case "config":
				project.Config = value
				inGit = false
				inRemote = false
				inRuntime = false
				inVM = false
			case "git":
				if value != "" {
					return Registry{}, unsupportedLineError(lineNumber, line)
				}
				inGit = true
				inRemote = false
				inRuntime = false
				inVM = false
			case "remote":
				if value != "" {
					return Registry{}, unsupportedLineError(lineNumber, line)
				}
				inGit = false
				inRemote = true
				inRuntime = false
				inVM = false
			case "runtime":
				if value != "" {
					return Registry{}, unsupportedLineError(lineNumber, line)
				}
				inGit = false
				inRemote = false
				inRuntime = true
				inVM = false
			case "vm":
				if value != "" {
					return Registry{}, unsupportedLineError(lineNumber, line)
				}
				inGit = false
				inRemote = false
				inRuntime = false
				inVM = true
			default:
				return Registry{}, unsupportedLineError(lineNumber, line)
			}
			reg.Projects[currentProject] = project

		case strings.HasPrefix(line, "      ") && !strings.HasPrefix(line, "        "):
			if currentProject == "" || (!inGit && !inRemote && !inRuntime && !inVM) {
				return Registry{}, unsupportedLineError(lineNumber, line)
			}
			key, value, ok := splitKeyValue(strings.TrimPrefix(line, "      "))
			if !ok {
				return Registry{}, unsupportedLineError(lineNumber, line)
			}
			project := reg.Projects[currentProject]
			if inGit {
				switch key {
				case "identity_file":
					project.Git.IdentityFile = value
				case "fingerprint":
					project.Git.Fingerprint = value
				default:
					return Registry{}, unsupportedLineError(lineNumber, line)
				}
			} else if inRemote {
				switch key {
				case "host":
					project.Remote.Host = value
				case "user":
					project.Remote.User = value
				case "port":
					port, err := strconv.Atoi(value)
					if err != nil {
						return Registry{}, fmt.Errorf("invalid remote.port on line %d: %s", lineNumber, value)
					}
					project.Remote.Port = port
				case "workdir":
					project.Remote.Workdir = value
				case "identity_file":
					project.Remote.IdentityFile = value
				default:
					return Registry{}, unsupportedLineError(lineNumber, line)
				}
			} else if inRuntime {
				switch key {
				case "type":
					project.Runtime.Type = value
				default:
					return Registry{}, unsupportedLineError(lineNumber, line)
				}
			} else {
				switch key {
				case "mode":
					project.VM.Mode = value
				case "name":
					project.VM.Name = value
				default:
					return Registry{}, unsupportedLineError(lineNumber, line)
				}
			}
			reg.Projects[currentProject] = project

		default:
			return Registry{}, unsupportedLineError(lineNumber, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return Registry{}, err
	}

	for name, project := range reg.Projects {
		normalized, err := normalizeProject(project)
		if err != nil {
			return Registry{}, fmt.Errorf("project %s: %w", name, err)
		}
		reg.Projects[name] = normalized
	}
	return reg, nil
}

func Marshal(reg Registry) []byte {
	var builder strings.Builder
	if reg.CurrentProject != "" {
		builder.WriteString("current_project: ")
		builder.WriteString(reg.CurrentProject)
		builder.WriteString("\n")
	}
	builder.WriteString("projects:\n")
	for _, name := range reg.ProjectNames() {
		project := reg.Projects[name]
		builder.WriteString("  ")
		builder.WriteString(name)
		builder.WriteString(":\n")
		builder.WriteString("    path: ")
		builder.WriteString(project.Path)
		builder.WriteString("\n")
		if project.Config != "" {
			builder.WriteString("    config: ")
			builder.WriteString(project.Config)
			builder.WriteString("\n")
		}
		if project.Git.IdentityFile != "" || project.Git.Fingerprint != "" {
			builder.WriteString("    git:\n")
			if project.Git.IdentityFile != "" {
				builder.WriteString("      identity_file: ")
				builder.WriteString(project.Git.IdentityFile)
				builder.WriteString("\n")
			}
			if project.Git.Fingerprint != "" {
				builder.WriteString("      fingerprint: ")
				builder.WriteString(project.Git.Fingerprint)
				builder.WriteString("\n")
			}
		}
		builder.WriteString("    runtime:\n")
		builder.WriteString("      type: ")
		builder.WriteString(project.Runtime.Type)
		builder.WriteString("\n")
		if project.Remote.Host != "" || project.Remote.User != "" || project.Remote.Port != 0 || project.Remote.Workdir != "" || project.Remote.IdentityFile != "" {
			builder.WriteString("    remote:\n")
			if project.Remote.Host != "" {
				builder.WriteString("      host: ")
				builder.WriteString(project.Remote.Host)
				builder.WriteString("\n")
			}
			if project.Remote.User != "" {
				builder.WriteString("      user: ")
				builder.WriteString(project.Remote.User)
				builder.WriteString("\n")
			}
			if project.Remote.Port != 0 {
				builder.WriteString("      port: ")
				builder.WriteString(strconv.Itoa(project.Remote.Port))
				builder.WriteString("\n")
			}
			if project.Remote.Workdir != "" {
				builder.WriteString("      workdir: ")
				builder.WriteString(project.Remote.Workdir)
				builder.WriteString("\n")
			}
			if project.Remote.IdentityFile != "" {
				builder.WriteString("      identity_file: ")
				builder.WriteString(project.Remote.IdentityFile)
				builder.WriteString("\n")
			}
		}
		if project.Runtime.Type == RuntimeTypeLocalVM || project.VM.Mode != "" || project.VM.Name != "" {
			builder.WriteString("    vm:\n")
			if project.VM.Mode != "" {
				builder.WriteString("      mode: ")
				builder.WriteString(project.VM.Mode)
				builder.WriteString("\n")
			}
			if project.VM.Name != "" {
				builder.WriteString("      name: ")
				builder.WriteString(project.VM.Name)
				builder.WriteString("\n")
			}
		}
	}
	return []byte(builder.String())
}

func normalizeProject(project Project) (Project, error) {
	absPath, err := filepath.Abs(project.Path)
	if err != nil {
		return Project{}, err
	}
	project.Path = absPath

	if project.Config == "" {
		project.Config = filepath.Join(project.Path, ".devctl.yml")
	} else {
		absConfig, err := filepath.Abs(project.Config)
		if err != nil {
			return Project{}, err
		}
		project.Config = absConfig
	}

	if project.Git.IdentityFile != "" {
		identityFile, err := filepath.Abs(project.Git.IdentityFile)
		if err != nil {
			return Project{}, err
		}
		project.Git.IdentityFile = identityFile
	}

	if project.Remote.IdentityFile != "" {
		identityFile, err := filepath.Abs(project.Remote.IdentityFile)
		if err != nil {
			return Project{}, err
		}
		project.Remote.IdentityFile = identityFile
	}
	if project.Remote.Port < 0 || project.Remote.Port > 65535 {
		return Project{}, fmt.Errorf("unsupported remote.port: %d", project.Remote.Port)
	}

	if project.Runtime.Type == "" {
		project.Runtime.Type = DefaultRuntimeType
	}
	if project.Runtime.Type != RuntimeTypeLocalVM && project.Runtime.Type != RuntimeTypeRemote {
		return Project{}, fmt.Errorf("unsupported runtime.type: %s", project.Runtime.Type)
	}

	if project.Runtime.Type == RuntimeTypeLocalVM {
		if project.VM.Mode == "" {
			project.VM.Mode = DefaultVMMode
		}
		if project.VM.Mode != "shared" && project.VM.Mode != "dedicated" {
			return Project{}, fmt.Errorf("unsupported vm.mode: %s", project.VM.Mode)
		}

		if project.VM.Name == "" {
			if project.VM.Mode == "shared" {
				project.VM.Name = DefaultVMName
			} else {
				project.VM.Name = filepath.Base(project.Path) + "-dev"
			}
		}
	}

	return project, nil
}

func validateProjectName(name string) error {
	if name == "" {
		return errors.New("project name is required")
	}
	for _, char := range name {
		if !(char == '_' || char == '-' || char >= '0' && char <= '9' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z') {
			return fmt.Errorf("invalid project name: %s", name)
		}
	}
	return nil
}

func splitKeyValue(line string) (string, string, bool) {
	key, value, found := strings.Cut(line, ":")
	if !found {
		return "", "", false
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(stripQuotes(value))
	if key == "" {
		return "", "", false
	}
	return key, value, true
}

func stripQuotes(value string) string {
	if len(value) < 2 {
		return value
	}
	if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
		return value[1 : len(value)-1]
	}
	return value
}

func stripInlineComment(line string) string {
	index := strings.Index(line, " #")
	if index == -1 {
		return line
	}
	return line[:index]
}

func unsupportedLineError(lineNumber int, line string) error {
	return fmt.Errorf("unsupported registry line %d: %s", lineNumber, line)
}
