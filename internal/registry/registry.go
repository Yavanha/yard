package registry

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	DefaultVMMode = "shared"
	DefaultVMName = "yard-shared"
)

type Registry struct {
	CurrentProject string
	Projects       map[string]Project
}

type Project struct {
	Path   string
	Config string
	VM     VM
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
				inVM = false
			case "config":
				project.Config = value
				inVM = false
			case "vm":
				if value != "" {
					return Registry{}, unsupportedLineError(lineNumber, line)
				}
				inVM = true
			default:
				return Registry{}, unsupportedLineError(lineNumber, line)
			}
			reg.Projects[currentProject] = project

		case strings.HasPrefix(line, "      ") && !strings.HasPrefix(line, "        "):
			if currentProject == "" || !inVM {
				return Registry{}, unsupportedLineError(lineNumber, line)
			}
			key, value, ok := splitKeyValue(strings.TrimPrefix(line, "      "))
			if !ok {
				return Registry{}, unsupportedLineError(lineNumber, line)
			}
			project := reg.Projects[currentProject]
			switch key {
			case "mode":
				project.VM.Mode = value
			case "name":
				project.VM.Name = value
			default:
				return Registry{}, unsupportedLineError(lineNumber, line)
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
		builder.WriteString("    vm:\n")
		builder.WriteString("      mode: ")
		builder.WriteString(project.VM.Mode)
		builder.WriteString("\n")
		builder.WriteString("      name: ")
		builder.WriteString(project.VM.Name)
		builder.WriteString("\n")
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
