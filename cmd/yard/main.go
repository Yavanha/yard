package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"yard/internal/config"
	"yard/internal/process"
	"yard/internal/prompt"
	"yard/internal/provider/lima"
	"yard/internal/registry"
)

const version = "0.2.0-dev"

type args struct {
	command      string
	subcommand   string
	positionals  []string
	projectPath  string
	registryPath string
	configPath   string
	vmMode       string
	vmName       string
	execCommand  []string
	help         bool
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(argv []string) error {
	parsed, err := parseArgs(argv)
	if err != nil {
		return err
	}

	if parsed.help || parsed.command == "" {
		printHelp()
		return nil
	}

	switch parsed.command {
	case "config":
		return runConfig(parsed)
	case "project":
		return runProject(parsed)
	case "use":
		return runUse(parsed)
	case "vm":
		return runVM(parsed)
	case "exec":
		return runExec(parsed)
	case "process":
		return runProcess(parsed)
	case "status":
		return runStatus(parsed)
	case "setup":
		return runSetup(parsed)
	default:
		return fmt.Errorf("unknown command: %s", parsed.command)
	}
}

func parseArgs(argv []string) (args, error) {
	parsed := args{}

	for index := 0; index < len(argv); index++ {
		value := argv[index]

		switch value {
		case "--":
			parsed.execCommand = append(parsed.execCommand, argv[index+1:]...)
			return parsed, nil
		case "--help", "-h":
			parsed.help = true
		case "--project":
			if index+1 >= len(argv) {
				return args{}, errors.New("--project requires a path")
			}
			parsed.projectPath = argv[index+1]
			index++
		case "--registry":
			if index+1 >= len(argv) {
				return args{}, errors.New("--registry requires a path")
			}
			parsed.registryPath = argv[index+1]
			index++
		case "--config":
			if index+1 >= len(argv) {
				return args{}, errors.New("--config requires a path")
			}
			parsed.configPath = argv[index+1]
			index++
		case "--vm-mode":
			if index+1 >= len(argv) {
				return args{}, errors.New("--vm-mode requires a value")
			}
			parsed.vmMode = argv[index+1]
			index++
		case "--vm-name":
			if index+1 >= len(argv) {
				return args{}, errors.New("--vm-name requires a value")
			}
			parsed.vmName = argv[index+1]
			index++
		default:
			if len(value) > 0 && value[0] == '-' {
				return args{}, fmt.Errorf("unknown flag: %s", value)
			}
			if parsed.command != "" {
				if parsed.subcommand == "" && (parsed.command == "project" || parsed.command == "vm" || parsed.command == "process") {
					parsed.subcommand = value
					continue
				}
				parsed.positionals = append(parsed.positionals, value)
				continue
			}
			parsed.command = value
		}
	}

	return parsed, nil
}

func runConfig(parsed args) error {
	workDir, err := os.Getwd()
	if err != nil {
		return err
	}

	projectPath, err := resolvedConfigPath(parsed)
	if err != nil {
		return err
	}

	loaded, err := config.Load(projectPath, workDir)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(loaded)
}

func resolvedConfigPath(parsed args) (string, error) {
	if parsed.projectPath != "" {
		if len(parsed.positionals) > 0 {
			return "", errors.New("config accepts either a project name or --project, not both")
		}
		return parsed.projectPath, nil
	}
	if len(parsed.positionals) > 1 {
		return "", errors.New("usage: config [project-name] [--project <path>]")
	}

	path, err := resolvedRegistryPath(parsed)
	if err != nil {
		return "", err
	}
	reg, err := registry.Load(path)
	if err != nil {
		return "", err
	}

	name := ""
	if len(parsed.positionals) == 1 {
		name = parsed.positionals[0]
	}
	_, project, err := reg.Resolve(name)
	if err != nil {
		return "", err
	}
	return project.Config, nil
}

func runProject(parsed args) error {
	switch parsed.subcommand {
	case "add":
		return runProjectAdd(parsed)
	case "list":
		return runProjectList(parsed)
	case "use":
		return runUse(args{
			positionals:  parsed.positionals,
			registryPath: parsed.registryPath,
		})
	default:
		if parsed.subcommand == "" {
			return errors.New("project requires a subcommand: add, list, or use")
		}
		return fmt.Errorf("unknown project subcommand: %s", parsed.subcommand)
	}
}

func runProjectAdd(parsed args) error {
	if len(parsed.positionals) == 0 {
		return runProjectAddInteractive(parsed, prompt.New(os.Stdin, os.Stdout))
	}
	if len(parsed.positionals) != 2 {
		return errors.New("usage: project add [<name> <path>]")
	}

	path, err := resolvedRegistryPath(parsed)
	if err != nil {
		return err
	}
	reg, err := registry.Load(path)
	if err != nil {
		return err
	}

	reg, err = reg.Add(parsed.positionals[0], registry.Project{
		Path:   parsed.positionals[1],
		Config: parsed.configPath,
		VM: registry.VM{
			Mode: parsed.vmMode,
			Name: parsed.vmName,
		},
	})
	if err != nil {
		return err
	}
	if err := registry.Save(path, reg); err != nil {
		return err
	}

	fmt.Printf("added project %s\n", parsed.positionals[0])
	return nil
}

func runProjectAddInteractive(parsed args, prompter prompt.Prompter) error {
	defaultPath, err := os.Getwd()
	if err != nil {
		return err
	}

	projectPath, err := prompter.Ask("Repo path", defaultPath, true)
	if err != nil {
		return err
	}
	absProjectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return err
	}

	defaultName := filepath.Base(absProjectPath)
	name, err := prompter.Ask("Project alias", defaultName, true)
	if err != nil {
		return err
	}

	defaultConfig := filepath.Join(absProjectPath, config.FileName)
	configPath := parsed.configPath
	if configPath == "" {
		configPath, err = prompter.Ask("Config path", defaultConfig, false)
		if err != nil {
			return err
		}
	}

	vmMode := parsed.vmMode
	if vmMode == "" {
		vmMode, err = prompter.Ask("VM mode (shared/dedicated)", registry.DefaultVMMode, true)
		if err != nil {
			return err
		}
	}
	if vmMode != "shared" && vmMode != "dedicated" {
		return fmt.Errorf("unsupported vm.mode: %s", vmMode)
	}

	defaultVMName := registry.DefaultVMName
	if vmMode == "dedicated" {
		defaultVMName = filepath.Base(absProjectPath) + "-dev"
	}
	vmName := parsed.vmName
	if vmName == "" {
		vmName, err = prompter.Ask("VM name", defaultVMName, true)
		if err != nil {
			return err
		}
	}

	path, err := resolvedRegistryPath(parsed)
	if err != nil {
		return err
	}
	reg, err := registry.Load(path)
	if err != nil {
		return err
	}

	next, err := reg.Add(name, registry.Project{
		Path:   absProjectPath,
		Config: configPath,
		VM: registry.VM{
			Mode: vmMode,
			Name: vmName,
		},
	})
	if err != nil {
		return err
	}

	fmt.Fprintln(prompter.Writer())
	fmt.Fprintln(prompter.Writer(), "Registry preview:")
	fmt.Fprint(prompter.Writer(), string(registry.Marshal(next)))

	confirmed, err := prompter.Confirm("Write registry", true)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Fprintln(prompter.Writer(), "Aborted.")
		return nil
	}

	if err := registry.Save(path, next); err != nil {
		return err
	}

	fmt.Printf("added project %s\n", name)
	return nil
}

