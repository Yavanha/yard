package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"yard/internal/config"
	"yard/internal/gitrepo"
	"yard/internal/prompt"
	"yard/internal/registry"
	"yard/internal/sshkeys"
)

type gitImporter interface {
	TestAccess(repoURL string, identityFile string) error
	Clone(repoURL string, identityFile string, destination string) error
}

type identityFingerprinter interface {
	FingerprintForIdentity(identityFile string) (string, error)
}

type importKeyProvider interface {
	identityFingerprinter
	List() ([]sshkeys.KeyCandidate, error)
}

type projectImportOptions struct {
	Name         string
	RepoURL      string
	Destination  string
	ConfigPath   string
	IdentityFile string
	Fingerprint  string
	VMMode       string
	VMName       string
}

func runProjectImport(parsed args) error {
	gitClient := gitrepo.NewClient(nil)
	detector := sshkeys.NewDetector(nil, defaultSSHDir())
	if shouldRunProjectImportInteractive(parsed) {
		return runProjectImportInteractiveWithDeps(parsed, gitClient, detector, prompt.New(os.Stdin, os.Stdout))
	}
	return runProjectImportWithDeps(parsed, gitClient, detector, os.Stdout)
}

func shouldRunProjectImportInteractive(parsed args) bool {
	return len(parsed.positionals) == 0 &&
		parsed.repoURL == "" &&
		parsed.identityFile == "" &&
		parsed.importPath == ""
}

func defaultSSHDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ssh")
}

func runProjectImportInteractiveWithDeps(parsed args, importer gitImporter, keys importKeyProvider, prompter prompt.Prompter) error {
	key, err := selectImportSSHKey(prompter, keys)
	if err != nil {
		return err
	}

	repoURL, err := prompter.Ask("Repository URL", parsed.repoURL, true)
	if err != nil {
		return err
	}
	defaultName := defaultProjectNameFromRepo(repoURL)
	name, err := prompter.Ask("Project alias", defaultName, true)
	if err != nil {
		return err
	}
	defaultPath := defaultImportPath(name)
	destination, err := prompter.Ask("Local path", defaultPath, true)
	if err != nil {
		return err
	}
	defaultConfig := filepath.Join(destination, config.FileName)
	configPath, err := prompter.Ask("Config path", defaultConfig, false)
	if err != nil {
		return err
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
		defaultVMName = filepath.Base(destination) + "-dev"
	}
	vmName := parsed.vmName
	if vmName == "" {
		vmName, err = prompter.Ask("VM name", defaultVMName, true)
		if err != nil {
			return err
		}
	}

	options, err := resolveProjectImportOptions(projectImportOptions{
		Name:         name,
		RepoURL:      repoURL,
		Destination:  destination,
		ConfigPath:   configPath,
		IdentityFile: key.Path,
		Fingerprint:  key.Fingerprint,
		VMMode:       vmMode,
		VMName:       vmName,
	}, keys)
	if err != nil {
		return err
	}
	return executeProjectImport(parsed, options, importer, prompter.Writer())
}

func defaultProjectNameFromRepo(repoURL string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(repoURL), "/")
	name := filepath.Base(trimmed)
	name = strings.TrimSuffix(name, ".git")
	if name == "" || name == "." || name == "/" {
		return "project"
	}
	return name
}

func defaultImportPath(name string) string {
	workDir, err := os.Getwd()
	if err != nil {
		return name
	}
	return filepath.Join(workDir, name)
}

func selectImportSSHKey(prompter prompt.Prompter, keys importKeyProvider) (sshkeys.KeyCandidate, error) {
	answer, err := askSSHKeyAvailability(prompter)
	if err != nil {
		return sshkeys.KeyCandidate{}, err
	}
	if answer == "no" {
		return sshkeys.KeyCandidate{}, errors.New("SSH key creation is not implemented yet. Create a host SSH key, upload it to your Git provider, then run: yard project import")
	}

	candidates, err := keys.List()
	if err != nil {
		return sshkeys.KeyCandidate{}, err
	}
	selectable := pathBackedKeys(candidates)
	if len(selectable) == 0 {
		return sshkeys.KeyCandidate{}, errors.New("no path-backed SSH keys found. Run: yard ssh keys")
	}

	fmt.Fprintln(prompter.Writer(), "Available SSH keys:")
	for index, key := range selectable {
		fmt.Fprintf(
			prompter.Writer(),
			"  %d) %s  %s  %s  agent=%s\n",
			index+1,
			key.Fingerprint,
			key.Path,
			formatEmpty(key.Comment),
			formatBool(key.InAgent),
		)
	}

	for {
		value, err := prompter.Ask("SSH key number", "1", true)
		if err != nil {
			return sshkeys.KeyCandidate{}, err
		}
		index, err := strconv.Atoi(value)
		if err != nil || index < 1 || index > len(selectable) {
			fmt.Fprintln(prompter.Writer(), "Choose a listed key number.")
			continue
		}
		return selectable[index-1], nil
	}
}

