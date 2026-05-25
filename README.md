# Yard

Yard is a Go CLI for provisioning and operating isolated development environments from the host machine.

The product surface is `yard`. Project configuration is `.yard.yml`. No other project config filename is supported.

CLI-visible text stays in English: help, errors, prompts, confirmations, and tables.

## Status

Current capabilities:

- host-side project registry;
- local development VM runtime through Lima;
- remote server runtime through SSH;
- `.yard.yml` project config scaffolding;
- host-side project import with SSH identity metadata;
- runtime `setup`, `status`, and `exec`;
- service `start`, `stop`, `logs`, and `process list`;
- Lima VM `list`, `status`, `start`, `stop`, `delete`, and `exec`;
- remote SSH host key fingerprint inspection;
- embedded CLI scenario guides through `yard guide`.

Remote server support is implemented and has a local OrbStack smoke path. Treat it as `Experimental` until it is exercised against more real SSH hosts.

## Requirements

Development requirements:

- Go, version declared in [go.mod](go.mod)
- Lima, for local VM runtime tests and usage
- SSH client tools

Optional tools:

- OrbStack, to simulate a remote SSH server for smoke testing
- GitHub CLI, only for publishing or repository automation

Node and pnpm are not required by Yard itself.

## Run From Source

```bash
go run ./cmd/yard --help
```

The user documentation writes commands as `yard ...`. When running from this repository, replace `yard` with:

```bash
go run ./cmd/yard
```

Run the project check:

```bash
./scripts/check.sh
```

The check validates Yard help, the example project config, and all Go tests. GitHub Actions runs the same script in [.github/workflows/check.yml](.github/workflows/check.yml).

## Quick Start: Local VM

Create a project config:

```bash
yard init web-app --yes --config .yard.yml
```

Register the project with the local VM runtime:

```bash
yard project add web-app /path/to/web-app --runtime local-vm
yard use web-app
```

Create or verify the runtime target, then start services:

```bash
yard setup web-app
yard start web-app
yard status web-app
yard process logs web-app web --tail 80
```

Stop services:

```bash
yard stop web-app
```

Delete a stopped Lima VM when it is no longer needed:

```bash
yard vm delete yard-shared
```

## Quick Start: Remote Server

Get the remote SSH host key fingerprint:

```bash
yard ssh host-key dev.example.com --port 22
```

Register a remote runtime target:

```bash
yard project add api /path/to/api \
  --runtime remote-server \
  --remote-host dev.example.com \
  --remote-user ubuntu \
  --remote-workdir /home/ubuntu/workspaces/api \
  --remote-host-key SHA256:...
```

Verify and use the remote target:

```bash
yard setup api
yard exec api -- pwd
yard process list api
yard start api
yard stop api
```

`yard stop` stops Yard-managed services on a remote target. It does not stop or delete the remote machine itself.

## Project Config

The canonical project config file is `.yard.yml`.

Example:

```yaml
org: acme
project: web-app
repo: git@github.com:acme/web-app.git
vm_name: web-app-dev
host: web-app.test
vm_user: ubuntu
repo_dir: /home/ubuntu/workspaces/web-app
vm:
  provider: auto
  type: vz
services:
  web:
    command: pnpm dev --host 0.0.0.0
    workdir: .
    port: 3000
  worker:
    command: pnpm worker
    workdir: .
resources:
  cpus: 4
  memory: 6G
  disk: 50G
ports:
  web: 3000
  preview: 4173
```

Reusable example: [examples/web-app.yard.yml](examples/web-app.yard.yml)

## Host Registry

Yard stores host-local choices in:

```text
~/.config/yard/config.yaml
```

The registry can contain:

- current project;
- local project path;
- project config path;
- runtime target: `local-vm` or `remote-server`;
- local VM mode and name;
- remote SSH metadata;
- host-side Git identity metadata.

Real secrets must never be stored in the repository, `.yard.yml`, the host registry, a Dev VM, or a Remote Server by Yard. Runtime secrets must come from an external provider.

## Documentation

Primary docs:

- [CLI Command Gallery](docs/cli/README.md): scenario-oriented user docs.
- [Full CLI smoke test](docs/cli/smoke-test.md): complete local VM, remote SSH, and cleanup path.
- [Canonical Yard surface ADR](docs/adr/0001-yard-canonical-product-surface.md): product naming and config-surface decision.
- [Domain context](CONTEXT.md): project language, boundaries, and registry shape.
- [Agent instructions](AGENTS.md): contribution constraints for coding agents.

The Command Gallery is also available from the CLI:

```bash
yard guide list
yard guide smoke-test
```

## Development

Before committing:

```bash
./scripts/check.sh
```

Branch and commit conventions are documented in [AGENTS.md](AGENTS.md).

## Feature Status Labels

- `Available`: implemented and covered by automated checks.
- `Experimental`: implemented, but still useful to validate in real environments.
- `Planned`: not implemented yet.
