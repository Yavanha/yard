package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

type ScaffoldOptions struct {
	Project        string
	Repo           string
	VMName         string
	VMUser         string
	RepoDir        string
	VMProvider     string
	VMType         string
	CPUs           int
	Memory         string
	Disk           string
	ServiceName    string
	ServiceCommand string
	ServiceWorkdir string
	ServicePort    int
}

func DefaultScaffoldOptions(projectName string) ScaffoldOptions {
	if projectName == "" {
		projectName = "example-app"
	}

	return ScaffoldOptions{
		Project:        projectName,
		Repo:           "",
		VMName:         projectName + "-dev",
		VMUser:         "ubuntu",
		RepoDir:        "/home/ubuntu/workspaces/" + projectName,
		VMProvider:     "auto",
		VMType:         defaultVMType(),
		CPUs:           4,
		Memory:         "6G",
		Disk:           "50G",
		ServiceName:    "app",
		ServiceCommand: "make dev",
		ServiceWorkdir: ".",
		ServicePort:    3000,
	}
}

func (options ScaffoldOptions) Normalized() ScaffoldOptions {
	defaults := DefaultScaffoldOptions(options.Project)
	if options.Project == "" {
		options.Project = defaults.Project
	}
	if options.VMName == "" {
		options.VMName = options.Project + "-dev"
	}
	if options.VMUser == "" {
		options.VMUser = defaults.VMUser
	}
	if options.RepoDir == "" {
		options.RepoDir = "/home/ubuntu/workspaces/" + options.Project
	}
	if options.VMProvider == "" {
		options.VMProvider = defaults.VMProvider
	}
	if options.VMType == "" {
		options.VMType = defaults.VMType
	}
	if options.CPUs == 0 {
		options.CPUs = defaults.CPUs
	}
	if options.Memory == "" {
		options.Memory = defaults.Memory
	}
	if options.Disk == "" {
		options.Disk = defaults.Disk
	}
	if options.ServiceName == "" {
		options.ServiceName = defaults.ServiceName
	}
	if options.ServiceCommand == "" {
		options.ServiceCommand = defaults.ServiceCommand
	}
	if options.ServiceWorkdir == "" {
		options.ServiceWorkdir = defaults.ServiceWorkdir
	}
	if options.ServicePort == 0 {
		options.ServicePort = defaults.ServicePort
	}
	return options
}

func RenderScaffold(options ScaffoldOptions) ([]byte, error) {
	options = options.Normalized()
	if err := validateScaffoldOptions(options); err != nil {
		return nil, err
	}

	var builder strings.Builder
	builder.WriteString("project: ")
	builder.WriteString(quoteYAML(options.Project))
	builder.WriteString("\n")
	if options.Repo != "" {
		builder.WriteString("repo: ")
		builder.WriteString(quoteYAML(options.Repo))
		builder.WriteString("\n")
	}
	builder.WriteString("vm_name: ")
	builder.WriteString(quoteYAML(options.VMName))
	builder.WriteString("\n")
	builder.WriteString("vm_user: ")
	builder.WriteString(quoteYAML(options.VMUser))
	builder.WriteString("\n")
	builder.WriteString("repo_dir: ")
	builder.WriteString(quoteYAML(options.RepoDir))
	builder.WriteString("\n")
	builder.WriteString("vm:\n")
	builder.WriteString("  provider: ")
	builder.WriteString(quoteYAML(options.VMProvider))
	builder.WriteString("\n")
	builder.WriteString("  type: ")
	builder.WriteString(quoteYAML(options.VMType))
	builder.WriteString("\n")
	builder.WriteString("resources:\n")
	builder.WriteString(fmt.Sprintf("  cpus: %d\n", options.CPUs))
	builder.WriteString("  memory: ")
	builder.WriteString(quoteYAML(options.Memory))
	builder.WriteString("\n")
	builder.WriteString("  disk: ")
	builder.WriteString(quoteYAML(options.Disk))
	builder.WriteString("\n")
	builder.WriteString("ports:\n")
	builder.WriteString("  ")
	builder.WriteString(options.ServiceName)
	builder.WriteString(": ")
	builder.WriteString(fmt.Sprint(options.ServicePort))
	builder.WriteString("\n")
	builder.WriteString("services:\n")
	builder.WriteString("  ")
	builder.WriteString(options.ServiceName)
	builder.WriteString(":\n")
	builder.WriteString("    command: ")
	builder.WriteString(quoteYAML(options.ServiceCommand))
	builder.WriteString("\n")
	builder.WriteString("    workdir: ")
	builder.WriteString(quoteYAML(options.ServiceWorkdir))
	builder.WriteString("\n")
	builder.WriteString("    port: ")
	builder.WriteString(fmt.Sprint(options.ServicePort))
	builder.WriteString("\n")
	return []byte(builder.String()), nil
}

func validateScaffoldOptions(options ScaffoldOptions) error {
	for key, value := range map[string]string{
		"project":         options.Project,
		"vm_name":         options.VMName,
		"vm_user":         options.VMUser,
		"repo_dir":        options.RepoDir,
		"vm.provider":     options.VMProvider,
		"vm.type":         options.VMType,
		"service.name":    options.ServiceName,
		"service.command": options.ServiceCommand,
		"service.workdir": options.ServiceWorkdir,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("missing scaffold option: %s", key)
		}
	}
	if err := validateServiceName(options.ServiceName); err != nil {
		return err
	}
	if options.CPUs <= 0 {
		return fmt.Errorf("cpus must be positive")
	}
	if options.ServicePort <= 0 || options.ServicePort > 65535 {
		return fmt.Errorf("service port is out of range")
	}
	return nil
}

func quoteYAML(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
