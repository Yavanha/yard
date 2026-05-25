package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yard/internal/prompt"
	"yard/internal/registry"
	"yard/internal/sshkeys"
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
		"--runtime",
		"local-vm",
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
	assertEqual(t, parsed.runtimeType, "local-vm")
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
	assertEqual(t, project.Runtime.Type, "local-vm")
	assertEqual(t, project.VM.Mode, "dedicated")
	assertEqual(t, project.VM.Name, "api-dev")

	for _, expected := range []string{"Testing repository access...", "Cloning repository...", "imported project api"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected output to contain %q:\n%s", expected, output.String())
		}
	}
}

func TestRunProjectImportRegistersRemoteRuntime(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "yard", "config.yaml")
	destination := filepath.Join(tempDir, "api")
	identityFile := filepath.Join(tempDir, "ssh", "yard_acme")
	remoteIdentity := filepath.Join(tempDir, "ssh", "remote")
	importer := &fakeGitImporter{}
	fingerprinter := &fakeFingerprinter{fingerprint: "SHA256:abc123"}

	err := runProjectImportWithDeps(args{
		positionals:        []string{"api"},
		repoURL:            "git@github.com:acme/api.git",
		identityFile:       identityFile,
		importPath:         destination,
		registryPath:       registryPath,
		runtimeType:        "remote-server",
		remoteHost:         "dev.example.com",
		remoteUser:         "ubuntu",
		remotePort:         2222,
		remoteWorkdir:      "/home/ubuntu/workspaces/api",
		remoteIdentityFile: remoteIdentity,
		remoteHostKey:      "SHA256:host123",
	}, importer, fingerprinter, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("runProjectImportWithDeps returned error: %v", err)
	}

	reg, err := registry.Load(registryPath)
	if err != nil {
		t.Fatalf("registry.Load returned error: %v", err)
	}
	project := reg.Projects["api"]
	assertEqual(t, project.Runtime.Type, "remote-server")
	assertEqual(t, project.Remote.Host, "dev.example.com")
	assertEqual(t, project.Remote.User, "ubuntu")
	assertEqual(t, project.Remote.Port, 2222)
	assertEqual(t, project.Remote.Workdir, "/home/ubuntu/workspaces/api")
	assertEqual(t, project.Remote.IdentityFile, remoteIdentity)
	assertEqual(t, project.Remote.HostKeyFingerprint, "SHA256:host123")
	assertEqual(t, project.VM.Mode, "")
	assertEqual(t, project.VM.Name, "")
}

