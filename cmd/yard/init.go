package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"yard/internal/config"
	"yard/internal/prompt"
)

func runInit(parsed args) error {
	if len(parsed.positionals) > 1 {
		return errors.New("usage: init [project-name] [--yes] [--force]")
	}
	if parsed.projectPath != "" {
		return errors.New("init writes a project config; use --config to choose the output path")
	}

	target, err := targetConfigPath(parsed)
	if err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil && !parsed.force {
		return fmt.Errorf("%s already exists. Pass --force to overwrite", target)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	options, err := scaffoldOptionsFromArgs(parsed)
	if err != nil {
		return err
	}
	if parsed.yes {
		return writeScaffold(target, options)
	}
	return runInitInteractive(target, options, prompt.New(os.Stdin, os.Stdout))
}

func runInitInteractive(target string, options config.ScaffoldOptions, prompter prompt.Prompter) error {
	var err error
	options = options.Normalized()
	initial := options

	options.Project, err = prompter.Ask("Project name", options.Project, true)
	if err != nil {
		return err
	}
	if options.VMName == initial.VMName {
		options.VMName = options.Project + "-dev"
	}
	if options.RepoDir == initial.RepoDir {
		options.RepoDir = "/home/ubuntu/workspaces/" + options.Project
	}

	options.Repo, err = prompter.Ask("Repository URL", options.Repo, false)
	if err != nil {
		return err
	}
	options.VMName, err = prompter.Ask("VM name", options.VMName, true)
	if err != nil {
		return err
	}
	options.VMUser, err = prompter.Ask("VM user", options.VMUser, true)
	if err != nil {
		return err
	}
	options.RepoDir, err = prompter.Ask("Repo dir in VM", options.RepoDir, true)
	if err != nil {
		return err
	}
	options.VMProvider, err = prompter.Ask("VM provider", options.VMProvider, true)
	if err != nil {
		return err
	}
	options.VMType, err = prompter.Ask("VM type", options.VMType, true)
	if err != nil {
		return err
	}
	options.CPUs, err = askPositiveInt(prompter, "CPUs", options.CPUs)
	if err != nil {
		return err
	}
	options.Memory, err = prompter.Ask("Memory", options.Memory, true)
	if err != nil {
		return err
	}
	options.Disk, err = prompter.Ask("Disk", options.Disk, true)
	if err != nil {
		return err
	}
	options.ServiceName, err = prompter.Ask("Service name", options.ServiceName, true)
	if err != nil {
		return err
	}
	options.ServiceCommand, err = prompter.Ask("Service command", options.ServiceCommand, true)
	if err != nil {
		return err
	}
	options.ServiceWorkdir, err = prompter.Ask("Service workdir", options.ServiceWorkdir, true)
	if err != nil {
		return err
	}
	options.ServicePort, err = askPositiveInt(prompter, "Service port", options.ServicePort)
	if err != nil {
		return err
	}

	content, err := config.RenderScaffold(options)
	if err != nil {
		return err
	}

	fmt.Fprintln(prompter.Writer())
	fmt.Fprintf(prompter.Writer(), "Config preview: %s\n", target)
	fmt.Fprint(prompter.Writer(), string(content))

	confirmed, err := prompter.Confirm("Write config", true)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Fprintln(prompter.Writer(), "Aborted.")
		return nil
	}

	return writeScaffoldContent(target, content)
}

func scaffoldOptionsFromArgs(parsed args) (config.ScaffoldOptions, error) {
	projectName := ""
	if len(parsed.positionals) == 1 {
		projectName = parsed.positionals[0]
	} else {
		current, err := os.Getwd()
		if err != nil {
			return config.ScaffoldOptions{}, err
		}
		projectName = filepath.Base(current)
	}

	options := config.DefaultScaffoldOptions(projectName)
	options.Repo = detectGitRemote()
	if parsed.repoURL != "" {
		options.Repo = parsed.repoURL
	}
	if parsed.repoDir != "" {
		options.RepoDir = parsed.repoDir
	}
	if parsed.vmName != "" {
		options.VMName = parsed.vmName
	}
	if parsed.vmProvider != "" {
		options.VMProvider = parsed.vmProvider
	}
	if parsed.vmUser != "" {
		options.VMUser = parsed.vmUser
	}
	if parsed.vmType != "" {
		options.VMType = parsed.vmType
	}
	if parsed.cpus != 0 {
		options.CPUs = parsed.cpus
	}
	if parsed.memory != "" {
		options.Memory = parsed.memory
	}
	if parsed.disk != "" {
		options.Disk = parsed.disk
	}
	if parsed.serviceName != "" {
		options.ServiceName = parsed.serviceName
	}
	if parsed.serviceCmd != "" {
		options.ServiceCommand = parsed.serviceCmd
	}
	if parsed.serviceDir != "" {
		options.ServiceWorkdir = parsed.serviceDir
	}
	if parsed.servicePort != 0 {
		options.ServicePort = parsed.servicePort
	}

	return options.Normalized(), nil
}

func targetConfigPath(parsed args) (string, error) {
	target := parsed.configPath
	if target == "" {
		current, err := os.Getwd()
		if err != nil {
			return "", err
		}
		target = filepath.Join(current, config.FileName)
	}
	if err := config.RejectLegacyConfigPath(target); err != nil {
		return "", err
	}
	return filepath.Abs(target)
}

func writeScaffold(target string, options config.ScaffoldOptions) error {
	content, err := config.RenderScaffold(options)
	if err != nil {
		return err
	}
	return writeScaffoldContent(target, content)
}

func writeScaffoldContent(target string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(target, content, 0o644); err != nil {
		return err
	}
	fmt.Printf("Created %s\n", target)
	return nil
}

func askPositiveInt(prompter prompt.Prompter, label string, defaultValue int) (int, error) {
	for {
		answer, err := prompter.Ask(label, fmt.Sprint(defaultValue), true)
		if err != nil {
			return 0, err
		}
		parsed, err := strconv.Atoi(answer)
		if err == nil && parsed > 0 {
			return parsed, nil
		}
		fmt.Fprintln(prompter.Writer(), "Value must be a positive integer.")
	}
}

func detectGitRemote() string {
	command := exec.Command("git", "config", "--get", "remote.origin.url")
	output, err := command.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
