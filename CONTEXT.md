# Yard

Yard operates isolated development environments from the host machine, without storing real secrets in repositories or VMs.

## Language

**Host Controller**:
The `yard` process running on the host machine. It owns orchestration and access to local credentials.
_Avoid_: remote agent, VM CLI

**Project**:
An application repository registered and controlled by Yard.
_Avoid_: app, workspace

**Project Registry**:
The host-local configuration that maps a project name to its local repository and local runtime choices.
_Avoid_: `.yard.yml` for machine-local preferences

**Project Config**:
The versioned `.yard.yml` file that declares vendor-neutral metadata and services for a **Project**.
_Avoid_: Project Registry

**Dev VM**:
An isolated development VM that runs runtime tools without persistent credentials.
_Avoid_: host, devcontainer

**Remote Server**:
A real development server reachable over SSH that Yard can control as a runtime target instead of a local **Dev VM**.
_Avoid_: repository source, Git provider

**Runtime Target**:
The execution target of an **Environment**. Today this can be a local **Dev VM** or a **Remote Server** over SSH.
_Avoid_: assuming VM everywhere

**Environment**:
The active set formed by one or more **Projects**, a **Runtime Target**, and the services/processes needed for development.
_Avoid_: stack when also referring to VMs and app processes

**Start**:
An idempotent, non-destructive action that brings a configured **Environment** to an active state.
_Avoid_: reset, rebuild, restart

**Process**:
A development service launched for a **Project**, supervised in the runtime target, and observable from the **Host Controller**.
_Avoid_: blocked terminal, ad hoc command

**Service**:
A vendor-neutral declaration in project config describing a development command to supervise as a **Process**.
_Avoid_: framework adapter, required container

**Repository Source**:
A host-side provider that discovers and fetches accessible repositories, for example GitHub through `gh`.
_Avoid_: GitHub credentials in the VM

**Adapter**:
An optional integration enabled by a **Project** for a specific tool such as Supabase, Infisical, Vite, or a backend framework.
_Avoid_: mandatory core dependency

## Relationships

- A **Host Controller** controls one or more **Projects**.
- User-visible text in the `yard` CLI is English: help, errors, interactive prompts, confirmations, and tables.
- A **Project** is declared in a host-local **Project Registry**.
- A **Project** can have a versioned **Project Config** in `.yard.yml`.
- `.yard.yml` is the only supported **Project Config** filename.
- The **Project Registry** lives at `~/.config/yard/config.yaml` by default.
- A **Project** can use a shared or dedicated **Dev VM** through `vm.mode` in the **Project Registry**.
- A **Project** has an explicit **Runtime Target** choice: local **Dev VM** or **Remote Server**.
- The local VM vs remote server choice is host-local registry state, not a versioned `.yard.yml` option.
- Non-secret **Remote Server** metadata lives in the **Project Registry** under `remote`: host, user, SSH port, remote workdir, optional host-local identity path, and host key fingerprint.
- Core code must not assume that the **Runtime Target** is always a local VM; `start`, `stop`, `status`, `exec`, and `process` must go through a target interface.
- A **Remote Server** is an execution target, not a repository source. Git discovery and Git credentials stay host-side through **Repository Source** integrations.
- Real secrets must not be stored by Yard in the **Project Registry**, `.yard.yml`, a **Dev VM**, or a **Remote Server**.
- A **Project Registry** can store host-side Git identity metadata (`git.identity_file`, `git.fingerprint`) to test and clone a repository without writing that state to `.yard.yml`.
- An **Environment** can be single-project today and multi-project later for frontends, backends, workers, or services split across repositories.
- **Start** reuses already running resources instead of duplicating or destroying processes.
- A **Process** exposes at least state, a PID or equivalent identifier, ports, and logs readable from the **Host Controller**.
- A **Service** can represent a frontend, backend, worker, or any other development server in the same repository. If the backend lives in another repository, it becomes another **Project**.
- A **Repository Source** runs on the host and reuses existing host credentials.
- Yard core stays vendor-neutral. Frontend, backend, secrets, and service-specific tools belong in **Adapters**.
- `yard vm ...` commands operate an existing **Dev VM**. Creation and provisioning remain separate setup actions.
- `yard status` shows a dense table of **Projects** and **Dev VM** state, similar to `docker ps`.
- `yard setup` creates a missing **Dev VM** idempotently. Software provisioning remains a separate step.
- `yard start` orchestrates the **Dev VM** and configured **Services** without duplicating active **Processes**.
- `yard stop` stops **Services**. A shared **Dev VM** stays active unless `--vm` is passed.
- `yard init` creates a vendor-neutral project config with **Services**, without secrets or mandatory adapters. Overwriting requires `--force`.
- `yard project import` without arguments starts an SSH wizard: select an existing key or create a host-side key with `ssh-keygen`, with optional upload through `gh`.
- `yard project import` tests repository access with a host-side SSH identity, clones into a missing or empty directory, then registers the **Project** in the **Project Registry**.
- `yard project inspect` shows local paths, the target **Dev VM**, and host-side Git identity metadata for a **Project**.
- `yard project remove` only removes the **Project Registry** entry. It does not delete the local repository or the **Dev VM**.
- `yard ssh host-key <host>` prints public fingerprints for a **Remote Server** so they can be stored as `remote.host_key_fingerprint`.
- `yard project add/import --runtime remote-server` requires non-secret SSH metadata through prompts or `--remote-*` flags.
- `yard exec <remote-project> -- <command>`, `yard process ...`, `yard start`, and `yard stop` use SSH directly for `remote-server`. `remote.workdir` replaces `repo_dir` in remote process scripts.
- `yard setup <remote-project>` checks SSH reachability and the existence of `remote.workdir`; it does not perform destructive bootstrap, directory creation, or tool installation.
- Interactive commands must always keep an equivalent non-interactive mode through arguments or files.

