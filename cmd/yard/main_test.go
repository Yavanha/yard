package main

import (
	"bytes"
	"errors"
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

func TestParseVMDeleteArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseArgs([]string{"vm", "delete", "example"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	assertEqual(t, parsed.command, "vm")
	assertEqual(t, parsed.subcommand, "delete")
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

	configPath := filepath.Join(t.TempDir(), ".yard.yml")
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

	configPath := filepath.Join(t.TempDir(), ".yard.yml")
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

func TestRunInitRejectsLegacyConfigPath(t *testing.T) {
	t.Parallel()

	err := runInit(args{
		positionals: []string{"api"},
		configPath:  filepath.Join(t.TempDir(), "."+"dev"+"ctl.yml"),
		yes:         true,
	})
	if err == nil {
		t.Fatal("expected legacy config path to fail")
	}
}

func TestRunInitInteractive(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join(t.TempDir(), ".yard.yml")
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

func TestParseGuideArgs(t *testing.T) {
	t.Parallel()

	parsed, err := parseArgs([]string{"guide", "smoke-test"})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	assertEqual(t, parsed.command, "guide")
	assertEqual(t, parsed.subcommand, "smoke-test")
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

	resolved, err := resolvedConfigPath(args{projectPath: "/tmp/example/.yard.yml"})
	if err != nil {
		t.Fatalf("resolvedConfigPath returned error: %v", err)
	}
	assertEqual(t, resolved, "/tmp/example/.yard.yml")
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
	assertEqual(t, resolved, "/tmp/example/.yard.yml")
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
	assertEqual(t, resolved, "/tmp/api/.yard.yml")
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

func TestRunVMDeleteDeletesStoppedVM(t *testing.T) {
	t.Parallel()

	client := &recordingVMDeleteClient{
		statuses: map[string]lima.Instance{
			"raw-vm": {Name: "raw-vm", Status: "Stopped"},
		},
	}
	var output bytes.Buffer
	err := runVMDeleteWithClient(args{
		positionals:  []string{"raw-vm"},
		registryPath: filepath.Join(t.TempDir(), "missing.yaml"),
	}, client, &output)
	if err != nil {
		t.Fatalf("runVMDeleteWithClient returned error: %v", err)
	}

	assertEqual(t, client.deleted[0], "raw-vm")
	if !strings.Contains(output.String(), "VM deleted: raw-vm") {
		t.Fatalf("expected delete output, got %q", output.String())
	}
}

func TestRunVMDeleteRejectsRunningVM(t *testing.T) {
	t.Parallel()

	client := &recordingVMDeleteClient{
		statuses: map[string]lima.Instance{
			"raw-vm": {Name: "raw-vm", Status: "Running"},
		},
	}
	err := runVMDeleteWithClient(args{
		positionals:  []string{"raw-vm"},
		registryPath: filepath.Join(t.TempDir(), "missing.yaml"),
	}, client, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected running VM delete to fail")
	}
	if len(client.deleted) != 0 {
		t.Fatalf("expected no delete calls, got %#v", client.deleted)
	}
}

func TestRunVMDeleteRejectsRemoteRuntimeProject(t *testing.T) {
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

	err = runVMDeleteWithClient(args{
		positionals:  []string{"remote"},
		registryPath: registryPath,
	}, &recordingVMDeleteClient{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected remote runtime project to fail")
	}
}

func TestRunVMDeleteRejectsSharedVMByProjectAlias(t *testing.T) {
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

	err = runVMDeleteWithClient(args{
		positionals:  []string{"api"},
		registryPath: registryPath,
	}, &recordingVMDeleteClient{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected shared VM delete by project alias to fail")
	}
}

func TestResolvedProjectConfigUsesRegistryVMName(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, ".yard.yml")
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
	configPath := filepath.Join(dir, ".yard.yml")
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

func TestResolvedProcessProjectUsesRemoteWorkdir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, ".yard.yml")
	writeTestConfig(t, configPath)

	registryPath := filepath.Join(t.TempDir(), "config.yaml")
	reg, err := registry.New().Add("remote", registry.Project{
		Path:    dir,
		Config:  configPath,
		Runtime: registry.RuntimeTarget{Type: registry.RuntimeTypeRemote},
		Remote: registry.RemoteServer{
			Host:    "dev.example.com",
			User:    "ubuntu",
			Workdir: "/srv/api",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Save(registryPath, reg); err != nil {
		t.Fatal(err)
	}

	projectName, project, projectConfig, err := resolvedProcessProject(args{registryPath: registryPath}, "")
	if err != nil {
		t.Fatalf("resolvedProcessProject returned error: %v", err)
	}
	assertEqual(t, projectName, "remote")
	assertEqual(t, project.Runtime.Type, registry.RuntimeTypeRemote)
	assertEqual(t, projectConfig.RepoDir, "/srv/api")
}

func TestResolvedProjectConfigUsesDirectProjectConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	configPath := filepath.Join(dir, ".yard.yml")
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
	assertEqual(t, rows[0].Target, "api-vm")
	assertEqual(t, rows[0].TargetState, "missing")
	assertEqual(t, rows[1].Project, "front")
	assertEqual(t, rows[1].TargetState, "Running")
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
	assertEqual(t, rows[0].Target, "remote-server")
	assertEqual(t, rows[0].TargetState, "unsupported")
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
	assertEqual(t, rows[0].TargetState, "reachable")
}

func TestStatusNeedsLocalVMInstances(t *testing.T) {
	t.Parallel()

	reg, err := registry.New().Add("remote", registry.Project{
		Path:    "/tmp/remote",
		Runtime: registry.RuntimeTarget{Type: registry.RuntimeTypeRemote},
	})
	if err != nil {
		t.Fatal(err)
	}
	reg, err = reg.Add("local", registry.Project{Path: "/tmp/local"})
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, statusNeedsLocalVMInstances(reg, ""), true)
	assertEqual(t, statusNeedsLocalVMInstances(reg, "local"), true)
	assertEqual(t, statusNeedsLocalVMInstances(reg, "remote"), false)
	assertEqual(t, statusNeedsLocalVMInstances(reg, "missing"), false)
}

func TestRemoteReachabilityStateReportsRunnerFailure(t *testing.T) {
	t.Parallel()

	state := remoteReachabilityStateWithRunner(registry.Project{
		Runtime: registry.RuntimeTarget{Type: registry.RuntimeTypeRemote},
		Remote: registry.RemoteServer{
			Host: "127.0.0.1",
			User: "ubuntu",
		},
	}, failingRuntimeRunner{})

	assertEqual(t, state, "unreachable")
}

func TestRunRemoteSetupReportsUnreachable(t *testing.T) {
	t.Parallel()

	err := runRemoteSetup(registry.Project{
		Runtime: registry.RuntimeTarget{Type: registry.RuntimeTypeRemote},
		Remote: registry.RemoteServer{
			Host:    "dev.example.com",
			User:    "ubuntu",
			Workdir: "/srv/api",
		},
	}, failingRuntimeRunner{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected remote setup to fail")
	}
	if !strings.Contains(err.Error(), "remote target unreachable: ubuntu@dev.example.com") {
		t.Fatalf("expected unreachable error, got %v", err)
	}
}

func TestRunRemoteSetupReportsReachable(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := runRemoteSetup(registry.Project{
		Runtime: registry.RuntimeTarget{Type: registry.RuntimeTypeRemote},
		Remote: registry.RemoteServer{
			Host:    "dev.example.com",
			User:    "ubuntu",
			Workdir: "/srv/api",
		},
	}, successfulRuntimeRunner{}, &output)
	if err != nil {
		t.Fatalf("runRemoteSetup returned error: %v", err)
	}
	if !strings.Contains(output.String(), "Remote target reachable: ubuntu@dev.example.com") {
		t.Fatalf("expected reachable output, got %q", output.String())
	}
	if !strings.Contains(output.String(), "Remote workdir ready: /srv/api") {
		t.Fatalf("expected workdir output, got %q", output.String())
	}
}

func TestRunRemoteSetupChecksWorkdir(t *testing.T) {
	t.Parallel()

	runner := &recordingRuntimeRunner{}
	err := runRemoteSetup(registry.Project{
		Runtime: registry.RuntimeTarget{Type: registry.RuntimeTypeRemote},
		Remote: registry.RemoteServer{
			Host:    "dev.example.com",
			User:    "ubuntu",
			Workdir: "/srv/api's",
		},
	}, runner, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("runRemoteSetup returned error: %v", err)
	}
	if len(runner.runs) != 2 {
		t.Fatalf("expected reachability and workdir checks, got %#v", runner.runs)
	}
	last := runner.runs[1]
	want := "'sh' '-lc' 'test -d '\\''/srv/api'\\''\"'\\''\"'\\''s'\\'''"
	if got := last[len(last)-1]; got != want {
		t.Fatalf("expected workdir check command %q, got %#v", want, last)
	}
}

func TestRunRemoteSetupChecksHostKeyOnce(t *testing.T) {
	t.Parallel()

	runner := &recordingRuntimeRunner{
		outputs: map[string][]byte{
			"ssh-keyscan -p 22 -T 5 dev.example.com": []byte("dev.example.com ssh-ed25519 YWJj\n"),
		},
	}
	err := runRemoteSetup(registry.Project{
		Runtime: registry.RuntimeTarget{Type: registry.RuntimeTypeRemote},
		Remote: registry.RemoteServer{
			Host:               "dev.example.com",
			User:               "ubuntu",
			Workdir:            "/srv/api",
			HostKeyFingerprint: "SHA256:ungWv48Bz+pBQUDeXa4iI7ADYaOWF3qctBD/YfIAFa0",
		},
	}, runner, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("runRemoteSetup returned error: %v", err)
	}
	if len(runner.outputCalls) != 1 {
		t.Fatalf("expected one host key scan, got %#v", runner.outputCalls)
	}
	if len(runner.runs) != 2 {
		t.Fatalf("expected reachability and workdir SSH checks, got %#v", runner.runs)
	}
}

func TestRunRemoteSetupReportsMissingWorkdir(t *testing.T) {
	t.Parallel()

	runner := &recordingRuntimeRunner{}
	err := runRemoteSetup(registry.Project{
		Runtime: registry.RuntimeTarget{Type: registry.RuntimeTypeRemote},
		Remote: registry.RemoteServer{
			Host:               "dev.example.com",
			User:               "ubuntu",
			HostKeyFingerprint: "SHA256:host123",
		},
	}, runner, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected missing workdir to fail")
	}
	if !strings.Contains(err.Error(), "remote.workdir is required") {
		t.Fatalf("expected remote.workdir error, got %v", err)
	}
	if len(runner.outputCalls) != 0 || len(runner.runs) != 0 {
		t.Fatalf("expected no remote calls, got output=%#v runs=%#v", runner.outputCalls, runner.runs)
	}
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
		Current:     true,
		Project:     "api",
		Runtime:     "local-vm",
		Target:      "api-vm",
		TargetState: "Running",
		VMMode:      "dedicated",
		Config:      "/tmp/api/.yard.yml",
		Path:        "/tmp/api",
	}})
	if err != nil {
		t.Fatalf("writeStatusRows returned error: %v", err)
	}

	got := output.String()
	for _, expected := range []string{"CURRENT", "PROJECT", "RUNTIME", "TARGET_STATE", "*", "api", "local-vm", "api-vm", "Running"} {
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
	for _, expected := range []string{"PROJECT", "SERVICE", "STATUS", "TARGET", "api", "web", "running", "1234", "3000", "api-vm"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected output to contain %q:\n%s", expected, got)
		}
	}
}

func TestRunGuideList(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := runGuide(args{subcommand: "list"}, &output)
	if err != nil {
		t.Fatalf("runGuide returned error: %v", err)
	}

	got := output.String()
	for _, expected := range []string{"SLUG", "STATUS", "TITLE", "smoke-test", "Full CLI Smoke Test"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected guide list to contain %q:\n%s", expected, got)
		}
	}
	if strings.Contains(got, "README") {
		t.Fatalf("expected README to be excluded:\n%s", got)
	}
}

func TestRunGuideShow(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	err := runGuide(args{subcommand: "smoke-test"}, &output)
	if err != nil {
		t.Fatalf("runGuide returned error: %v", err)
	}
	if !strings.Contains(output.String(), "# Full CLI Smoke Test") {
		t.Fatalf("expected guide markdown, got:\n%s", output.String())
	}
}

func TestRunGuideRejectsUnknownGuide(t *testing.T) {
	t.Parallel()

	err := runGuide(args{subcommand: "missing"}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected unknown guide to fail")
	}
	if !strings.Contains(err.Error(), "unknown guide: missing") {
		t.Fatalf("expected unknown guide error, got %v", err)
	}
}

func TestStartProjectServicesExecutesServicesOnTarget(t *testing.T) {
	t.Parallel()

	target := &recordingTarget{}
	err := startProjectServices(target, "api", config.ProjectConfig{
		RepoDir: "/srv/api",
		Services: []config.ServiceConfig{{
			Name:    "web",
			Command: "pnpm dev",
			Workdir: ".",
			Port:    3000,
		}},
	})
	if err != nil {
		t.Fatalf("startProjectServices returned error: %v", err)
	}
	if len(target.commands) != 1 {
		t.Fatalf("expected one command, got %#v", target.commands)
	}
	assertEqual(t, target.commands[0][0], "sh")
	assertEqual(t, target.commands[0][1], "-lc")
	for _, expected := range []string{"/srv/api", "pnpm dev", ".yard/processes/api/web"} {
		if !strings.Contains(target.commands[0][2], expected) {
			t.Fatalf("expected start script to contain %q:\n%s", expected, target.commands[0][2])
		}
	}
}

func TestStopProjectServicesStopsServicesInReverseOrder(t *testing.T) {
	t.Parallel()

	target := &recordingTarget{}
	err := stopProjectServices(target, "api", config.ProjectConfig{
		Services: []config.ServiceConfig{
			{Name: "web"},
			{Name: "worker"},
		},
	})
	if err != nil {
		t.Fatalf("stopProjectServices returned error: %v", err)
	}
	if len(target.commands) != 2 {
		t.Fatalf("expected two commands, got %#v", target.commands)
	}
	if !strings.Contains(target.commands[0][2], ".yard/processes/api/worker") {
		t.Fatalf("expected worker to stop first:\n%s", target.commands[0][2])
	}
	if !strings.Contains(target.commands[1][2], ".yard/processes/api/web") {
		t.Fatalf("expected web to stop second:\n%s", target.commands[1][2])
	}
}

func TestShouldStopProjectVM(t *testing.T) {
	t.Parallel()

	assertEqual(t, shouldStopProjectVM(registry.Project{VM: registry.VM{Mode: "dedicated"}}, false), true)
	assertEqual(t, shouldStopProjectVM(registry.Project{VM: registry.VM{Mode: "shared"}}, false), false)
	assertEqual(t, shouldStopProjectVM(registry.Project{VM: registry.VM{Mode: "shared"}}, true), true)
}

func TestRunStopRejectsRemoteVMFlagBeforeConfigLoad(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	registryPath := filepath.Join(dir, "config.yaml")
	reg, err := registry.New().Add("remote", registry.Project{
		Path:    dir,
		Config:  filepath.Join(dir, "missing.yard.yml"),
		Runtime: registry.RuntimeTarget{Type: registry.RuntimeTypeRemote},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := registry.Save(registryPath, reg); err != nil {
		t.Fatal(err)
	}

	err = runStop(args{
		positionals:  []string{"remote"},
		registryPath: registryPath,
		stopVM:       true,
	})
	if err == nil {
		t.Fatal("expected remote --vm stop to fail")
	}
	if !strings.Contains(err.Error(), "--vm requires a local-vm runtime target") {
		t.Fatalf("expected remote --vm error, got %v", err)
	}
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

type failingRuntimeRunner struct{}

func (failingRuntimeRunner) Output(string, ...string) ([]byte, error) {
	return nil, errors.New("runner failed")
}

func (failingRuntimeRunner) Run(string, ...string) error {
	return errors.New("runner failed")
}

type successfulRuntimeRunner struct{}

func (successfulRuntimeRunner) Output(string, ...string) ([]byte, error) {
	return nil, nil
}

func (successfulRuntimeRunner) Run(string, ...string) error {
	return nil
}

type recordingRuntimeRunner struct {
	outputs     map[string][]byte
	outputCalls [][]string
	runs        [][]string
}

func (runner *recordingRuntimeRunner) Output(command string, args ...string) ([]byte, error) {
	call := append([]string{command}, args...)
	runner.outputCalls = append(runner.outputCalls, call)
	if output, ok := runner.outputs[strings.Join(call, " ")]; ok {
		return output, nil
	}
	return nil, nil
}

func (runner *recordingRuntimeRunner) Run(command string, args ...string) error {
	runner.runs = append(runner.runs, append([]string{command}, args...))
	return nil
}

type recordingVMDeleteClient struct {
	statuses map[string]lima.Instance
	deleted  []string
}

func (client *recordingVMDeleteClient) Status(name string) (lima.Instance, error) {
	instance, ok := client.statuses[name]
	if !ok {
		return lima.Instance{}, errors.New("VM not found: " + name)
	}
	return instance, nil
}

func (client *recordingVMDeleteClient) Delete(name string) error {
	client.deleted = append(client.deleted, name)
	return nil
}

type recordingTarget struct {
	commands [][]string
}

func (target *recordingTarget) Exec(command []string) error {
	target.commands = append(target.commands, append([]string(nil), command...))
	return nil
}

func (target *recordingTarget) ExecOutput([]string) ([]byte, error) {
	return nil, nil
}

func slicesEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