func TestRunProjectImportRemoteRuntimeRequiresMetadata(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	err := runProjectImportWithDeps(args{
		positionals:  []string{"api"},
		repoURL:      "git@github.com:acme/api.git",
		identityFile: filepath.Join(tempDir, "ssh", "yard_acme"),
		importPath:   filepath.Join(tempDir, "api"),
		registryPath: filepath.Join(tempDir, "yard", "config.yaml"),
		runtimeType:  "remote-server",
	}, &fakeGitImporter{}, &fakeFingerprinter{fingerprint: "SHA256:abc123"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected missing remote metadata to fail")
	}
	if !strings.Contains(err.Error(), "--remote-host is required") {
		t.Fatalf("expected remote host error, got %v", err)
	}
}

func TestRunProjectImportRejectsRemoteFlagsForLocalVM(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	err := runProjectImportWithDeps(args{
		positionals:  []string{"api"},
		repoURL:      "git@github.com:acme/api.git",
		identityFile: filepath.Join(tempDir, "ssh", "yard_acme"),
		importPath:   filepath.Join(tempDir, "api"),
		registryPath: filepath.Join(tempDir, "yard", "config.yaml"),
		runtimeType:  "local-vm",
		remoteHost:   "dev.example.com",
	}, &fakeGitImporter{}, &fakeFingerprinter{fingerprint: "SHA256:abc123"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected remote flags with local-vm to fail")
	}
	if !strings.Contains(err.Error(), "--remote-* flags require --runtime remote-server") {
		t.Fatalf("expected remote flag error, got %v", err)
	}
}

func TestRunProjectImportInteractiveSelectsExistingKey(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "yard", "config.yaml")
	destination := filepath.Join(tempDir, "api")
	identityFile := filepath.Join(tempDir, "ssh", "yard_acme")
	importer := &fakeGitImporter{}
	keys := &fakeFingerprinter{
		fingerprint: "SHA256:abc123",
		keys: []sshkeys.KeyCandidate{{
			Path:        identityFile,
			Comment:     "api@example.com",
			Fingerprint: "SHA256:abc123",
			InAgent:     true,
		}},
	}
	var output bytes.Buffer
	input := strings.Join([]string{
		"yes",
		"git@github.com:acme/api.git",
		"",
		destination,
		"",
		"local-vm",
		"dedicated",
		"",
		"1",
	}, "\n") + "\n"

	err := runProjectImportInteractiveWithDeps(
		args{registryPath: registryPath},
		importer,
		keys,
		prompt.New(strings.NewReader(input), &output),
	)
	if err != nil {
		t.Fatalf("runProjectImportInteractiveWithDeps returned error: %v", err)
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

	reg, err := registry.Load(registryPath)
	if err != nil {
		t.Fatalf("registry.Load returned error: %v", err)
	}
	project := reg.Projects["api"]
	assertEqual(t, project.Path, absDestination)
	assertEqual(t, project.Config, filepath.Join(absDestination, ".yard.yml"))
	assertEqual(t, project.Git.IdentityFile, absIdentity)
	assertEqual(t, project.Git.Fingerprint, "SHA256:abc123")
	assertEqual(t, project.Runtime.Type, "local-vm")
	assertEqual(t, project.VM.Mode, "dedicated")
	assertEqual(t, project.VM.Name, "api-dev")

	for _, expected := range []string{"Available SSH keys:", "Repository URL", "imported project api"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected output to contain %q:\n%s", expected, output.String())
		}
	}
}

func TestRunProjectImportInteractiveCreatesAndUploadsKey(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "yard", "config.yaml")
	destination := filepath.Join(tempDir, "api")
	identityFile := filepath.Join(tempDir, "ssh", "yard_acme_api_ed25519")
	importer := &fakeGitImporter{}
	keys := &fakeFingerprinter{
		fingerprint: "SHA256:created",
		publicKey:   "ssh-ed25519 AAAA yard api",
	}
	uploader := &fakePublicKeyUploader{available: true}
	var output bytes.Buffer
	input := strings.Join([]string{
		"no",
		"git@github.com:acme/api.git",
		"",
		destination,
		"",
		"local-vm",
		"shared",
		"",
		identityFile,
		"",
		"yes",
		"yes",
	}, "\n") + "\n"

	err := runProjectImportInteractiveWithDepsAndUploader(
		args{registryPath: registryPath},
		importer,
		keys,
		uploader,
		prompt.New(strings.NewReader(input), &output),
	)
	if err != nil {
		t.Fatalf("runProjectImportInteractiveWithDepsAndUploader returned error: %v", err)
	}

	absIdentity, err := filepath.Abs(identityFile)
	if err != nil {
		t.Fatal(err)
	}
	absDestination, err := filepath.Abs(destination)
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, keys.createdIdentityFile, absIdentity)
	assertEqual(t, keys.createdComment, "yard api")
	assertEqual(t, uploader.uploadedPublicKeyPath, absIdentity+".pub")
	assertEqual(t, uploader.uploadedTitle, "yard api")
	assertEqual(t, importer.accessCalls[0], gitCall{
		repoURL:      "git@github.com:acme/api.git",
		identityFile: absIdentity,
	})

	reg, err := registry.Load(registryPath)
	if err != nil {
		t.Fatalf("registry.Load returned error: %v", err)
	}
	project := reg.Projects["api"]
	assertEqual(t, project.Path, absDestination)
	assertEqual(t, project.Git.IdentityFile, absIdentity)
	assertEqual(t, project.Git.Fingerprint, "SHA256:created")

	for _, expected := range []string{"Running ssh-keygen.", "Public key:", "Upload public key with gh"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected output to contain %q:\n%s", expected, output.String())
		}
	}
}

