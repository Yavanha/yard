package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"devctl/internal/config"
	"devctl/internal/registry"
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
	default:
		return fmt.Errorf("unknown command: %s", parsed.command)
	}
}

func parseArgs(argv []string) (args, error) {
	parsed := args{}

	for index := 0; index < len(argv); index++ {
		value := argv[index]

		switch value {
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
				if parsed.subcommand == "" && parsed.command == "project" {
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

	loaded, err := config.Load(parsed.projectPath, workDir)
	if err != nil {
		return err
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(loaded)
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
	if len(parsed.positionals) != 2 {
		return errors.New("usage: project add <name> <path>")
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

func resolvedRegistryPath(parsed args) (string, error) {
	if parsed.registryPath != "" {
		return parsed.registryPath, nil
	}
	return registry.DefaultPath()
}

func printHelp() {
	fmt.Printf(`devctl-go %s

Usage:
  go run ./cmd/devctl --help
  go run ./cmd/devctl config [--project <path>]
  go run ./cmd/devctl project add <name> <path> [--config <path>] [--vm-mode shared|dedicated] [--vm-name <name>]
  go run ./cmd/devctl project list
  go run ./cmd/devctl use <name>

Commands:
  config   Print resolved project config as JSON.
  project  Manage the host project registry.
  use      Set the current project in the host project registry.
`, version)
}
