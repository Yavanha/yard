package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddProjectDefaults(t *testing.T) {
	t.Parallel()

	reg, err := New().Add("example", Project{Path: "/tmp/example"})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	project := reg.Projects["example"]
	assertEqual(t, reg.CurrentProject, "example")
	assertEqual(t, project.Config, "/tmp/example/.devctl.yml")
	assertEqual(t, project.VM.Mode, "shared")
	assertEqual(t, project.VM.Name, "yard-shared")
}

func TestAddDedicatedProjectDefaultsVMName(t *testing.T) {
	t.Parallel()

	reg, err := New().Add("example", Project{
		Path: "/tmp/example",
		VM:   VM{Mode: "dedicated"},
	})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	project := reg.Projects["example"]
	assertEqual(t, project.VM.Mode, "dedicated")
	assertEqual(t, project.VM.Name, "example-dev")
}

func TestUseRejectsUnknownProject(t *testing.T) {
	t.Parallel()

	_, err := New().Use("missing")
	if err == nil {
		t.Fatal("expected unknown project to fail")
	}
}

func TestResolveUsesCurrentProject(t *testing.T) {
	t.Parallel()

	reg, err := New().Add("example", Project{Path: "/tmp/example"})
	if err != nil {
		t.Fatal(err)
	}

	name, project, err := reg.Resolve("")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	assertEqual(t, name, "example")
	assertEqual(t, project.Path, "/tmp/example")
}

func TestResolveUsesNamedProject(t *testing.T) {
	t.Parallel()

	reg, err := New().Add("example", Project{Path: "/tmp/example"})
	if err != nil {
		t.Fatal(err)
	}
	reg, err = reg.Add("api", Project{Path: "/tmp/api"})
	if err != nil {
		t.Fatal(err)
	}

	name, project, err := reg.Resolve("api")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	assertEqual(t, name, "api")
	assertEqual(t, project.Path, "/tmp/api")
}

func TestResolveRejectsMissingCurrentProject(t *testing.T) {
	t.Parallel()

	_, _, err := New().Resolve("")
	if err == nil {
		t.Fatal("expected missing current project to fail")
	}
}

func TestParseRegistry(t *testing.T) {
	t.Parallel()

	reg, err := Parse([]byte(`current_project: example
projects:
  example:
    path: /tmp/example
    config: /tmp/example/custom.yml
    vm:
      mode: dedicated
      name: example-vm
`))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	project := reg.Projects["example"]
	assertEqual(t, reg.CurrentProject, "example")
	assertEqual(t, project.Path, "/tmp/example")
	assertEqual(t, project.Config, "/tmp/example/custom.yml")
	assertEqual(t, project.VM.Mode, "dedicated")
	assertEqual(t, project.VM.Name, "example-vm")
}

func TestMarshalRegistrySortsProjects(t *testing.T) {
	t.Parallel()

	reg := New()
	var err error
	reg, err = reg.Add("zeta", Project{Path: "/tmp/zeta"})
	if err != nil {
		t.Fatal(err)
	}
	reg, err = reg.Add("alpha", Project{Path: "/tmp/alpha"})
	if err != nil {
		t.Fatal(err)
	}
	reg, err = reg.Use("alpha")
	if err != nil {
		t.Fatal(err)
	}

	output := string(Marshal(reg))
	if !strings.Contains(output, "current_project: alpha\n") {
		t.Fatalf("missing current project in output:\n%s", output)
	}
	if strings.Index(output, "  alpha:") > strings.Index(output, "  zeta:") {
		t.Fatalf("expected projects to be sorted:\n%s", output)
	}
}

func TestSaveAndLoad(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "yard", "config.yaml")
	reg, err := New().Add("example", Project{Path: "/tmp/example"})
	if err != nil {
		t.Fatal(err)
	}
	if err := Save(path, reg); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("expected 0600 permissions, got %v", info.Mode().Perm())
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	assertEqual(t, loaded.CurrentProject, "example")
}

func assertEqual(t *testing.T, got any, want any) {
	t.Helper()
	if got != want {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
}