func runProjectList(parsed args) error {
	path, err := resolvedRegistryPath(parsed)
	if err != nil {
		return err
	}
	reg, err := registry.Load(path)
	if err != nil {
		return err
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "CURRENT\tNAME\tPATH\tVM_MODE\tVM_NAME")
	for _, name := range reg.ProjectNames() {
		project := reg.Projects[name]
		current := ""
		if reg.CurrentProject == name {
			current = "*"
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", current, name, project.Path, project.VM.Mode, project.VM.Name)
	}
	return writer.Flush()
}

func runUse(parsed args) error {
	if len(parsed.positionals) != 1 {
		return errors.New("usage: use <name>")
	}

	path, err := resolvedRegistryPath(parsed)
	if err != nil {
		return err
	}
	reg, err := registry.Load(path)
	if err != nil {
		return err
	}

	reg, err = reg.Use(parsed.positionals[0])
	if err != nil {
		return err
	}
	if err := registry.Save(path, reg); err != nil {
		return err
	}

	fmt.Printf("current project: %s\n", parsed.positionals[0])
	return nil
}

func runVM(parsed args) error {
	switch parsed.subcommand {
	case "list":
		return runVMList(parsed)
	case "status":
		return runVMStatus(parsed)
	case "start":
		return runVMStart(parsed)
	case "stop":
		return runVMStop(parsed)
	case "exec":
		return runVMExec(parsed)
	default:
		if parsed.subcommand == "" {
			return errors.New("vm requires a subcommand: list, status, start, stop, or exec")
		}
		return fmt.Errorf("unknown vm subcommand: %s", parsed.subcommand)
	}
}

func runVMList(parsed args) error {
	if len(parsed.positionals) != 0 {
		return errors.New("usage: vm list")
	}

	client := lima.NewClient(nil)
	instances, err := client.List()
	if err != nil {
		return err
	}

	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "NAME\tSTATUS\tTYPE\tCPUS\tMEMORY\tDISK\tSSH_PORT")
	for _, instance := range instances {
		fmt.Fprintf(
			writer,
			"%s\t%s\t%s\t%d\t%s\t%s\t%d\n",
			instance.Name,
			instance.Status,
			instance.VMType,
			instance.CPUs,
			formatBytes(instance.Memory),
			formatBytes(instance.Disk),
			instance.SSHLocalPort,
		)
	}
	return writer.Flush()
}

