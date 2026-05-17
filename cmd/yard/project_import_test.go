package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yard/internal/registry"
)

func TestParseProjectImportArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseArgs([]string{
		"project",
		"import",
		"api",
		"--repo",
		"git@github.com:acme/api.git",
		"--identity",
		"/Users/me/.ssh/yard_acme",
		"--path",
		"/Users/me/workspaces/api",
		"--registry",
		"/tmp/config.yaml",
		"--vm-mode",
		"dedicated",
		"--vm-name",
		"api-dev",
	})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	assertEqual(t, parsed.command, "project")
	assertEqual(t, parsed.subcommand, "import")
	assertEqual(t, parsed.positionals[0], "api")
	assertEqual(t, parsed.repoURL, "git@github.com:acme/api.git")
	assertEqual(t, parsed.identityFile, "/Users/me/.ssh/yard_acme")
	assertEqual(t, parsed.importPath, "/Users/me/workspaces/api")
	assertEqual(t, parsed.registryPath, "/tmp/config.yaml")
	assertEqual(t, parsed.vmMode, "dedicated")
	assertEqual(t, parsed.vmName, "api-dev")
}

func TestRunProjectImportTestsClonesAndRegistersProject(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "yard", "config.yaml")
	destination := filepath.Join(tempDir, "api")
	identityFile := filepath.Join(tempDir, "ssh", "yard_acme")
	importer := &fakeGitImporter{}
	fingerprinter := &fakeFingerprinter{fingerprint: "SHA256:abc123"}
	var output bytes.Buffer

	err := runProjectImportWithDeps(args{
		positionals:  []string{"api"},
		repoURL:      "git@github.com:acme/api.git",
		identityFile: identityFile,
		importPath:   destination,
		registryPath: registryPath,
		vmMode:       "dedicated",
		vmName:       "api-dev",
	}, importer, fingerprinter, &output)
	if err != nil {
		t.Fatalf("runProjectImportWithDeps returned error: %v", err)
	}

	absIdentity, err := filepath.Abs(identityFile)
	if err != nil {
		t.Fatal(err)
	}
	absDestination, err := filepath.Abs(destination)
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, importer.accessCalls[0], gitCall{
		repoURL:      "git@github.com:acme/api.git",
		identityFile: absIdentity,
	})
	assertEqual(t, importer.cloneCalls[0], gitCall{
		repoURL:      "git@github.com:acme/api.git",
		identityFile: absIdentity,
		destination:  absDestination,
	})
	assertEqual(t, fingerprinter.identityFile, absIdentity)

	reg, err := registry.Load(registryPath)
	if err != nil {
		t.Fatalf("registry.Load returned error: %v", err)
	}
	project := reg.Projects["api"]
	assertEqual(t, project.Path, absDestination)
	assertEqual(t, project.Git.IdentityFile, absIdentity)
	assertEqual(t, project.Git.Fingerprint, "SHA256:abc123")
	assertEqual(t, project.VM.Mode, "dedicated")
	assertEqual(t, project.VM.Name, "api-dev")

	for _, expected := range []string{"Testing repository access...", "Cloning repository...", "imported project api"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected output to contain %q:\n%s", expected, output.String())
		}
	}
}

func TestRunProjectImportRejectsNonEmptyDestination(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	destination := filepath.Join(tempDir, "api")
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "README.md"), []byte("# api\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	importer := &fakeGitImporter{}
	err := runProjectImportWithDeps(args{
		positionals:  []string{"api"},
		repoURL:      "git@github.com:acme/api.git",
		identityFile: filepath.Join(tempDir, "ssh", "yard_acme"),
		importPath:   destination,
		registryPath: filepath.Join(tempDir, "config.yaml"),
	}, importer, &fakeFingerprinter{fingerprint: "SHA256:abc123"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected non-empty destination to fail")
	}
	if len(importer.accessCalls) != 0 || len(importer.cloneCalls) != 0 {
		t.Fatalf("expected no git calls, got %#v %#v", importer.accessCalls, importer.cloneCalls)
	}
}

func TestRunProjectImportDoesNotCloneWhenAccessFails(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	importer := &fakeGitImporter{accessErr: errors.New("permission denied")}
	err := runProjectImportWithDeps(args{
		positionals:  []string{"api"},
		repoURL:      "git@github.com:acme/api.git",
		identityFile: filepath.Join(tempDir, "ssh", "yard_acme"),
		importPath:   filepath.Join(tempDir, "api"),
		registryPath: filepath.Join(tempDir, "config.yaml"),
	}, importer, &fakeFingerprinter{fingerprint: "SHA256:abc123"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected access failure")
	}
	if len(importer.cloneCalls) != 0 {
		t.Fatalf("expected no clone calls, got %#v", importer.cloneCalls)
	}
	if _, statErr := os.Stat(filepath.Join(tempDir, "config.yaml")); statErr == nil {
		t.Fatal("expected registry not to be written")
	}
}

type fakeGitImporter struct {
	accessCalls []gitCall
	cloneCalls  []gitCall
	accessErr   error
	cloneErr    error
}

type gitCall struct {
	repoURL      string
	identityFile string
	destination  string
}

func (i *fakeGitImporter) TestAccess(repoURL string, identityFile string) error {
	i.accessCalls = append(i.accessCalls, gitCall{
		repoURL:      repoURL,
		identityFile: identityFile,
	})
	return i.accessErr
}

func (i *fakeGitImporter) Clone(repoURL string, identityFile string, destination string) error {
	i.cloneCalls = append(i.cloneCalls, gitCall{
		repoURL:      repoURL,
		identityFile: identityFile,
		destination:  destination,
	})
	return i.cloneErr
}

type fakeFingerprinter struct {
	identityFile string
	fingerprint  string
	err          error
}

func (f *fakeFingerprinter) FingerprintForIdentity(identityFile string) (string, error) {
	f.identityFile = identityFile
	return f.fingerprint, f.err
}