func askSSHKeyAvailability(prompter prompt.Prompter) (string, error) {
	for {
		answer, err := prompter.Ask("Do you already have an SSH key for this repository or organization? (yes/no/not sure)", "not sure", true)
		if err != nil {
			return "", err
		}
		normalized := strings.ToLower(strings.TrimSpace(answer))
		switch normalized {
		case "yes", "no", "not sure":
			return normalized, nil
		default:
			fmt.Fprintln(prompter.Writer(), "Answer yes, no, or not sure.")
		}
	}
}

func pathBackedKeys(keys []sshkeys.KeyCandidate) []sshkeys.KeyCandidate {
	filtered := make([]sshkeys.KeyCandidate, 0, len(keys))
	for _, key := range keys {
		if key.Path != "" {
			filtered = append(filtered, key)
		}
	}
	return filtered
}

func runProjectImportWithDeps(parsed args, importer gitImporter, fingerprinter identityFingerprinter, output io.Writer) error {
	if len(parsed.positionals) != 1 {
		return errors.New("usage: project import <name> --repo <url> --identity <path> --path <path>")
	}
	options, err := resolveProjectImportOptions(projectImportOptions{
		Name:         parsed.positionals[0],
		RepoURL:      parsed.repoURL,
		Destination:  parsed.importPath,
		ConfigPath:   parsed.configPath,
		IdentityFile: parsed.identityFile,
		VMMode:       parsed.vmMode,
		VMName:       parsed.vmName,
	}, fingerprinter)
	if err != nil {
		return err
	}
	return executeProjectImport(parsed, options, importer, output)
}

func resolveProjectImportOptions(options projectImportOptions, fingerprinter identityFingerprinter) (projectImportOptions, error) {
	if options.Name == "" {
		return projectImportOptions{}, errors.New("project name is required")
	}
	if options.RepoURL == "" {
		return projectImportOptions{}, errors.New("--repo is required")
	}
	if options.IdentityFile == "" {
		return projectImportOptions{}, errors.New("--identity is required")
	}
	if options.Destination == "" {
		return projectImportOptions{}, errors.New("--path is required")
	}

	destination, err := expandHomePath(options.Destination)
	if err != nil {
		return projectImportOptions{}, err
	}
	options.Destination, err = filepath.Abs(destination)
	if err != nil {
		return projectImportOptions{}, err
	}

	identityFile, err := expandHomePath(options.IdentityFile)
	if err != nil {
		return projectImportOptions{}, err
	}
	options.IdentityFile, err = filepath.Abs(identityFile)
	if err != nil {
		return projectImportOptions{}, err
	}

	if options.ConfigPath != "" {
		configPath, err := expandHomePath(options.ConfigPath)
		if err != nil {
			return projectImportOptions{}, err
		}
		options.ConfigPath, err = filepath.Abs(configPath)
		if err != nil {
			return projectImportOptions{}, err
		}
	}

	if options.Fingerprint == "" {
		fingerprint, err := fingerprinter.FingerprintForIdentity(options.IdentityFile)
		if err != nil {
			return projectImportOptions{}, err
		}
		options.Fingerprint = fingerprint
	}
	return options, nil
}

func executeProjectImport(parsed args, options projectImportOptions, importer gitImporter, output io.Writer) error {
	if err := ensureImportDestinationAvailable(options.Destination); err != nil {
		return err
	}

	fmt.Fprintln(output, "Testing repository access...")
	if err := importer.TestAccess(options.RepoURL, options.IdentityFile); err != nil {
		return err
	}

	fmt.Fprintln(output, "Cloning repository...")
	if err := importer.Clone(options.RepoURL, options.IdentityFile, options.Destination); err != nil {
		return err
	}

	registryPath, err := resolvedRegistryPath(parsed)
	if err != nil {
		return err
	}
	reg, err := registry.Load(registryPath)
	if err != nil {
		return err
	}
	reg, err = reg.Add(options.Name, registry.Project{
		Path:   options.Destination,
		Config: options.ConfigPath,
		Git: registry.Git{
			IdentityFile: options.IdentityFile,
			Fingerprint:  options.Fingerprint,
		},
		VM: registry.VM{
			Mode: options.VMMode,
			Name: options.VMName,
		},
	})
	if err != nil {
		return err
	}
	if err := registry.Save(registryPath, reg); err != nil {
		return err
	}

	fmt.Fprintf(output, "imported project %s\n", options.Name)
	return nil
}

func expandHomePath(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return home, nil
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}
	return path, nil
}

func ensureImportDestinationAvailable(destination string) error {
	entries, err := os.ReadDir(destination)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("destination path is not empty: %s", destination)
	}
	return nil
}