func TestRunProjectImportInteractiveNotSureFallsBackToNewKey(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "yard", "config.yaml")
	destination := filepath.Join(tempDir, "api")
	existingIdentityFile := filepath.Join(tempDir, "ssh", "existing")
	createdIdentityFile := filepath.Join(tempDir, "ssh", "yard_acme_api_ed25519")
	importer := &fakeGitImporter{
		accessErrs: []error{
			errors.New("permission denied"),
			nil,
		},
	}
	keys := &fakeFingerprinter{
		fingerprint: "SHA256:created",
		publicKey:   "ssh-ed25519 AAAA yard api",
		keys: []sshkeys.KeyCandidate{{
			Path:        existingIdentityFile,
			Comment:     "old@example.com",
			Fingerprint: "SHA256:old",
		}},
	}
	var output bytes.Buffer
	input := strings.Join([]string{
		"not sure",
		"git@github.com:acme/api.git",
		"",
		destination,
		"",
		"local-vm",
		"shared",
		"",
		"1",
		"yes",
		createdIdentityFile,
		"",
		"yes",
		"",
	}, "\n") + "\n"

	err := runProjectImportInteractiveWithDepsAndUploader(
		args{registryPath: registryPath},
		importer,
		keys,
		&fakePublicKeyUploader{},
		prompt.New(strings.NewReader(input), &output),
	)
	if err != nil {
		t.Fatalf("runProjectImportInteractiveWithDepsAndUploader returned error: %v", err)
	}

	absExistingIdentity, err := filepath.Abs(existingIdentityFile)
	if err != nil {
		t.Fatal(err)
	}
	absCreatedIdentity, err := filepath.Abs(createdIdentityFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(importer.accessCalls) != 2 {
		t.Fatalf("expected 2 access tests, got %#v", importer.accessCalls)
	}
	assertEqual(t, importer.accessCalls[0], gitCall{
		repoURL:      "git@github.com:acme/api.git",
		identityFile: absExistingIdentity,
	})
	assertEqual(t, importer.accessCalls[1], gitCall{
		repoURL:      "git@github.com:acme/api.git",
		identityFile: absCreatedIdentity,
	})
	assertEqual(t, keys.createdIdentityFile, absCreatedIdentity)

	reg, err := registry.Load(registryPath)
	if err != nil {
		t.Fatalf("registry.Load returned error: %v", err)
	}
	project := reg.Projects["api"]
	assertEqual(t, project.Git.IdentityFile, absCreatedIdentity)
	assertEqual(t, project.Git.Fingerprint, "SHA256:created")

	for _, expected := range []string{"Selected SSH key did not work:", "Create a new SSH key", "Public key:"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected output to contain %q:\n%s", expected, output.String())
		}
	}
}