func runVMStatus(parsed args) error {
	vmName, err := resolvedVMName(parsed)
	if err != nil {
		return err
	}

	client := lima.NewClient(nil)
	instance, err := client.Status(vmName)
	if err != nil {
		return err
	}

	fmt.Printf("name       %s\n", instance.Name)
	fmt.Printf("status     %s\n", instance.Status)
	fmt.Printf("type       %s\n", instance.VMType)
	fmt.Printf("arch       %s\n", instance.Arch)
	fmt.Printf("cpus       %d\n", instance.CPUs)
	fmt.Printf("memory     %s\n", formatBytes(instance.Memory))
	fmt.Printf("disk       %s\n", formatBytes(instance.Disk))
	fmt.Printf("ssh_port   %d\n", instance.SSHLocalPort)
	fmt.Printf("ssh_config %s\n", instance.SSHConfigFile)
	return nil
}

func runVMStart(parsed args) error {
	vmName, err := resolvedVMName(parsed)
	if err != nil {
		return err
	}

	client := lima.NewClient(nil)
	instance, err := client.Status(vmName)
	if err != nil {
		return err
	}
	if strings.EqualFold(instance.Status, "Running") {
		fmt.Printf("VM already running: %s\n", vmName)
		return nil
	}

	if err := client.Start(vmName); err != nil {
		return err
	}
	fmt.Printf("VM started: %s\n", vmName)
	return nil
}

func runVMStop(parsed args) error {
	vmName, err := resolvedVMName(parsed)
	if err != nil {
		return err
	}

	client := lima.NewClient(nil)
	instance, err := client.Status(vmName)
	if err != nil {
		return err
	}
	if strings.EqualFold(instance.Status, "Stopped") {
		fmt.Printf("VM already stopped: %s\n", vmName)
		return nil
	}

	if err := client.Stop(vmName); err != nil {
		return err
	}
	fmt.Printf("VM stopped: %s\n", vmName)
	return nil
}

func runVMExec(parsed args) error {
	vmName, err := resolvedVMName(parsed)
	if err != nil {
		return err
	}
	if len(parsed.execCommand) == 0 {
		return errors.New("usage: vm exec [project-or-vm] -- <command>")
	}

	client := lima.NewClient(nil)
	return client.Exec(vmName, parsed.execCommand)
}

func runExec(parsed args) error {
	if len(parsed.positionals) > 1 {
		return errors.New("usage: exec [project-name] -- <command>")
	}
	if len(parsed.execCommand) == 0 {
		return errors.New("usage: exec [project-name] -- <command>")
	}

	name := ""
	if len(parsed.positionals) == 1 {
		name = parsed.positionals[0]
	}
	_, project, err := resolvedRegistryProject(parsed, name)
	if err != nil {
		return err
	}

	client := lima.NewClient(nil)
	return client.Exec(project.VM.Name, parsed.execCommand)
}

