package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const FileName = ".devctl.yml"

var intPattern = regexp.MustCompile(`^-?\d+$`)

type Loaded struct {
	ConfigPath string         `json:"configPath"`
	Config     map[string]any `json:"config"`
}

func FindPath(startDir string) (string, bool) {
	current, err := filepath.Abs(startDir)
	if err != nil {
		return "", false
	}

	for {
		candidate := filepath.Join(current, FileName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, true
		}

		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

func ResolveProjectPath(projectPath string) (string, bool) {
	resolved, err := filepath.Abs(projectPath)
	if err != nil {
		return "", false
	}

	if info, err := os.Stat(resolved); err == nil && info.IsDir() {
		return FindPath(resolved)
	}

	return resolved, true
}

func Load(projectPath string, workDir string) (Loaded, error) {
	var configPath string
	var ok bool

	if projectPath != "" {
		configPath, ok = ResolveProjectPath(projectPath)
	} else {
		configPath, ok = FindPath(workDir)
	}
	if !ok {
		return Loaded{}, fmt.Errorf("no %s found", FileName)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		return Loaded{}, err
	}

	parsed, err := ParseSimpleYAML(string(content))
	if err != nil {
		return Loaded{}, err
	}

	return Loaded{
		ConfigPath: configPath,
		Config:     parsed,
	}, nil
}

func ParseSimpleYAML(content string) (map[string]any, error) {
	root := map[string]any{}
	stack := []yamlFrame{{indent: -1, values: root}}

	for lineNumber, rawLine := range strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n") {
		line := stripInlineComment(rawLine)
		if strings.TrimSpace(line) == "" {
			continue
		}

		indent, ok := leadingSpaces(line)
		if !ok || indent%2 != 0 {
			return nil, unsupportedLineError(lineNumber, rawLine)
		}
		for len(stack) > 1 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		parent := stack[len(stack)-1]
		if indent > parent.indent+2 {
			return nil, unsupportedLineError(lineNumber, rawLine)
		}

		key, value, ok := splitKeyValue(strings.TrimLeft(line, " "))
		if !ok {
			return nil, unsupportedLineError(lineNumber, rawLine)
		}

		if strings.TrimSpace(value) == "" {
			child := map[string]any{}
			parent.values[key] = child
			stack = append(stack, yamlFrame{indent: indent, values: child})
			continue
		}

		parent.values[key] = coerceScalar(value)
	}

	return root, nil
}

type yamlFrame struct {
	indent int
	values map[string]any
}

func leadingSpaces(line string) (int, bool) {
	count := 0
	for _, char := range line {
		switch char {
		case ' ':
			count++
		case '\t':
			return 0, false
		default:
			return count, true
		}
	}
	return count, true
}

func splitKeyValue(line string) (string, string, bool) {
	key, value, found := strings.Cut(line, ":")
	if !found {
		return "", "", false
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return "", "", false
	}

	for _, char := range key {
		if !(char == '_' || char == '-' || char >= '0' && char <= '9' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z') {
			return "", "", false
		}
	}

	return key, value, true
}

func coerceScalar(value string) any {
	stripped := stripQuotes(strings.TrimSpace(value))
	if intPattern.MatchString(stripped) {
		if parsed, err := strconv.Atoi(stripped); err == nil {
			return parsed
		}
	}
	if stripped == "true" {
		return true
	}
	if stripped == "false" {
		return false
	}
	return stripped
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

func unsupportedLineError(lineNumber int, rawLine string) error {
	return fmt.Errorf("unsupported YAML line %d: %s", lineNumber+1, rawLine)
}