## Example Dialogue

> **Developer:** "Does a project always need its own VM?"
> **Domain expert:** "No. That is a local choice in the **Project Registry** through `vm.mode`; the project repository must not impose it on every machine."
>
> **Developer:** "When `start` launches the app server, does my terminal stay blocked?"
> **Domain expert:** "No. The **Process** is supervised in the runtime target, and the **Host Controller** can read its state and logs."
>
> **Developer:** "If the backend is in another repository, is it still the same project?"
> **Domain expert:** "No. Each repository remains a **Project**. Later, an **Environment** can compose multiple **Projects**."
>
> **Developer:** "To fetch a repository from a GitHub organization, does the VM need my GitHub credentials?"
> **Domain expert:** "No. GitHub is a host-side **Repository Source**, probably through `gh`; Yard then clones or syncs without persisting credentials in the **Dev VM**."

## Flagged Ambiguities

- "dedicated" means `vm.mode: dedicated` in the **Project Registry**, not a versioned `.yard.yml` option.
- "local VM or remote server" means a **Runtime Target** choice in the **Project Registry**; `vm.mode` remains a detail of the local **Dev VM** choice.
- Historical project names should not appear in user-facing examples; use generic names like `web-app`, `api`, or `worker`.
- `start` means idempotent and non-destructive startup. Destructive actions belong to `reset` or explicitly confirmed commands.
- "open process" means a **Process** observable and controlled through `yard status` and logs, not a leftover interactive terminal.
- "backend" is not a special **Project** type. It is often a **Process** in the same repository, or another **Project** composed later in a multi-project **Environment**.
- `services` describes generic commands. NestJS, PHP, Vite, Supabase, or other choices belong in the command or adapters, not in core.
- "GitHub org" is a **Repository Source** capability, not a hardcoded Yard core assumption.
- The SSH identity selected during import is host-side state. It must not be copied into the **Dev VM** or `.yard.yml`.
- "remote server" means **Remote Server** as a **Runtime Target**, not a replacement for the **Host Controller** or a place to store secrets.
- Supabase, Infisical, and Vite are optional **Adapters**, not prerequisites for all **Projects**.

## Registry Shape

```yaml
current_project: example
projects:
  example:
    path: /Users/me/workspaces/example
    config: /Users/me/workspaces/example/.yard.yml
    git:
      identity_file: /Users/me/.ssh/yard_acme_ed25519
      fingerprint: SHA256:abc123
    runtime:
      type: local-vm
    vm:
      mode: shared
      name: yard-shared
  remote-api:
    path: /Users/me/workspaces/api
    runtime:
      type: remote-server
    remote:
      host: dev.example.com
      user: ubuntu
      port: 22
      workdir: /home/ubuntu/workspaces/api
      identity_file: /Users/me/.ssh/yard_remote_ed25519
      host_key_fingerprint: SHA256:...
```

`config` is optional and defaults to `<path>/.yard.yml`. `git` is optional and remains local to the host machine. `runtime.type` defaults to `local-vm` and can be `remote-server` for an SSH target. `remote` is optional, host-local, and stores only non-secret metadata. `remote.identity_file` is a path to a host private key, never the key content. `remote.host_key_fingerprint` is a non-secret host key fingerprint checked before connection when provided. `vm.mode` defaults to `shared`, and `vm.name` defaults to `yard-shared` in shared mode.
