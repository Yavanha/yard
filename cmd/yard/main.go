package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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
	importPath   string
	registryPath string
	configPath   string
	repoURL      string
	repoDir      string
	identityFile string
	vmMode       string
	vmName       string
	vmProvider   string
	vmUser       string
	vmType       string
	cpus         int
	memory       string
	disk         string
	serviceName  string
	serviceCmd   string
	serviceDir   string
	servicePort  int
	tailLines    int
	follow       bool
	yes          bool
	force        bool
	stopVM       bool
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
	case "init":
		return runInit(parsed)
	case "vm":
		return runVM(parsed)
	case "ssh":
		return runSSH(parsed)
	case "exec":
		return runExec(parsed)
	case "process":
		return runProcess(parsed)
	case "start":
		return runStart(parsed)
	case "stop":
		return runStop(parsed)
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
		case "--path":
			if index+1 >= len(argv) {
				return args{}, errors.New("--path requires a path")
			}
			parsed.importPath = argv[index+1]
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
		case "--repo":
			if index+1 >= len(argv) {
				return args{}, errors.New("--repo requires a value")
			}
			parsed.repoURL = argv[index+1]
			index++
		case "--repo-dir":
			if index+1 >= len(argv) {
				return args{}, errors.New("--repo-dir requires a path")
			}
			parsed.repoDir = argv[index+1]
			index++
		case "--identity":
			if index+1 >= len(argv) {
				return args{}, errors.New("--identity requires a path")
			}
			parsed.identityFile = argv[index+1]
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
		case "--vm-provider":
			if index+1 >= len(argv) {
				return args{}, errors.New("--vm-provider requires a value")
			}
			parsed.vmProvider = argv[index+1]
			index++
		case "--vm-user":
			if index+1 >= len(argv) {
				return args{}, errors.New("--vm-user requires a value")
			}
			parsed.vmUser = argv[index+1]
			index++
		case "--vm-type":
			if index+1 >= len(argv) {
				return args{}, errors.New("--vm-type requires a value")
			}
			parsed.vmType = argv[index+1]
			index++
		case "--cpus":
			if index+1 >= len(argv) {
				return args{}, errors.New("--cpus requires a value")
			}
			cpus, err := strconv.Atoi(argv[index+1])
			if err != nil || cpus <= 0 {
				return args{}, errors.New("--cpus requires a positive integer")
			}
			parsed.cpus = cpus
			index++
		case "--memory":
			if index+1 >= len(argv) {
				return args{}, errors.New("--memory requires a value")
			}
			parsed.memory = argv[index+1]
			index++
		case "--disk":
			if index+1 >= len(argv) {
				return args{}, errors.New("--disk requires a value")
			}
			parsed.disk = argv[index+1]
			index++
		case "--service":
			if index+1 >= len(argv) {
				return args{}, errors.New("--service requires a value")
			}
			parsed.serviceName = argv[index+1]
			index++
		case "--command":
			if index+1 >= len(argv) {
				return args{}, errors.New("--command requires a value")
			}
			parsed.serviceCmd = argv[index+1]
			index++
		case "--workdir":
			if index+1 >= len(argv) {
				return args{}, errors.New("--workdir requires a value")
			}
			parsed.serviceDir = argv[index+1]
			index++
		case "--port":
			if index+1 >= len(argv) {
				return args{}, errors.New("--port requires a value")
			}
			port, err := strconv.Atoi(argv[index+1])
			if err != nil || port <= 0 || port > 65535 {
				return args{}, errors.New("--port requires an integer between 1 and 65535")
			}
			parsed.servicePort = port
			index++
		case "--tail":
			if index+1 >= len(argv) {
				return args{}, errors.New("--tail requires a value")
			}
			tailLines, err := strconv.Atoi(argv[index+1])
			if err != nil || tailLines <= 0 {
				return args{}, errors.New("--tail requires a positive integer")
			}
			parsed.tailLines = tailLines
			index++
		case "--follow", "-f":
			parsed.follow = true
		case "--vm":
			parsed.stopVM = true
		case "--yes", "-y":
			parsed.yes = true
		case "--force":
			parsed.force = true
		default:
			if len(value) > 0 && value[0] == '-' {
				return args{}, fmt.Errorf("unknown flag: %s", value)
			}
			if parsed.command != "" {
				if parsed.subcommand == "" && (parsed.command == "project" || parsed.command == "vm" || parsed.command == "process" || parsed.command == "ssh") {
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
	case "import":
		return runProjectImport(parsed)
	case "use":
		return runUse(args{
			positionals:  parsed.positionals,
			registryPath: parsed.registryPath,
		})
	default:
		if parsed.subcommand == "" {
			return errors.New("project requires a subcommand: add, import, list, or use")
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
	case "logs":
		return runProcessLogs(parsed)
	default:
		if parsed.subcommand == "" {
			return errors.New("process requires a subcommand: list, start, stop, or logs")
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

func runProcessLogs(parsed args) error {
	projectName, serviceName, err := processActionTarget(parsed.positionals)
	if err != nil {
		return err
	}
	if parsed.projectPath != "" {
		return errors.New("process logs uses the project registry; --project is not supported")
	}

	projectName, project, projectConfig, err := resolvedRuntimeProject(parsed, projectName)
	if err != nil {
		return err
	}
	if _, err := findService(projectConfig.Services, serviceName); err != nil {
		return err
	}

	command, err := process.LogsCommand(projectName, serviceName, parsed.tailLines, parsed.follow)
	if err != nil {
		return err
	}

	client := lima.NewClient(nil)
	return client.Exec(project.VM.Name, command)
}

func runStart(parsed args) error {
	if len(parsed.positionals) > 1 {
		return errors.New("usage: start [project-name]")
	}
	if parsed.projectPath != "" {
		return errors.New("start uses the project registry; --project is not supported")
	}

	projectName := ""
	if len(parsed.positionals) == 1 {
		projectName = parsed.positionals[0]
	}
	projectName, project, projectConfig, err := resolvedRuntimeProject(parsed, projectName)
	if err != nil {
		return err
	}
	if projectConfig.VM.Provider != "auto" && projectConfig.VM.Provider != "lima" {
		return fmt.Errorf("unsupported VM provider for start: %s", projectConfig.VM.Provider)
	}

	client := lima.NewClient(nil)
	if err := ensureProjectVM(client, projectConfig); err != nil {
		return err
	}
	return startProjectServices(client, projectName, project, projectConfig)
}

func runStop(parsed args) error {
	if len(parsed.positionals) > 1 {
		return errors.New("usage: stop [project-name] [--vm]")
	}
	if parsed.projectPath != "" {
		return errors.New("stop uses the project registry; --project is not supported")
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
	if err := stopProjectServices(client, projectName, project, projectConfig); err != nil {
		return err
	}
	if !shouldStopProjectVM(project, parsed.stopVM) {
		return reportVMLeftRunning(client, project)
	}
	return stopProjectVM(client, project)
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

func ensureProjectVM(client lima.Client, projectConfig config.ProjectConfig) error {
	result, err := client.Setup(projectConfig)
	if err != nil {
		return err
	}
	if result.Created {
		fmt.Printf("VM created: %s\n", result.VMName)
		return nil
	}

	instance, err := client.Status(projectConfig.VMName)
	if err != nil {
		return err
	}
	if strings.EqualFold(instance.Status, "Running") {
		fmt.Printf("VM already running: %s\n", projectConfig.VMName)
		return nil
	}
	if err := client.Start(projectConfig.VMName); err != nil {
		return err
	}
	fmt.Printf("VM started: %s\n", projectConfig.VMName)
	return nil
}

func startProjectServices(client lima.Client, projectName string, project registry.Project, projectConfig config.ProjectConfig) error {
	for _, service := range projectConfig.Services {
		command, err := process.StartCommand(process.ServiceFromConfig(projectName, projectConfig, service))
		if err != nil {
			return err
		}
		fmt.Printf("Starting service: %s\n", service.Name)
		if err := client.Exec(project.VM.Name, command); err != nil {
			return err
		}
	}
	return nil
}

func stopProjectServices(client lima.Client, projectName string, project registry.Project, projectConfig config.ProjectConfig) error {
	instances, err := client.List()
	if err != nil {
		return err
	}
	vmState := findVMState(instances, project.VM.Name)
	if !strings.EqualFold(vmState, "Running") {
		fmt.Printf("VM not running: %s\n", project.VM.Name)
		return nil
	}

	for index := len(projectConfig.Services) - 1; index >= 0; index-- {
		service := projectConfig.Services[index]
		command, err := process.StopCommand(projectName, service.Name)
		if err != nil {
			return err
		}
		fmt.Printf("Stopping service: %s\n", service.Name)
		if err := client.Exec(project.VM.Name, command); err != nil {
			return err
		}
	}
	return nil
}

func stopProjectVM(client lima.Client, project registry.Project) error {
	instances, err := client.List()
	if err != nil {
		return err
	}
	vmState := findVMState(instances, project.VM.Name)
	if vmState == "missing" {
		fmt.Printf("VM missing: %s\n", project.VM.Name)
		return nil
	}
	if strings.EqualFold(vmState, "Stopped") {
		fmt.Printf("VM already stopped: %s\n", project.VM.Name)
		return nil
	}
	if err := client.Stop(project.VM.Name); err != nil {
		return err
	}
	fmt.Printf("VM stopped: %s\n", project.VM.Name)
	return nil
}

func reportVMLeftRunning(client lima.Client, project registry.Project) error {
	instances, err := client.List()
	if err != nil {
		return err
	}
	if strings.EqualFold(findVMState(instances, project.VM.Name), "Running") {
		fmt.Printf("VM left running: %s (%s)\n", project.VM.Name, project.VM.Mode)
	}
	return nil
}

func shouldStopProjectVM(project registry.Project, force bool) bool {
	return force || project.VM.Mode == "dedicated"
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
  go run ./cmd/yard project import
  go run ./cmd/yard project import <name> --repo <url> --identity <path> --path <path>
  go run ./cmd/yard project list
  go run ./cmd/yard use <name>
  go run ./cmd/yard init [project-name] [--yes] [--force]
  go run ./cmd/yard vm list
  go run ./cmd/yard vm status [project-or-vm]
  go run ./cmd/yard vm start [project-or-vm]
  go run ./cmd/yard vm stop [project-or-vm]
  go run ./cmd/yard vm exec [project-or-vm] -- <command>
  go run ./cmd/yard ssh keys
  go run ./cmd/yard exec [project-name] -- <command>
  go run ./cmd/yard process list [project-name]
  go run ./cmd/yard process start [project-name] <service-name>
  go run ./cmd/yard process stop [project-name] <service-name>
  go run ./cmd/yard process logs [project-name] <service-name> [--tail <lines>] [--follow]
  go run ./cmd/yard start [project-name]
  go run ./cmd/yard stop [project-name] [--vm]
  go run ./cmd/yard status [project-name]
  go run ./cmd/yard setup [project-name]

Commands:
  config   Print resolved project config as JSON.
  project  Manage the host project registry.
  use      Set the current project in the host project registry.
  init     Create a project .devctl.yml.
  vm       Manage Lima VMs.
  ssh      Inspect host SSH keys for Git onboarding.
  exec     Execute a command in the current or named project's VM.
  process  Manage configured dev service processes in the project VM.
  start    Start the project VM and configured services.
  stop     Stop configured services, and dedicated VMs by default.
  status   Show projects and VM state in a table.
  setup    Create the project VM if it does not exist.
`, version)
}