func runProcess(parsed args) error {
	switch parsed.subcommand {
	case "list":
		return runProcessList(parsed)
	case "start":
		return runProcessStart(parsed)
	case "stop":
		return runProcessStop(parsed)
	default:
		if parsed.subcommand == "" {
			return errors.New("process requires a subcommand: list, start, or stop")
		}
		return fmt.Errorf("unknown process subcommand: %s", parsed.subcommand)
	}
}

func runProcessList(parsed args) error {
	if len(parsed.positionals) > 1 {
		return errors.New("usage: process list [project-name]")
	}
	if parsed.projectPath != "" {
		return errors.New("process list uses the project registry; --project is not supported")
	}

	projectName := ""
	if len(parsed.positionals) == 1 {
		projectName = parsed.positionals[0]
	}
	projectName, project, projectConfig, err := resolvedRuntimeProject(parsed, projectName)
	if err != nil {
		return err
	}

	client := lima.NewClient(nil)
	instances, err := client.List()
	if err != nil {
		return err
	}

	vmState := findVMState(instances, project.VM.Name)
	rows := make([]process.State, 0, len(projectConfig.Services))
	if !strings.EqualFold(vmState, "Running") {
		status := processStatusFromVMState(vmState)
		for _, service := range projectConfig.Services {
			rows = append(rows, process.StaticState(projectName, service, status))
		}
		return writeProcessRows(os.Stdout, rows, project.VM.Name)
	}

	for _, service := range projectConfig.Services {
		command, err := process.StatusCommand(projectName, service.Name)
		if err != nil {
			return err
		}
		output, err := client.ExecOutput(project.VM.Name, command)
		if err != nil {
			return err
		}
		rows = append(rows, process.ParseStatusOutput(projectName, service, output))
	}
	return writeProcessRows(os.Stdout, rows, project.VM.Name)
}

func runProcessStart(parsed args) error {
	projectName, serviceName, err := processActionTarget(parsed.positionals)
	if err != nil {
		return err
	}
	if parsed.projectPath != "" {
		return errors.New("process start uses the project registry; --project is not supported")
	}

	projectName, project, projectConfig, err := resolvedRuntimeProject(parsed, projectName)
	if err != nil {
		return err
	}
	service, err := findService(projectConfig.Services, serviceName)
	if err != nil {
		return err
	}

	command, err := process.StartCommand(process.ServiceFromConfig(projectName, projectConfig, service))
	if err != nil {
		return err
	}

	client := lima.NewClient(nil)
	return client.Exec(project.VM.Name, command)
}

func runProcessStop(parsed args) error {
	projectName, serviceName, err := processActionTarget(parsed.positionals)
	if err != nil {
		return err
	}
	if parsed.projectPath != "" {
		return errors.New("process stop uses the project registry; --project is not supported")
	}

	projectName, project, projectConfig, err := resolvedRuntimeProject(parsed, projectName)
	if err != nil {
		return err
	}
	if _, err := findService(projectConfig.Services, serviceName); err != nil {
		return err
	}

	command, err := process.StopCommand(projectName, serviceName)
	if err != nil {
		return err
	}

	client := lima.NewClient(nil)
	return client.Exec(project.VM.Name, command)
}

type statusRow struct {
	Current bool
	Project string
	VM      string
	VMState string
	VMMode  string
	Config  string
	Path    string
}

func runStatus(parsed args) error {
	if len(parsed.positionals) > 1 {
		return errors.New("usage: status [project-name]")
	}

	path, err := resolvedRegistryPath(parsed)
	if err != nil {
		return err
	}
	reg, err := registry.Load(path)
	if err != nil {
		return err
	}

	client := lima.NewClient(nil)
	instances, err := client.List()
	if err != nil {
		return err
	}

	filter := ""
	if len(parsed.positionals) == 1 {
		filter = parsed.positionals[0]
	}
	rows, err := buildStatusRows(reg, instances, filter)
	if err != nil {
		return err
	}
	return writeStatusRows(os.Stdout, rows)
}

