package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"yard/internal/gitrepo"
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

func runProjectImport(parsed args) error {
	gitClient := gitrepo.NewClient(nil)
	fingerprinter := sshkeys.NewDetector(nil, "")
	return runProjectImportWithDeps(parsed, gitClient, fingerprinter, os.Stdout)
}

func runProjectImportWithDeps(parsed args, importer gitImporter, fingerprinter identityFingerprinter, output io.Writer) error {
	if len(parsed.positionals) != 1 {
		return errors.New("usage: project import <name> --repo <url> --identity <path> --path <path>")
	}
	if parsed.repoURL == "" {
		return errors.New("--repo is required")
	}
	if parsed.identityFile == "" {
		return errors.New("--identity is required")
	}
	if parsed.importPath == "" {
		return errors.New("--path is required")
	}

	projectName := parsed.positionals[0]
	destination, err := filepath.Abs(parsed.importPath)
	if err != nil {
		return err
	}
	identityFile, err := filepath.Abs(parsed.identityFile)
	if err != nil {
		return err
	}

	if err := ensureImportDestinationAvailable(destination); err != nil {
		return err
	}

	fingerprint, err := fingerprinter.FingerprintForIdentity(identityFile)
	if err != nil {
		return err
	}

	fmt.Fprintln(output, "Testing repository access...")
	if err := importer.TestAccess(parsed.repoURL, identityFile); err != nil {
		return err
	}

	fmt.Fprintln(output, "Cloning repository...")
	if err := importer.Clone(parsed.repoURL, identityFile, destination); err != nil {
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
	reg, err = reg.Add(projectName, registry.Project{
		Path:   destination,
		Config: parsed.configPath,
		Git: registry.Git{
			IdentityFile: identityFile,
			Fingerprint:  fingerprint,
		},
		VM: registry.VM{
			Mode: parsed.vmMode,
			Name: parsed.vmName,
		},
	})
	if err != nil {
		return err
	}
	if err := registry.Save(registryPath, reg); err != nil {
		return err
	}

	fmt.Fprintf(output, "imported project %s\n", projectName)
	return nil
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