func TestRunProjectImportInteractiveYesFallsBackToNewKey(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	registryPath := filepath.Join(tempDir, "yard", "config.yaml")
	destination := filepath.Join(tempDir, "api")
	existingIdentityFile := filepath.Join(tempDir, "ssh", "existing")
	createdIdentityFile := filepath.Join(tempDir, "ssh", "yard_acme_api_ed25519")
	importer := &fakeGitImporter{
		accessErrs: []error{
			errors.New("permission denied"),
			nil,
		},
	}
	keys := &fakeFingerprinter{
		fingerprint: "SHA256:created",
		publicKey:   "ssh-ed25519 AAAA yard api",
		keys: []sshkeys.KeyCandidate{{
			Path:        existingIdentityFile,
			Comment:     "old@example.com",
			Fingerprint: "SHA256:old",
		}},
	}
	var output bytes.Buffer
	input := strings.Join([]string{
		"yes",
		"git@github.com:acme/api.git",
		"",
		destination,
		"",
		"local-vm",
		"shared",
		"",
		"1",
		"yes",
		createdIdentityFile,
		"",
		"yes",
		"",
	}, "\n") + "\n"

	err := runProjectImportInteractiveWithDepsAndUploader(
		args{registryPath: registryPath},
		importer,
		keys,
		&fakePublicKeyUploader{},
		prompt.New(strings.NewReader(input), &output),
	)
	if err != nil {
		t.Fatalf("runProjectImportInteractiveWithDepsAndUploader returned error: %v", err)
	}

	absExistingIdentity, err := filepath.Abs(existingIdentityFile)
	if err != nil {
		t.Fatal(err)
	}
	absCreatedIdentity, err := filepath.Abs(createdIdentityFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(importer.accessCalls) != 2 {
		t.Fatalf("expected 2 access tests, got %#v", importer.accessCalls)
	}
	assertEqual(t, importer.accessCalls[0], gitCall{
		repoURL:      "git@github.com:acme/api.git",
		identityFile: absExistingIdentity,
	})
	assertEqual(t, importer.accessCalls[1], gitCall{
		repoURL:      "git@github.com:acme/api.git",
		identityFile: absCreatedIdentity,
	})
	assertEqual(t, keys.createdIdentityFile, absCreatedIdentity)

	reg, err := registry.Load(registryPath)
	if err != nil {
		t.Fatalf("registry.Load returned error: %v", err)
	}
	project := reg.Projects["api"]
	assertEqual(t, project.Git.IdentityFile, absCreatedIdentity)
	assertEqual(t, project.Git.Fingerprint, "SHA256:created")

	for _, expected := range []string{"Selected SSH key did not work:", "Create a new SSH key", "Public key:"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected output to contain %q:\n%s", expected, output.String())
		}
	}
}

func TestCreateImportSSHKeyPrintsManualInstructionsWhenGHIsMissing(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	identityFile := filepath.Join(tempDir, "ssh", "yard_acme_api_ed25519")
	keys := &fakeFingerprinter{
		fingerprint: "SHA256:created",
		publicKey:   "ssh-ed25519 AAAA yard api",
	}
	var output bytes.Buffer
	input := strings.Join([]string{
		identityFile,
		"",
		"yes",
		"",
	}, "\n") + "\n"

	key, err := createImportSSHKey(
		prompt.New(strings.NewReader(input), &output),
		keys,
		&fakePublicKeyUploader{},
		"git@github.com:acme/api.git",
	)
	if err != nil {
		t.Fatalf("createImportSSHKey returned error: %v", err)
	}

	absIdentity, err := filepath.Abs(identityFile)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, key.Path, absIdentity)
	assertEqual(t, keys.createdIdentityFile, absIdentity)
	for _, expected := range []string{"Public key:", "Add this public key to GitHub, GitLab, or your Git provider"} {
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
	accessErrs  []error
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
	if len(i.accessErrs) > 0 {
		err := i.accessErrs[0]
		i.accessErrs = i.accessErrs[1:]
		return err
	}
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
	identityFile        string
	fingerprint         string
	err                 error
	keys                []sshkeys.KeyCandidate
	createdIdentityFile string
	createdComment      string
	publicKey           string
}

func (f *fakeFingerprinter) FingerprintForIdentity(identityFile string) (string, error) {
	f.identityFile = identityFile
	return f.fingerprint, f.err
}

func (f *fakeFingerprinter) List() ([]sshkeys.KeyCandidate, error) {
	return f.keys, f.err
}

func (f *fakeFingerprinter) Create(identityFile string, comment string) (sshkeys.KeyCandidate, error) {
	f.createdIdentityFile = identityFile
	f.createdComment = comment
	if f.err != nil {
		return sshkeys.KeyCandidate{}, f.err
	}
	return sshkeys.KeyCandidate{
		Path:        identityFile,
		Comment:     comment,
		Fingerprint: f.fingerprint,
	}, nil
}

func (f *fakeFingerprinter) PublicKey(identityFile string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.publicKey, nil
}

type fakePublicKeyUploader struct {
	available             bool
	uploadedPublicKeyPath string
	uploadedTitle         string
	err                   error
}

func (u *fakePublicKeyUploader) Available() bool {
	return u.available
}

func (u *fakePublicKeyUploader) UploadPublicKey(publicKeyPath string, title string) error {
	u.uploadedPublicKeyPath = publicKeyPath
	u.uploadedTitle = title
	return u.err
}