func runSetup(parsed args) error {
	if len(parsed.positionals) > 1 {
		return errors.New("usage: setup [project-name] [--project <path>]")
	}

	projectConfig, err := resolvedProjectConfig(parsed)
	if err != nil {
		return err
	}
	if projectConfig.VM.Provider != "auto" && projectConfig.VM.Provider != "lima" {
		return fmt.Errorf("unsupported VM provider for setup: %s", projectConfig.VM.Provider)
	}

	client := lima.NewClient(nil)
	result, err := client.Setup(projectConfig)
	if err != nil {
		return err
	}
	if result.Created {
		fmt.Printf("VM created: %s\n", result.VMName)
		return nil
	}
	fmt.Printf("VM already exists: %s\n", result.VMName)
	return nil
}

func resolvedProjectConfig(parsed args) (config.ProjectConfig, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return config.ProjectConfig{}, err
	}

	projectPath := parsed.projectPath
	registryVMName := ""
	if projectPath == "" {
		if len(parsed.positionals) > 1 {
			return config.ProjectConfig{}, errors.New("usage: setup [project-name] [--project <path>]")
		}
		name := ""
		if len(parsed.positionals) == 1 {
			name = parsed.positionals[0]
		}
		_, project, err := resolvedRegistryProject(parsed, name)
		if err != nil {
			return config.ProjectConfig{}, err
		}
		projectPath = project.Config
		registryVMName = project.VM.Name
	} else if len(parsed.positionals) > 0 {
		return config.ProjectConfig{}, errors.New("setup accepts either a project name or --project, not both")
	}

	loaded, err := config.Load(projectPath, workDir)
	if err != nil {
		return config.ProjectConfig{}, err
	}
	projectConfig, err := config.ProjectConfigFromMap(loaded.Config)
	if err != nil {
		return config.ProjectConfig{}, err
	}
	if registryVMName != "" {
		projectConfig.VMName = registryVMName
	}
	return projectConfig, nil
}

func resolvedRuntimeProject(parsed args, name string) (string, registry.Project, config.ProjectConfig, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return "", registry.Project{}, config.ProjectConfig{}, err
	}

	projectName, project, err := resolvedRegistryProject(parsed, name)
	if err != nil {
		return "", registry.Project{}, config.ProjectConfig{}, err
	}

	loaded, err := config.Load(project.Config, workDir)
	if err != nil {
		return "", registry.Project{}, config.ProjectConfig{}, err
	}
	projectConfig, err := config.ProjectConfigFromMap(loaded.Config)
	if err != nil {
		return "", registry.Project{}, config.ProjectConfig{}, err
	}
	if project.VM.Name != "" {
		projectConfig.VMName = project.VM.Name
	}
	return projectName, project, projectConfig, nil
}

func processActionTarget(positionals []string) (string, string, error) {
	switch len(positionals) {
	case 1:
		return "", positionals[0], nil
	case 2:
		return positionals[0], positionals[1], nil
	default:
		return "", "", errors.New("usage: process <start|stop> [project-name] <service-name>")
	}
}

func findService(services []config.ServiceConfig, name string) (config.ServiceConfig, error) {
	for _, service := range services {
		if service.Name == name {
			return service, nil
		}
	}
	return config.ServiceConfig{}, fmt.Errorf("unknown service: %s", name)
}

func findVMState(instances []lima.Instance, vmName string) string {
	for _, instance := range instances {
		if instance.Name == vmName {
			return instance.Status
		}
	}
	return "missing"
}

func processStatusFromVMState(vmState string) string {
	normalized := strings.ToLower(strings.TrimSpace(vmState))
	if normalized == "" {
		normalized = "unknown"
	}
	return "vm_" + normalized
}

func writeProcessRows(output io.Writer, rows []process.State, vmName string) error {
	writer := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "PROJECT\tSERVICE\tSTATUS\tPID\tPORT\tVM\tCOMMAND\tLOG")
	for _, row := range rows {
		fmt.Fprintf(
			writer,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			row.Project,
			row.Service,
			row.Status,
			row.PID,
			formatPort(row.Port),
			vmName,
			row.Command,
			row.Log,
		)
	}
	return writer.Flush()
}

func formatPort(port int) string {
	if port <= 0 {
		return "-"
	}
	return fmt.Sprint(port)
}

