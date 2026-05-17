package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"

	"yard/internal/config"
	"yard/internal/prompt"
	"yard/internal/registry"
)

func runProject(parsed args) error {
	switch parsed.subcommand {
	case "add":
		return runProjectAdd(parsed)
	case "list":
		return runProjectList(parsed)
	case "import":
		return runProjectImport(parsed)
	case "inspect":
		return runProjectInspect(parsed, os.Stdout)
	case "remove":
		return runProjectRemove(parsed)
	case "use":
		return runUse(args{
			positionals:  parsed.positionals,
			registryPath: parsed.registryPath,
		})
	default:
		if parsed.subcommand == "" {
			return errors.New("project requires a subcommand: add, import, inspect, list, remove, or use")
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

	runtimeType, err := resolvedProjectRuntimeType(parsed.runtimeType)
	if err != nil {
		return err
	}
	if runtimeType == registry.RuntimeTypeRemote && (parsed.vmMode != "" || parsed.vmName != "") {
		return errors.New("--vm-mode and --vm-name require --runtime local-vm")
	}
	vm := registry.VM{}
	if runtimeType == registry.RuntimeTypeLocalVM {
		vm = registry.VM{
			Mode: parsed.vmMode,
			Name: parsed.vmName,
		}
	}

	reg, err = reg.Add(parsed.positionals[0], registry.Project{
		Path:    parsed.positionals[1],
		Config:  parsed.configPath,
		Runtime: registry.RuntimeTarget{Type: runtimeType},
		VM:      vm,
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

	runtimeType := parsed.runtimeType
	if runtimeType == "" {
		runtimeType, err = prompter.Ask("Runtime target (local-vm/remote-server)", registry.DefaultRuntimeType, true)
		if err != nil {
			return err
		}
	}
	runtimeType, err = resolvedProjectRuntimeType(runtimeType)
	if err != nil {
		return err
	}
	if runtimeType == registry.RuntimeTypeRemote && (parsed.vmMode != "" || parsed.vmName != "") {
		return errors.New("--vm-mode and --vm-name require --runtime local-vm")
	}

	vm := registry.VM{}
	if runtimeType == registry.RuntimeTypeLocalVM {
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
		vm = registry.VM{
			Mode: vmMode,
			Name: vmName,
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
		Path:    absProjectPath,
		Config:  configPath,
		Runtime: registry.RuntimeTarget{Type: runtimeType},
		VM:      vm,
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

func resolvedProjectRuntimeType(runtimeType string) (string, error) {
	if runtimeType == "" {
		return registry.DefaultRuntimeType, nil
	}
	if runtimeType != registry.RuntimeTypeLocalVM && runtimeType != registry.RuntimeTypeRemote {
		return "", fmt.Errorf("unsupported runtime.type: %s", runtimeType)
	}
	return runtimeType, nil
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
	fmt.Fprintln(writer, "CURRENT\tNAME\tRUNTIME\tPATH\tVM_MODE\tVM_NAME")
	for _, name := range reg.ProjectNames() {
		project := reg.Projects[name]
		current := ""
		if reg.CurrentProject == name {
			current = "*"
		}
		fmt.Fprintf(
			writer,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			current,
			name,
			project.Runtime.Type,
			project.Path,
			formatEmpty(project.VM.Mode),
			formatEmpty(project.VM.Name),
		)
	}
	return writer.Flush()
}

func runProjectInspect(parsed args, output io.Writer) error {
	if len(parsed.positionals) > 1 {
		return errors.New("usage: project inspect [name]")
	}

	path, err := resolvedRegistryPath(parsed)
	if err != nil {
		return err
	}
	reg, err := registry.Load(path)
	if err != nil {
		return err
	}

	name := ""
	if len(parsed.positionals) == 1 {
		name = parsed.positionals[0]
	}
	resolvedName, project, err := reg.Resolve(name)
	if err != nil {
		return err
	}

	return writeProjectInspect(output, reg.CurrentProject, resolvedName, project)
}

func runProjectRemove(parsed args) error {
	if len(parsed.positionals) != 1 {
		return errors.New("usage: project remove <name>")
	}

	path, err := resolvedRegistryPath(parsed)
	if err != nil {
		return err
	}
	reg, err := registry.Load(path)
	if err != nil {
		return err
	}

	reg, err = reg.Remove(parsed.positionals[0])
	if err != nil {
		return err
	}
	if err := registry.Save(path, reg); err != nil {
		return err
	}

	fmt.Printf("removed project %s\n", parsed.positionals[0])
	return nil
}

func writeProjectInspect(output io.Writer, currentProject string, name string, project registry.Project) error {
	current := "no"
	if currentProject == name {
		current = "yes"
	}

	writer := tabwriter.NewWriter(output, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "FIELD\tVALUE")
	fmt.Fprintf(writer, "name\t%s\n", name)
	fmt.Fprintf(writer, "current\t%s\n", current)
	fmt.Fprintf(writer, "path\t%s\n", project.Path)
	fmt.Fprintf(writer, "config\t%s\n", formatEmpty(project.Config))
	fmt.Fprintf(writer, "runtime.type\t%s\n", formatEmpty(project.Runtime.Type))
	fmt.Fprintf(writer, "vm.mode\t%s\n", formatEmpty(project.VM.Mode))
	fmt.Fprintf(writer, "vm.name\t%s\n", formatEmpty(project.VM.Name))
	fmt.Fprintf(writer, "git.identity_file\t%s\n", formatEmpty(project.Git.IdentityFile))
	fmt.Fprintf(writer, "git.fingerprint\t%s\n", formatEmpty(project.Git.Fingerprint))
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
