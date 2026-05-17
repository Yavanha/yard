package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yard/internal/prompt"
	"yard/internal/registry"
)

func TestParseProjectAddArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseArgs([]string{
		"project",
		"add",
		"example",
		"/tmp/example",
		"--runtime",
		"remote-server",
	})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	assertEqual(t, parsed.command, "project")
	assertEqual(t, parsed.subcommand, "add")
	assertEqual(t, parsed.positionals[0], "example")
	assertEqual(t, parsed.positionals[1], "/tmp/example")
	assertEqual(t, parsed.runtimeType, "remote-server")
}

func TestParseProjectAddVMArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseArgs([]string{
		"project",
		"add",
		"example",
		"/tmp/example",
		"--vm-mode",
		"dedicated",
		"--vm-name",
		"example-vm",
	})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	assertEqual(t, parsed.command, "project")
	assertEqual(t, parsed.subcommand, "add")
	assertEqual(t, parsed.positionals[0], "example")
	assertEqual(t, parsed.positionals[1], "/tmp/example")
	assertEqual(t, parsed.vmMode, "dedicated")
	assertEqual(t, parsed.vmName, "example-vm")
}

func TestRunProjectAddInteractive(t *testing.T) {
	t.Parallel()

	registryPath := filepath.Join(t.TempDir(), "config.yaml")
	repoPath := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	input := strings.Join([]string{
		repoPath,
		"api",
		"",
		"local-vm",
		"dedicated",
		"api-dev",
		"yes",
		"",
	}, "\n")

	err := runProjectAddInteractive(args{registryPath: registryPath}, prompt.New(strings.NewReader(input), &output))
	if err != nil {
		t.Fatalf("runProjectAddInteractive returned error: %v", err)
	}

	reg, err := registry.Load(registryPath)
	if err != nil {
		t.Fatalf("registry.Load returned error: %v", err)
	}
	project := reg.Projects["api"]
	assertEqual(t, project.Path, repoPath)
	assertEqual(t, project.Config, filepath.Join(repoPath, ".devctl.yml"))
	assertEqual(t, project.Runtime.Type, "local-vm")
	assertEqual(t, project.VM.Mode, "dedicated")
	assertEqual(t, project.VM.Name, "api-dev")
	if !strings.Contains(output.String(), "Registry preview:") {
		t.Fatalf("expected preview output, got:\n%s", output.String())
	}
}

func TestRunProjectAddInteractiveAbort(t *testing.T) {
	t.Parallel()

	registryPath := filepath.Join(t.TempDir(), "config.yaml")
	repoPath := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	input := strings.Join([]string{
		repoPath,
		"api",
		"",
		"local-vm",
		"shared",
		"",
		"no",
		"",
	}, "\n")

	err := runProjectAddInteractive(args{registryPath: registryPath}, prompt.New(strings.NewReader(input), &output))
	if err != nil {
		t.Fatalf("runProjectAddInteractive returned error: %v", err)
	}
	if _, err := os.Stat(registryPath); err == nil {
		t.Fatal("expected registry file not to be written")
	}
}

func TestRunProjectAddRemoteRuntime(t *testing.T) {
	t.Parallel()

	registryPath := filepath.Join(t.TempDir(), "config.yaml")
	err := runProjectAdd(args{
		positionals:  []string{"remote", "/tmp/remote"},
		registryPath: registryPath,
		runtimeType:  "remote-server",
	})
	if err != nil {
		t.Fatalf("runProjectAdd returned error: %v", err)
	}

	reg, err := registry.Load(registryPath)
	if err != nil {
		t.Fatalf("registry.Load returned error: %v", err)
	}
	project := reg.Projects["remote"]
	assertEqual(t, project.Runtime.Type, "remote-server")
	assertEqual(t, project.VM.Mode, "")
	assertEqual(t, project.VM.Name, "")
}

func TestRunProjectInspectPrintsGitIdentity(t *testing.T) {
	t.Parallel()

	registryPath := filepath.Join(t.TempDir(), "config.yaml")
	reg, err := registry.New().Add("api", registry.Project{
		Path:   "/tmp/api",
		Config: "/tmp/api/.devctl.yml",
		Git: registry.Git{
			IdentityFile: "/tmp/ssh/yard_acme_ed25519",
			Fingerprint:  "SHA256:abc123",
		},
		VM: registry.VM{
			Mode: "dedicated",
			Name: "api-dev",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Save(registryPath, reg); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	err = runProjectInspect(args{
		positionals:  []string{"api"},
		registryPath: registryPath,
	}, &output)
	if err != nil {
		t.Fatalf("runProjectInspect returned error: %v", err)
	}

	for _, expected := range []string{
		"FIELD",
		"name",
		"api",
		"current",
		"yes",
		"path",
		"/tmp/api",
		"runtime.type",
		"local-vm",
		"vm.mode",
		"dedicated",
		"git.identity_file",
		"/tmp/ssh/yard_acme_ed25519",
		"git.fingerprint",
		"SHA256:abc123",
	} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("expected output to contain %q:\n%s", expected, output.String())
		}
	}
}

func TestRunProjectInspectUsesCurrentProject(t *testing.T) {
	t.Parallel()

	registryPath := filepath.Join(t.TempDir(), "config.yaml")
	reg, err := registry.New().Add("api", registry.Project{Path: "/tmp/api"})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Save(registryPath, reg); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	err = runProjectInspect(args{registryPath: registryPath}, &output)
	if err != nil {
		t.Fatalf("runProjectInspect returned error: %v", err)
	}
	if !strings.Contains(output.String(), "api") {
		t.Fatalf("expected output to contain current project:\n%s", output.String())
	}
}

func TestRunProjectRemoveDeletesRegistryEntry(t *testing.T) {
	t.Parallel()

	registryPath := filepath.Join(t.TempDir(), "config.yaml")
	reg, err := registry.New().Add("api", registry.Project{Path: "/tmp/api"})
	if err != nil {
		t.Fatal(err)
	}
	reg, err = reg.Add("web", registry.Project{Path: "/tmp/web"})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Save(registryPath, reg); err != nil {
		t.Fatal(err)
	}

	err = runProjectRemove(args{
		positionals:  []string{"web"},
		registryPath: registryPath,
	})
	if err != nil {
		t.Fatalf("runProjectRemove returned error: %v", err)
	}

	reg, err = registry.Load(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Projects["web"]; ok {
		t.Fatal("expected web to be removed")
	}
	assertEqual(t, reg.CurrentProject, "api")
}

func TestRunProjectRemoveClearsCurrentProject(t *testing.T) {
	t.Parallel()

	registryPath := filepath.Join(t.TempDir(), "config.yaml")
	reg, err := registry.New().Add("api", registry.Project{Path: "/tmp/api"})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Save(registryPath, reg); err != nil {
		t.Fatal(err)
	}

	err = runProjectRemove(args{
		positionals:  []string{"api"},
		registryPath: registryPath,
	})
	if err != nil {
		t.Fatalf("runProjectRemove returned error: %v", err)
	}

	reg, err = registry.Load(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, reg.CurrentProject, "")
	if len(reg.Projects) != 0 {
		t.Fatalf("expected no projects, got %#v", reg.Projects)
	}
}

func TestParseUseArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseArgs([]string{"use", "example", "--registry", "/tmp/config.yaml"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	assertEqual(t, parsed.command, "use")
	assertEqual(t, parsed.positionals[0], "example")
	assertEqual(t, parsed.registryPath, "/tmp/config.yaml")
}