func buildStatusRows(reg registry.Registry, instances []lima.Instance, filter string) ([]statusRow, error) {
	byVM := map[string]lima.Instance{}
	for _, instance := range instances {
		byVM[instance.Name] = instance
	}

	names := reg.ProjectNames()
	if filter != "" {
		if _, ok := reg.Projects[filter]; !ok {
			return nil, fmt.Errorf("unknown project: %s", filter)
		}
		names = []string{filter}
	}

	rows := make([]statusRow, 0, len(names))
	for _, name := range names {
		project := reg.Projects[name]
		vmState := "missing"
		if instance, ok := byVM[project.VM.Name]; ok {
			vmState = instance.Status
		}
		rows = append(rows, statusRow{
			Current: reg.CurrentProject == name,
			Project: name,
			VM:      project.VM.Name,
			VMState: vmState,
			VMMode:  project.VM.Mode,
			Config:  project.Config,
			Path:    project.Path,
		})
	}
	return rows, nil
}

func writeStatusRows(output io.Writer, rows []statusRow) error {
	writer := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "CURRENT\tPROJECT\tVM\tVM_STATE\tVM_MODE\tCONFIG\tPATH")
	for _, row := range rows {
		current := ""
		if row.Current {
			current = "*"
		}
		fmt.Fprintf(
			writer,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			current,
			row.Project,
			row.VM,
			row.VMState,
			row.VMMode,
			row.Config,
			row.Path,
		)
	}
	return writer.Flush()
}

func resolvedRegistryPath(parsed args) (string, error) {
	if parsed.registryPath != "" {
		return parsed.registryPath, nil
	}
	return registry.DefaultPath()
}

func resolvedRegistryProject(parsed args, name string) (string, registry.Project, error) {
	path, err := resolvedRegistryPath(parsed)
	if err != nil {
		return "", registry.Project{}, err
	}
	reg, err := registry.Load(path)
	if err != nil {
		return "", registry.Project{}, err
	}
	return reg.Resolve(name)
}

func resolvedVMName(parsed args) (string, error) {
	if len(parsed.positionals) > 1 {
		return "", errors.New("usage: vm <list|status|start|stop|exec> [project-or-vm]")
	}

	target := ""
	if len(parsed.positionals) == 1 {
		target = parsed.positionals[0]
	}

	_, project, err := resolvedRegistryProject(parsed, target)
	if err == nil {
		return project.VM.Name, nil
	}
	if target != "" {
		return target, nil
	}
	return "", err
}

func formatBytes(value int64) string {
	if value <= 0 {
		return "-"
	}

	const gib = 1024 * 1024 * 1024
	if value%gib == 0 {
		return fmt.Sprintf("%dGiB", value/gib)
	}
	return fmt.Sprintf("%.1fGiB", float64(value)/float64(gib))
}

func printHelp() {
	fmt.Printf(`yard %s

Usage:
  go run ./cmd/yard --help
  go run ./cmd/yard config [project-name] [--project <path>]
  go run ./cmd/yard project add
  go run ./cmd/yard project add <name> <path> [--config <path>] [--vm-mode shared|dedicated] [--vm-name <name>]
  go run ./cmd/yard project list
  go run ./cmd/yard use <name>
  go run ./cmd/yard vm list
  go run ./cmd/yard vm status [project-or-vm]
  go run ./cmd/yard vm start [project-or-vm]
  go run ./cmd/yard vm stop [project-or-vm]
  go run ./cmd/yard vm exec [project-or-vm] -- <command>
  go run ./cmd/yard exec [project-name] -- <command>
  go run ./cmd/yard process list [project-name]
  go run ./cmd/yard process start [project-name] <service-name>
  go run ./cmd/yard process stop [project-name] <service-name>
  go run ./cmd/yard status [project-name]
  go run ./cmd/yard setup [project-name]

Commands:
  config   Print resolved project config as JSON.
  project  Manage the host project registry.
  use      Set the current project in the host project registry.
  vm       Manage Lima VMs.
  exec     Execute a command in the current or named project's VM.
  process  Manage configured dev service processes in the project VM.
  status   Show projects and VM state in a table.
  setup    Create the project VM if it does not exist.
`, version)
}
