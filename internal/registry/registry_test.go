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
	assertEqual(t, project.Runtime.Type, "local-vm")
	assertEqual(t, project.Config, "/tmp/example/.devctl.yml")
	assertEqual(t, project.VM.Mode, "shared")
	assertEqual(t, project.VM.Name, "yard-shared")
}

func TestAddProjectAcceptsRemoteRuntimeTarget(t *testing.T) {
	t.Parallel()

	reg, err := New().Add("example", Project{
		Path:    "/tmp/example",
		Runtime: RuntimeTarget{Type: "remote-server"},
		Remote: RemoteServer{
			Host:               "dev.example.com",
			User:               "ubuntu",
			Port:               2222,
			Workdir:            "/home/ubuntu/workspaces/example",
			IdentityFile:       "/tmp/ssh/yard_remote_ed25519",
			HostKeyFingerprint: "SHA256:host123",
		},
	})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	project := reg.Projects["example"]
	assertEqual(t, project.Runtime.Type, "remote-server")
	assertEqual(t, project.Remote.Host, "dev.example.com")
	assertEqual(t, project.Remote.User, "ubuntu")
	assertEqual(t, project.Remote.Port, 2222)
	assertEqual(t, project.Remote.Workdir, "/home/ubuntu/workspaces/example")
	assertEqual(t, project.Remote.IdentityFile, "/tmp/ssh/yard_remote_ed25519")
	assertEqual(t, project.Remote.HostKeyFingerprint, "SHA256:host123")
	assertEqual(t, project.VM.Mode, "")
	assertEqual(t, project.VM.Name, "")
}

func TestAddProjectRejectsInvalidRemotePort(t *testing.T) {
	t.Parallel()

	_, err := New().Add("example", Project{
		Path:   "/tmp/example",
		Remote: RemoteServer{Port: 70000},
	})
	if err == nil {
		t.Fatal("expected invalid remote port to fail")
	}
	if !strings.Contains(err.Error(), "unsupported remote.port") {
		t.Fatalf("expected remote.port error, got %v", err)
	}
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

func TestRemoveProject(t *testing.T) {
	t.Parallel()

	reg, err := New().Add("example", Project{Path: "/tmp/example"})
	if err != nil {
		t.Fatal(err)
	}
	reg, err = reg.Add("api", Project{Path: "/tmp/api"})
	if err != nil {
		t.Fatal(err)
	}

	reg, err = reg.Remove("api")
	if err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if _, ok := reg.Projects["api"]; ok {
		t.Fatal("expected api to be removed")
	}
	assertEqual(t, reg.CurrentProject, "example")
}

func TestRemoveCurrentProjectClearsCurrent(t *testing.T) {
	t.Parallel()

	reg, err := New().Add("example", Project{Path: "/tmp/example"})
	if err != nil {
		t.Fatal(err)
	}
	reg, err = reg.Remove("example")
	if err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}

	assertEqual(t, reg.CurrentProject, "")
	if len(reg.Projects) != 0 {
		t.Fatalf("expected no projects, got %#v", reg.Projects)
	}
}

func TestRemoveRejectsUnknownProject(t *testing.T) {
	t.Parallel()

	_, err := New().Remove("missing")
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
    git:
      identity_file: /tmp/ssh/yard_acme_ed25519
      fingerprint: SHA256:abc123
    runtime:
      type: remote-server
    remote:
      host: dev.example.com
      user: ubuntu
      port: 2222
      workdir: /home/ubuntu/workspaces/example
      identity_file: /tmp/ssh/yard_remote_ed25519
      host_key_fingerprint: SHA256:host123
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
	assertEqual(t, project.Git.IdentityFile, "/tmp/ssh/yard_acme_ed25519")
	assertEqual(t, project.Git.Fingerprint, "SHA256:abc123")
	assertEqual(t, project.Runtime.Type, "remote-server")
	assertEqual(t, project.Remote.Host, "dev.example.com")
	assertEqual(t, project.Remote.User, "ubuntu")
	assertEqual(t, project.Remote.Port, 2222)
	assertEqual(t, project.Remote.Workdir, "/home/ubuntu/workspaces/example")
	assertEqual(t, project.Remote.IdentityFile, "/tmp/ssh/yard_remote_ed25519")
	assertEqual(t, project.Remote.HostKeyFingerprint, "SHA256:host123")
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

func TestMarshalRegistryIncludesGitIdentity(t *testing.T) {
	t.Parallel()

	reg, err := New().Add("api", Project{
		Path: "/tmp/api",
		Git: Git{
			IdentityFile: "/tmp/ssh/yard_acme_ed25519",
			Fingerprint:  "SHA256:abc123",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	output := string(Marshal(reg))
	for _, expected := range []string{
		"    git:\n",
		"      identity_file: /tmp/ssh/yard_acme_ed25519\n",
		"      fingerprint: SHA256:abc123\n",
		"    runtime:\n",
		"      type: local-vm\n",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q:\n%s", expected, output)
		}
	}
}

func TestMarshalRemoteRuntimeOmitsVMDefaults(t *testing.T) {
	t.Parallel()

	reg, err := New().Add("api", Project{
		Path:    "/tmp/api",
		Runtime: RuntimeTarget{Type: "remote-server"},
		Remote: RemoteServer{
			Host:               "dev.example.com",
			User:               "ubuntu",
			Port:               2222,
			Workdir:            "/home/ubuntu/workspaces/api",
			IdentityFile:       "/tmp/ssh/yard_remote_ed25519",
			HostKeyFingerprint: "SHA256:host123",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	output := string(Marshal(reg))
	for _, expected := range []string{
		"    runtime:\n",
		"      type: remote-server\n",
		"    remote:\n",
		"      host: dev.example.com\n",
		"      user: ubuntu\n",
		"      port: 2222\n",
		"      workdir: /home/ubuntu/workspaces/api\n",
		"      identity_file: /tmp/ssh/yard_remote_ed25519\n",
		"      host_key_fingerprint: SHA256:host123\n",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q:\n%s", expected, output)
		}
	}
	if strings.Contains(output, "    vm:\n") {
		t.Fatalf("expected remote runtime output to omit vm defaults:\n%s", output)
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
