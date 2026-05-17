package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	Create(identityFile string, comment string) (sshkeys.KeyCandidate, error)
	PublicKey(identityFile string) (string, error)
}

type publicKeyUploader interface {
	Available() bool
	UploadPublicKey(publicKeyPath string, title string) error
}

type projectImportOptions struct {
	Name         string
	RepoURL      string
	Destination  string
	ConfigPath   string
	IdentityFile string
	Fingerprint  string
	RuntimeType  string
	VMMode       string
	VMName       string
}

var errNoPathBackedSSHKeys = errors.New("no path-backed SSH keys found. Run: yard ssh keys")

func runProjectImport(parsed args) error {
	gitClient := gitrepo.NewClient(nil)
	detector := sshkeys.NewDetector(nil, defaultSSHDir())
	if shouldRunProjectImportInteractive(parsed) {
		return runProjectImportInteractiveWithDepsAndUploader(parsed, gitClient, detector, ghKeyUploader{}, prompt.New(os.Stdin, os.Stdout))
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
	return runProjectImportInteractiveWithDepsAndUploader(parsed, importer, keys, ghKeyUploader{}, prompter)
}

func runProjectImportInteractiveWithDepsAndUploader(parsed args, importer gitImporter, keys importKeyProvider, uploader publicKeyUploader, prompter prompt.Prompter) error {
	sshKeyAvailability, err := askSSHKeyAvailability(prompter)
	if err != nil {
		return err
	}

	options, err := askProjectImportOptions(parsed, prompter)
	if err != nil {
		return err
	}

	var key sshkeys.KeyCandidate
	switch sshKeyAvailability {
	case "no":
		key, err = createImportSSHKey(prompter, keys, uploader, options.RepoURL)
		if err != nil {
			return err
		}
	case "yes":
		key, err = selectImportSSHKey(prompter, keys)
		if err != nil {
			return err
		}
	case "not sure":
		key, err = selectImportSSHKey(prompter, keys)
		if errors.Is(err, errNoPathBackedSSHKeys) {
			fmt.Fprintln(prompter.Writer(), "No existing SSH key could be selected.")
			key, err = createImportSSHKey(prompter, keys, uploader, options.RepoURL)
		}
		if err != nil {
			return err
		}
	}

	options.IdentityFile = key.Path
	options.Fingerprint = key.Fingerprint
	options, err = resolveProjectImportOptions(options, keys)
	if err != nil {
		return err
	}
	if sshKeyAvailability == "yes" || sshKeyAvailability == "not sure" {
		return executeProjectImportWithFallback(parsed, options, importer, keys, uploader, prompter)
	}
	return executeProjectImport(parsed, options, importer, prompter.Writer())
}

func executeProjectImportWithFallback(parsed args, options projectImportOptions, importer gitImporter, keys importKeyProvider, uploader publicKeyUploader, prompter prompt.Prompter) error {
	if err := ensureImportDestinationAvailable(options.Destination); err != nil {
		return err
	}

	if err := testProjectImportAccess(options, importer, prompter.Writer()); err == nil {
		return finishProjectImport(parsed, options, importer, prompter.Writer())
	} else {
		fmt.Fprintf(prompter.Writer(), "Selected SSH key did not work: %v\n", err)
	}

	createKey, err := prompter.Confirm("Create a new SSH key", true)
	if err != nil {
		return err
	}
	if !createKey {
		return errors.New("repository access test failed")
	}

	key, err := createImportSSHKey(prompter, keys, uploader, options.RepoURL)
	if err != nil {
		return err
	}
	options.IdentityFile = key.Path
	options.Fingerprint = key.Fingerprint
	options, err = resolveProjectImportOptions(options, keys)
	if err != nil {
		return err
	}
	return executeProjectImport(parsed, options, importer, prompter.Writer())
}

func askProjectImportOptions(parsed args, prompter prompt.Prompter) (projectImportOptions, error) {
	repoURL, err := prompter.Ask("Repository URL", parsed.repoURL, true)
	if err != nil {
		return projectImportOptions{}, err
	}
	defaultName := defaultProjectNameFromRepo(repoURL)
	name, err := prompter.Ask("Project alias", defaultName, true)
	if err != nil {
		return projectImportOptions{}, err
	}
	defaultPath := defaultImportPath(name)
	destination, err := prompter.Ask("Local path", defaultPath, true)
	if err != nil {
		return projectImportOptions{}, err
	}
	defaultConfig := filepath.Join(destination, config.FileName)
	configPath, err := prompter.Ask("Config path", defaultConfig, false)
	if err != nil {
		return projectImportOptions{}, err
	}

	runtimeType := parsed.runtimeType
	if runtimeType == "" {
		runtimeType, err = prompter.Ask("Runtime target (local-vm/remote-server)", registry.DefaultRuntimeType, true)
		if err != nil {
			return projectImportOptions{}, err
		}
	}
	runtimeType, err = resolvedProjectRuntimeType(runtimeType)
	if err != nil {
		return projectImportOptions{}, err
	}
	if runtimeType == registry.RuntimeTypeRemote && (parsed.vmMode != "" || parsed.vmName != "") {
		return projectImportOptions{}, errors.New("--vm-mode and --vm-name require --runtime local-vm")
	}

	vmMode := ""
	vmName := ""
	if runtimeType == registry.RuntimeTypeLocalVM {
		vmMode = parsed.vmMode
		if vmMode == "" {
			vmMode, err = prompter.Ask("VM mode (shared/dedicated)", registry.DefaultVMMode, true)
			if err != nil {
				return projectImportOptions{}, err
			}
		}
		if vmMode != "shared" && vmMode != "dedicated" {
			return projectImportOptions{}, fmt.Errorf("unsupported vm.mode: %s", vmMode)
		}

		defaultVMName := registry.DefaultVMName
		if vmMode == "dedicated" {
			defaultVMName = filepath.Base(destination) + "-dev"
		}
		vmName = parsed.vmName
		if vmName == "" {
			vmName, err = prompter.Ask("VM name", defaultVMName, true)
			if err != nil {
				return projectImportOptions{}, err
			}
		}
	}

	return projectImportOptions{
		Name:        name,
		RepoURL:     repoURL,
		Destination: destination,
		ConfigPath:  configPath,
		RuntimeType: runtimeType,
		VMMode:      vmMode,
		VMName:      vmName,
	}, nil
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
	candidates, err := keys.List()
	if err != nil {
		return sshkeys.KeyCandidate{}, err
	}
	selectable := pathBackedKeys(candidates)
	if len(selectable) == 0 {
		return sshkeys.KeyCandidate{}, errNoPathBackedSSHKeys
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

func createImportSSHKey(prompter prompt.Prompter, keys importKeyProvider, uploader publicKeyUploader, repoURL string) (sshkeys.KeyCandidate, error) {
	defaultPath := sshkeys.DefaultIdentityPath(defaultSSHDir(), repoURL)
	identityFile, err := prompter.Ask("SSH identity path", defaultPath, true)
	if err != nil {
		return sshkeys.KeyCandidate{}, err
	}
	identityFile, err = expandHomePath(identityFile)
	if err != nil {
		return sshkeys.KeyCandidate{}, err
	}
	identityFile, err = filepath.Abs(identityFile)
	if err != nil {
		return sshkeys.KeyCandidate{}, err
	}

	comment, err := prompter.Ask("SSH key comment", defaultSSHKeyComment(repoURL), true)
	if err != nil {
		return sshkeys.KeyCandidate{}, err
	}
	confirmed, err := prompter.Confirm("Create SSH key", true)
	if err != nil {
		return sshkeys.KeyCandidate{}, err
	}
	if !confirmed {
		return sshkeys.KeyCandidate{}, errors.New("Aborted.")
	}

	fmt.Fprintln(prompter.Writer(), "Running ssh-keygen. Enter a passphrase when prompted, or press Enter for none.")
	key, err := keys.Create(identityFile, comment)
	if err != nil {
		return sshkeys.KeyCandidate{}, err
	}

	publicKey, err := keys.PublicKey(key.Path)
	if err != nil {
		return sshkeys.KeyCandidate{}, err
	}
	fmt.Fprintln(prompter.Writer())
	fmt.Fprintln(prompter.Writer(), "Public key:")
	fmt.Fprintln(prompter.Writer(), publicKey)
	fmt.Fprintln(prompter.Writer())

	publicKeyPath := key.Path + ".pub"
	title := defaultSSHKeyTitle(repoURL)
	if uploader.Available() {
		upload, err := prompter.Confirm("Upload public key with gh", false)
		if err != nil {
			return sshkeys.KeyCandidate{}, err
		}
		if upload {
			if err := uploader.UploadPublicKey(publicKeyPath, title); err != nil {
				return sshkeys.KeyCandidate{}, err
			}
			return key, nil
		}
	}

	fmt.Fprintln(prompter.Writer(), "Add this public key to GitHub, GitLab, or your Git provider before continuing.")
	if _, err := prompter.Ask("Press Enter after adding the public key", "", false); err != nil {
		return sshkeys.KeyCandidate{}, err
	}
	return key, nil
}

func defaultSSHKeyComment(repoURL string) string {
	return "yard " + defaultProjectNameFromRepo(repoURL)
}

func defaultSSHKeyTitle(repoURL string) string {
	return "yard " + defaultProjectNameFromRepo(repoURL)
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
		RuntimeType:  parsed.runtimeType,
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

	runtimeType, err := resolvedProjectRuntimeType(options.RuntimeType)
	if err != nil {
		return projectImportOptions{}, err
	}
	options.RuntimeType = runtimeType
	if options.RuntimeType == registry.RuntimeTypeRemote && (options.VMMode != "" || options.VMName != "") {
		return projectImportOptions{}, errors.New("--vm-mode and --vm-name require --runtime local-vm")
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

	if err := testProjectImportAccess(options, importer, output); err != nil {
		return err
	}
	return finishProjectImport(parsed, options, importer, output)
}

func testProjectImportAccess(options projectImportOptions, importer gitImporter, output io.Writer) error {
	fmt.Fprintln(output, "Testing repository access...")
	if err := importer.TestAccess(options.RepoURL, options.IdentityFile); err != nil {
		return err
	}
	return nil
}

func finishProjectImport(parsed args, options projectImportOptions, importer gitImporter, output io.Writer) error {
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
		Path:    options.Destination,
		Config:  options.ConfigPath,
		Runtime: registry.RuntimeTarget{Type: options.RuntimeType},
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

type ghKeyUploader struct{}

func (ghKeyUploader) Available() bool {
	_, err := exec.LookPath("gh")
	return err == nil
}

func (ghKeyUploader) UploadPublicKey(publicKeyPath string, title string) error {
	cmd := exec.Command("gh", "ssh-key", "add", publicKeyPath, "--title", title)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
