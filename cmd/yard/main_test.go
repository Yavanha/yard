package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"yard/internal/provider/lima"
	"yard/internal/registry"
)

func TestParseProjectAddArgs(t *testing.T) {
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
	}, "")
	if err != nil {
		t.Fatalf("buildStatusRows returned error: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	assertEqual(t, rows[0].Project, "api")
	assertEqual(t, rows[0].Current, true)
	assertEqual(t, rows[0].VMState, "missing")
	assertEqual(t, rows[1].Project, "front")
	assertEqual(t, rows[1].VMState, "Running")
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

	rows, err := buildStatusRows(reg, nil, "api")
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
	for _, expected := range []string{"CURRENT", "PROJECT", "VM_STATE", "*", "api", "api-vm", "Running"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("expected output to contain %q:\n%s", expected, got)
		}
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
