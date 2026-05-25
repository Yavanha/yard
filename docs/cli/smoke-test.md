# Full CLI Smoke Test

Status: Available for config, registry, Lima, and process management. Experimental for remote server over SSH.

Goal: test Yard end to end without touching the real host registry, then clean up every resource created during the run.

## 1. Prepare An Isolated Workspace

```bash
export SMOKE=/tmp/yard-smoke
```

Defines a disposable directory for test files. This keeps the smoke test separate from real projects.

```bash
export REG=$SMOKE/registry.yaml
```

Defines a temporary Yard registry. The normal registry lives at `~/.config/yard/config.yaml`; this test uses a separate file to avoid changing user state.

```bash
mkdir -p "$SMOKE/app"
```

Creates a fake local project. It will hold the generated `.yard.yml` and act as the project path for both Lima and remote SSH tests.

```bash
yardd() { yard --registry "$REG" "$@"; }
```

Creates a shell shortcut that injects the temporary registry into every command. `"$@"` forwards all arguments passed to `yardd` while preserving arguments that contain spaces.

When running from this source tree, use:

```bash
yardd() { go run ./cmd/yard --registry "$REG" "$@"; }
```

## 2. Check The Baseline

```bash
yard --help
```

Prints the main help output. This confirms that the CLI responds and exposes the expected command surface.

```bash
yard config --project examples/web-app.yard.yml
```

Loads a project config without using the registry. This validates `.yard.yml` parsing and JSON output.

From this repo, also run:

```bash
./scripts/check.sh
```

Runs the full repository check: Yard help, the example config, and all Go tests.

## 3. Create A Project Config

```bash
yard init smoke --yes --config "$SMOKE/app/.yard.yml" \
  --repo git@example.com:acme/smoke.git \
  --service web \
  --command "while true; do echo yard-smoke; sleep 2; done" \
  --port 3000
```

Creates a `.yard.yml` without prompts. `--yes` tells Yard to use the provided flags and default values for the remaining fields.

The `web` service is intentionally simple: it keeps running and writes logs. That makes it useful for testing `process start`, `process logs`, and `process stop` without requiring a real application.

```bash
yard config --project "$SMOKE/app/.yard.yml"
```

Reads the generated config back through Yard. This confirms that `init` produced a valid file.

## 4. Test The Host Registry

```bash
yardd project add smoke "$SMOKE/app" --config "$SMOKE/app/.yard.yml" --runtime local-vm
```

Adds the project to the temporary registry with a `local-vm` runtime. If this is the first project in the registry, it also becomes the current project.

```bash
yardd project list
```

Lists registered projects. The `smoke` project should appear.

```bash
yardd project inspect smoke
```

Shows every field stored for `smoke`: local path, config path, runtime, VM settings, and optional metadata.

```bash
yardd use smoke
```

Sets `smoke` as the current project. Later commands can then omit the project name.

```bash
yardd status
```

Shows the registry and runtime target state.

```bash
yardd config smoke
```

Resolves config through the registry: project name, registry entry, `.yard.yml` path, then final JSON.

## 5. Test The Local VM Runtime

The `local-vm` runtime uses Lima.

```bash
yardd setup smoke
```

Creates the Lima VM if it does not exist. If it already exists, Yard reports that without recreating it.

```bash
yardd vm list
```

Lists Lima VMs visible to Yard.

```bash
yardd vm status smoke
```

Resolves `smoke` to its VM name and prints detailed VM state.

```bash
yardd vm exec smoke -- mkdir -p /home/ubuntu/workspaces/smoke
```

Creates the runtime work directory inside the VM. The process manager must be able to enter `repo_dir` before starting a service.

```bash
yardd exec smoke -- uname -a
```

Runs a generic command in the project runtime. This validates SSH access to the VM.

```bash
yardd process list smoke
```

Shows configured service state. Before startup, `web` should be `stopped` or reflect the VM state.

```bash
yardd process start smoke web
```

Starts only the `web` service.

```bash
yardd process logs smoke web --tail 20
```

Reads the last service log lines. This confirms that the process is running and writing to the log file managed by Yard.

```bash
yardd process stop smoke web
```

