# Agents Guide - Yard

## Communication

- Keep responses short and technical.
- User-visible CLI text (`help`, errors, prompts, tables, confirmations) must be in English.
- Never store real secrets in the repository.

## Git

- Branches: `feat/<topic>`, `fix/<topic>`, `docs/<topic>`, `chore/<topic>`.
- Commits: Conventional Commits.
- Before committing: run `./scripts/check.sh`.

## Scope

- `yard` is a generic CLI for provisioning and operating isolated development environments.
- The canonical project config lives in `.yard.yml` in the target repository.
- Real secrets come from an external runtime provider, never from a versioned file.

## Documentation

- User-facing CLI documentation lives in `docs/cli/`.
- Update Command Gallery stories when CLI behavior changes.
- Use `yard` and `.yard.yml` in user-facing documentation.
- Mark unfinished features as `Experimental` or `Planned`.
