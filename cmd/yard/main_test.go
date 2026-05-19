package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"yard/internal/config"
	"yard/internal/process"
	"yard/internal/prompt"
	"yard/internal/provider/lima"
	"yard/internal/registry"
)

func TestParseVMStatusArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseArgs([]string{"vm", "status", "example"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	assertEqual(t, parsed.command, "vm")
	assertEqual(t, parsed.subcommand, "status")
	assertEqual(t, parsed.positionals[0], "example")
}

func TestParseExecArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseArgs([]string{"exec", "example", "--", "echo", "hello"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	assertEqual(t, parsed.command, "exec")
	assertEqual(t, parsed.positionals[0], "example")
	assertEqual(t, parsed.execCommand[0], "echo")
	assertEqual(t, parsed.execCommand[1], "hello")
}

func TestParseConfigNamedProjectArg(t *testing.T) {
	t.Parallel()

	parsed, err := parseArgs([]string{"config", "example", "--registry", "/tmp/config.yaml"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	assertEqual(t, parsed.command, "config")
	assertEqual(t, parsed.positionals[0], "example")
	assertEqual(t, parsed.registryPath, "/tmp/config.yaml")
}

func TestParseStatusArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseArgs([]string{"status", "example"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	assertEqual(t, parsed.command, "status")
	assertEqual(t, parsed.positionals[0], "example")
}

func TestParseSetupArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseArgs([]string{"setup", "example", "--registry", "/tmp/config.yaml"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	assertEqual(t, parsed.command, "setup")
	assertEqual(t, parsed.positionals[0], "example")
	assertEqual(t, parsed.registryPath, "/tmp/config.yaml")
}

func TestParseInitArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseArgs([]string{
		"init",
		"api",
		"--yes",
		"--force",
		"--config",
		"/tmp/api.yml",
		"--repo",
		"git@github.com:acme/api.git",
		"--repo-dir",
		"/home/ubuntu/workspaces/api",
		"--vm-name",
		"api-dev",
		"--vm-provider",
		"auto",
		"--vm-user",
		"ubuntu",
		"--vm-type",
		"vz",
		"--cpus",
		"4",
		"--memory",
		"6G",
		"--disk",
		"50G",
		"--service",
		"web",
		"--command",
		"pnpm dev",
		"--workdir",
		".",
		"--port",
		"3000",
	})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	assertEqual(t, parsed.command, "init")
	assertEqual(t, parsed.positionals[0], "api")
	assertEqual(t, parsed.yes, true)
	assertEqual(t, parsed.force, true)
	assertEqual(t, parsed.configPath, "/tmp/api.yml")
	assertEqual(t, parsed.repoURL, "git@github.com:acme/api.git")
	assertEqual(t, parsed.vmProvider, "auto")
	assertEqual(t, parsed.serviceName, "web")
	assertEqual(t, parsed.serviceCmd, "pnpm dev")
	assertEqual(t, parsed.servicePort, 3000)
}

func TestRunInitWritesConfigNonInteractive(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), ".devctl.yml")
	err := runInit(args{
		positionals: []string{"api"},
		configPath:  configPath,
		yes:         true,
		repoURL:     "git@github.com:acme/api.git",
		serviceName: "web",
		serviceCmd:  "go run ./cmd/api",
		servicePort: 8080,
		vmName:      "api-dev",
		vmProvider:  "auto",
		vmType:      "vz",
		cpus:        2,
		memory:      "4G",
		disk:        "40G",
		repoDir:     "/home/ubuntu/workspaces/api",
		serviceDir:  ".",
	})
	if err != nil {
		t.Fatalf("runInit returned error: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	for _, expected := range []string{
		`project: "api"`,
		`repo: "git@github.com:acme/api.git"`,
		`vm_name: "api-dev"`,
		`  cpus: 2`,
		`  web: 8080`,
		`    command: "go run ./cmd/api"`,
	} {
		if !strings.Contains(string(content), expected) {
			t.Fatalf("expected config to contain %q:\n%s", expected, string(content))
		}
	}
}

func TestRunInitRejectsExistingConfigWithoutForce(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), ".devctl.yml")
	if err := os.WriteFile(configPath, []byte("project: existing\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := runInit(args{
		positionals: []string{"api"},
		configPath:  configPath,
		yes:         true,
	})
	if err == nil {
		t.Fatal("expected existing config to fail")
	}
}

func TestRunInitInteractive(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), ".devctl.yml")
	var output bytes.Buffer
	input := strings.Join([]string{
		"api",
		"git@github.com:acme/api.git",
		"",
		"",
		"",
		"",
		"",
		"2",
		"4G",
		"40G",
		"web",
		"go run ./cmd/api",
		".",
		"8080",
		"yes",
		"",
	}, "\n")

	err := runInitInteractive(configPath, config.DefaultScaffoldOptions("example"), prompt.New(strings.NewReader(input), &output))
	if err != nil {
		t.Fatalf("runInitInteractive returned error: %v", err)
	}

	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile returned error: %v", err)
	}
	if !strings.Contains(string(content), `project: "api"`) {
		t.Fatalf("expected config to use prompted project:\n%s", string(content))
	}
	if !strings.Contains(string(content), `vm_name: "api-dev"`) {
		t.Fatalf("expected VM name default to follow prompted project:\n%s", string(content))
	}
	if !strings.Contains(output.String(), "Config preview:") {
		t.Fatalf("expected preview output, got:\n%s", output.String())
	}
}

func TestParseStartArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseArgs([]string{"start", "example", "--registry", "/tmp/config.yaml"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	assertEqual(t, parsed.command, "start")
	assertEqual(t, parsed.positionals[0], "example")
	assertEqual(t, parsed.registryPath, "/tmp/config.yaml")
}

func TestParseStopVMArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseArgs([]string{"stop", "example", "--vm"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	assertEqual(t, parsed.command, "stop")
	assertEqual(t, parsed.positionals[0], "example")
	assertEqual(t, parsed.stopVM, true)
}

func TestParseProcessStartArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseArgs([]string{"process", "start", "api", "web", "--registry", "/tmp/config.yaml"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	assertEqual(t, parsed.command, "process")
	assertEqual(t, parsed.subcommand, "start")
	assertEqual(t, parsed.positionals[0], "api")
	assertEqual(t, parsed.positionals[1], "web")
	assertEqual(t, parsed.registryPath, "/tmp/config.yaml")
}

func TestParseProcessLogsArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseArgs([]string{"process", "logs", "api", "web", "--tail", "80", "--follow"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	assertEqual(t, parsed.command, "process")
	assertEqual(t, parsed.subcommand, "logs")
	assertEqual(t, parsed.positionals[0], "api")
	assertEqual(t, parsed.positionals[1], "web")
	assertEqual(t, parsed.tailLines, 80)
	assertEqual(t, parsed.follow, true)
}

func TestProcessActionTarget(t *testing.T) {
	t.Parallel()

	projectName, serviceName, err := processActionTarget([]string{"web"})
	if err != nil {
		t.Fatalf("processActionTarget returned error: %v", err)
	}
	assertEqual(t, projectName, "")
	assertEqual(t, serviceName, "web")

	projectName, serviceName, err = processActionTarget([]string{"api", "web"})
	if err != nil {
		t.Fatalf("processActionTarget returned error: %v", err)
	}
	assertEqual(t, projectName, "api")
	assertEqual(t, serviceName, "web")
}

func TestResolvedConfigPathUsesDirectProjectPath(t *testing.T) {
	t.Parallel()

	resolved, err := resolvedConfigPath(args{projectPath: "/tmp/example/.devctl.yml"})
	if err != nil {
		t.Fatalf("resolvedConfigPath returned error: %v", err)
	}
	assertEqual(t, resolved, "/tmp/example/.devctl.yml")
}

func TestResolvedConfigPathUsesCurrentRegistryProject(t *testing.T) {
	t.Parallel()

	registryPath := filepath.Join(t.TempDir(), "config.yaml")
	reg, err := registry.New().Add("example", registry.Project{Path: "/tmp/example"})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Save(registryPath, reg); err != nil {
		t.Fatal(err)
	}

	resolved, err := resolvedConfigPath(args{registryPath: registryPath})
	if err != nil {
		t.Fatalf("resolvedConfigPath returned error: %v", err)
	}
	assertEqual(t, resolved, "/tmp/example/.devctl.yml")
}

func TestResolvedConfigPathUsesNamedRegistryProject(t *testing.T) {
	t.Parallel()

	registryPath := filepath.Join(t.TempDir(), "config.yaml")
	reg, err := registry.New().Add("front", registry.Project{Path: "/tmp/front"})
	if err != nil {
		t.Fatal(err)
	}
	reg, err = reg.Add("api", registry.Project{Path: "/tmp/api"})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Save(registryPath, reg); err != nil {
		t.Fatal(err)
	}

	resolved, err := resolvedConfigPath(args{
		positionals:  []string{"api"},
		registryPath: registryPath,
	})
	if err != nil {
		t.Fatalf("resolvedConfigPath returned error: %v", err)
	}
	assertEqual(t, resolved, "/tmp/api/.devctl.yml")
}

func TestResolvedVMNameUsesCurrentRegistryProject(t *testing.T) {
	t.Parallel()

	registryPath := filepath.Join(t.TempDir(), "config.yaml")
	reg, err := registry.New().Add("example", registry.Project{
		Path: "/tmp/example",
		VM:   registry.VM{Name: "example-vm"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Save(registryPath, reg); err != nil {
		t.Fatal(err)
	}

	resolved, err := resolvedVMName(args{registryPath: registryPath})
	if err != nil {
		t.Fatalf("resolvedVMName returned error: %v", err)
	}
	assertEqual(t, resolved, "example-vm")
}

func TestResolvedVMNameUsesNamedRegistryProject(t *testing.T) {
	t.Parallel()

	registryPath := filepath.Join(t.TempDir(), "config.yaml")
	reg, err := registry.New().Add("front", registry.Project{Path: "/tmp/front"})
	if err != nil {
		t.Fatal(err)
	}
	reg, err = reg.Add("api", registry.Project{
		Path: "/tmp/api",
		VM:   registry.VM{Name: "api-vm"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Save(registryPath, reg); err != nil {
		t.Fatal(err)
	}

	resolved, err := resolvedVMName(args{
		positionals:  []string{"api"},
		registryPath: registryPath,
	})
	if err != nil {
		t.Fatalf("resolvedVMName returned error: %v", err)
	}
	assertEqual(t, resolved, "api-vm")
}

func TestResolvedVMNameFallsBackToLiteralVMName(t *testing.T) {
	t.Parallel()

	resolved, err := resolvedVMName(args{
		positionals:  []string{"raw-vm"},
		registryPath: filepath.Join(t.TempDir(), "missing.yaml"),
	})
	if err != nil {
		t.Fatalf("resolvedVMName returned error: %v", err)
	}
	assertEqual(t, resolved, "raw-vm")
}

func TestResolvedVMNameRejectsRemoteRuntimeProject(t *testing.T) {
	t.Parallel()

	registryPath := filepath.Join(t.TempDir(), "config.yaml")
	reg, err := registry.New().Add("remote", registry.Project{
		Path:    "/tmp/remote",
		Runtime: registry.RuntimeTarget{Type: registry.RuntimeTypeRemote},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Save(registryPath, reg); err != nil {
		t.Fatal(err)
	}

	_, err = resolvedVMName(args{registryPath: registryPath})
	if err == nil {
		t.Fatal("expected remote runtime project to fail")
	}
	if !strings.Contains(err.Error(), "runtime target remote-server is not supported yet") {
		t.Fatalf("expected remote runtime error, got %v", err)
	}
}

func TestResolvedProjectConfigUsesRegistryVMName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, ".devctl.yml")
	writeTestConfig(t, configPath)

	registryPath := filepath.Join(t.TempDir(), "config.yaml")
	reg, err := registry.New().Add("example", registry.Project{
		Path:   dir,
		Config: configPath,
		VM:     registry.VM{Name: "registry-vm"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Save(registryPath, reg); err != nil {
		t.Fatal(err)
	}

	projectConfig, err := resolvedProjectConfig(args{
		registryPath: registryPath,
	})
	if err != nil {
		t.Fatalf("resolvedProjectConfig returned error: %v", err)
	}
	assertEqual(t, projectConfig.VMName, "registry-vm")
	assertEqual(t, projectConfig.Resources.CPUs, 4)
}

func TestResolvedProjectConfigRejectsRemoteRuntimeProject(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, ".devctl.yml")
	writeTestConfig(t, configPath)

	registryPath := filepath.Join(t.TempDir(), "config.yaml")
	reg, err := registry.New().Add("remote", registry.Project{
		Path:    dir,
		Config:  configPath,
		Runtime: registry.RuntimeTarget{Type: registry.RuntimeTypeRemote},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Save(registryPath, reg); err != nil {
		t.Fatal(err)
	}

	_, err = resolvedProjectConfig(args{registryPath: registryPath})
	if err == nil {
		t.Fatal("expected remote runtime project to fail")
	}
	if !strings.Contains(err.Error(), "runtime target remote-server is not supported yet") {
		t.Fatalf("expected remote runtime error, got %v", err)
	}
}

func TestResolvedProjectConfigUsesDirectProjectConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, ".devctl.yml")
	writeTestConfig(t, configPath)

	projectConfig, err := resolvedProjectConfig(args{projectPath: configPath})
	if err != nil {
		t.Fatalf("resolvedProjectConfig returned error: %v", err)
	}
	assertEqual(t, projectConfig.VMName, "file-vm")
}

func TestBuildStatusRows(t *testing.T) {
	t.Parallel()

	reg, err := registry.New().Add("front", registry.Project{
		Path: "/tmp/front",
		VM:   registry.VM{Name: "front-vm"},
	})
	if err != nil {
		t.Fatal(err)
	}
	reg, err = reg.Add("api", registry.Project{
		Path: "/tmp/api",
		VM:   registry.VM{Name: "api-vm"},
	})
	if err != nil {
		t.Fatal(err)
	}
	reg, err = reg.Use("api")
	if err != nil {
		t.Fatal(err)
	}

	rows, err := buildStatusRows(reg, []lima.Instance{
		{Name: "front-vm", Status: "Running"},
	}, "", nil)
	if err != nil {
		t.Fatalf("buildStatusRows returned error: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	assertEqual(t, rows[0].Project, "api")
	assertEqual(t, rows[0].Current, true)
	assertEqual(t, rows[0].Runtime, "local-vm")
	assertEqual(t, rows[0].VMState, "missing")
	assertEqual(t, rows[1].Project, "front")
	assertEqual(t, rows[1].VMState, "Running")
}

func TestBuildStatusRowsShowsRemoteRuntimeUnsupported(t *testing.T) {
	t.Parallel()

	reg, err := registry.New().Add("remote", registry.Project{
		Path:    "/tmp/remote",
		Runtime: registry.RuntimeTarget{Type: registry.RuntimeTypeRemote},
	})
	if err != nil {
		t.Fatal(err)
	}

	rows, err := buildStatusRows(reg, nil, "", nil)
	if err != nil {
		t.Fatalf("buildStatusRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	assertEqual(t, rows[0].Project, "remote")
	assertEqual(t, rows[0].Runtime, "remote-server")
	assertEqual(t, rows[0].VM, "")
	assertEqual(t, rows[0].VMState, "unsupported")
	assertEqual(t, rows[0].VMMode, "")
}

func TestBuildStatusRowsChecksRemoteRuntimeReachability(t *testing.T) {
	t.Parallel()

	reg, err := registry.New().Add("remote", registry.Project{
		Path:    "/tmp/remote",
		Runtime: registry.RuntimeTarget{Type: registry.RuntimeTypeRemote},
	})
	if err != nil {
		t.Fatal(err)
	}

	rows, err := buildStatusRows(reg, nil, "", func(registry.Project) string {
		return "reachable"
	})
	if err != nil {
		t.Fatalf("buildStatusRows returned error: %v", err)
	}
	assertEqual(t, rows[0].VMState, "reachable")
}

func TestBuildStatusRowsFiltersProject(t *testing.T) {
	t.Parallel()

	reg, err := registry.New().Add("front", registry.Project{Path: "/tmp/front"})
	if err != nil {
		t.Fatal(err)
	}
	reg, err = reg.Add("api", registry.Project{Path: "/tmp/api"})
	if err != nil {
		t.Fatal(err)
	}

	rows, err := buildStatusRows(reg, nil, "api", nil)
	if err != nil {
		t.Fatalf("buildStatusRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	assertEqual(t, rows[0].Project, "api")
}

func TestWriteStatusRows(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := writeStatusRows(&output, []statusRow{{
		Current: true,
		Project: "api",
		Runtime: "local-vm",
		VM:      "api-vm",
		VMState: "Running",
		VMMode:  "dedicated",
		Config:  "/tmp/api/.devctl.yml",
		Path:    "/tmp/api",
	}})
	if err != nil {
		t.Fatalf("writeStatusRows returned error: %v", err)
	}

	got := output.String()
	for _, expected := range []string{"CURRENT", "PROJECT", "RUNTIME", "VM_STATE", "*", "api", "local-vm", "api-vm", "Running"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected output to contain %q:\n%s", expected, got)
		}
	}
}

func TestWriteProcessRows(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := writeProcessRows(&output, []process.State{{
		Project: "api",
		Service: "web",
		Status:  "running",
		PID:     "1234",
		Port:    3000,
		Command: "pnpm dev",
		Log:     "/home/ubuntu/.yard/processes/api/web/stdout.log",
	}}, "api-vm")
	if err != nil {
		t.Fatalf("writeProcessRows returned error: %v", err)
	}

	got := output.String()
	for _, expected := range []string{"PROJECT", "SERVICE", "STATUS", "api", "web", "running", "1234", "3000", "api-vm"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected output to contain %q:\n%s", expected, got)
		}
	}
}

func TestShouldStopProjectVM(t *testing.T) {
	t.Parallel()

	assertEqual(t, shouldStopProjectVM(registry.Project{VM: registry.VM{Mode: "dedicated"}}, false), true)
	assertEqual(t, shouldStopProjectVM(registry.Project{VM: registry.VM{Mode: "shared"}}, false), false)
	assertEqual(t, shouldStopProjectVM(registry.Project{VM: registry.VM{Mode: "shared"}}, true), true)
}

func TestParseRejectsMissingFlagValue(t *testing.T) {
	t.Parallel()

	_, err := parseArgs([]string{"project", "add", "example", "/tmp/example", "--vm-mode"})
	if err == nil {
		t.Fatal("expected missing flag value to fail")
	}
}

func assertEqual(t *testing.T, got any, want any) {
	t.Helper()
	if got != want {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}

func writeTestConfig(t *testing.T, path string) {
	t.Helper()
	content := []byte(`vm_name: file-vm
vm_user: ubuntu
vm:
  provider: auto
  type: vz
resources:
  cpus: 4
  memory: 6G
  disk: 50G
ports:
  app: 3000
`)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
}