Stops the `web` service.

```bash
yardd start smoke
```

Starts the VM if needed, then starts every configured service.

```bash
yardd stop smoke
```

Stops project services. For a shared VM, the VM stays running by default.

```bash
yardd stop smoke --vm
```

Stops services, then stops the VM as well.

## 6. Simulate A Remote Server With OrbStack

The `remote-server` runtime uses SSH. OrbStack can act as a local remote server for smoke testing.

```bash
orb create ubuntu yard-remote
```

Creates an OrbStack Linux machine named `yard-remote`.

```bash
orb -m yard-remote mkdir -p "/home/$USER/workspaces/smoke"
```

Creates the remote work directory expected by Yard. `yard setup remote` checks that the workdir exists, but does not create it.

```bash
yard ssh host-key localhost --port 32222
```

Scans the SSH host key fingerprints exposed by OrbStack.

```bash
FP=$(yard ssh host-key localhost --port 32222 | awk 'NR==2 {print $3; exit}')
```

Stores the first fingerprint in `FP` so it can be pinned in the remote project entry.

```bash
yardd project add remote "$SMOKE/app" \
  --config "$SMOKE/app/.yard.yml" \
  --runtime remote-server \
  --remote-host localhost \
  --remote-port 32222 \
  --remote-user "$USER@yard-remote" \
  --remote-workdir "/home/$USER/workspaces/smoke" \
  --remote-identity "$HOME/.orbstack/ssh/id_ed25519" \
  --remote-host-key "$FP"
```

Adds a `remote` project to the temporary registry. The entry points to the OrbStack machine through SSH and uses the remote workdir as the runtime repo directory.

```bash
yardd project inspect remote
```

Checks the remote entry: host, port, user, identity file, host key fingerprint, and workdir.

```bash
yardd setup remote
```

Checks the host key, SSH reachability, and remote workdir existence.

```bash
yardd exec remote -- uname -a
```

Runs a command on the OrbStack machine through the `remote-server` runtime.

```bash
yardd process start remote web
```

Starts the `web` service on the remote target.

```bash
yardd process list remote
```

Shows service state on the remote target.

```bash
yardd process logs remote web --tail 20
```

Reads remote service logs.

```bash
yardd process stop remote web
```

Stops the remote service.

```bash
yardd stop remote
```

Stops services managed by Yard on the remote target. This does not stop or delete the remote machine itself.

## 7. Clean Up Resources

```bash
yardd stop remote
```

Stops any remaining Yard-managed services on the remote target.

```bash
orb delete yard-remote
```

Deletes the OrbStack machine and frees its resources.

```bash
yardd project remove remote
```

Removes the remote project entry from the temporary registry.

```bash
yardd project remove smoke
```

Removes the local VM project entry from the temporary registry.

```bash
yardd vm delete yard-shared
```

Deletes the shared Lima VM created by the scenario. The VM must already be stopped; Yard does not stop it implicitly before deletion.

```bash
rm -rf "$SMOKE"
```

Deletes temporary smoke test files: generated config, temporary registry, and fake local project.

## 8. Negative Cases To Validate

```bash
yardd project inspect missing
```

Should fail with an unknown project error.

```bash
yardd project add bad "$SMOKE/app" --runtime remote-server
```

Should fail because a remote runtime requires `--remote-host`, `--remote-user`, and `--remote-workdir`.

```bash
yardd project add bad "$SMOKE/app" --runtime local-vm --remote-host localhost
```

Should fail because `--remote-*` flags are only valid with `--runtime remote-server`.

```bash
yardd process start remote missing-service
```

Should fail with an unknown service error.

```bash
yardd project add remote-bad-key "$SMOKE/app" \
  --config "$SMOKE/app/.yard.yml" \
  --runtime remote-server \
  --remote-host localhost \
  --remote-port 32222 \
  --remote-user "$USER@yard-remote" \
  --remote-workdir "/home/$USER/workspaces/smoke" \
  --remote-identity "$HOME/.orbstack/ssh/id_ed25519" \
  --remote-host-key SHA256:wrong
```

Then:

```bash
yardd setup remote-bad-key
```

Should fail with a host key mismatch.
