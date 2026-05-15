# Agents Guide - devctl

## Communication

- Reponses courtes, techniques, en francais.
- Ne jamais stocker de secret reel dans le repo.

## Git

- Branches: `feat/<topic>`, `fix/<topic>`, `docs/<topic>`, `chore/<topic>`.
- Commits: Conventional Commits.
- Avant commit: executer `npm run check`.

## Scope

- `devctl` est une CLI generique pour provisionner et piloter des environnements dev isoles.
- La configuration projet vit dans `.devctl.yml` dans le repo cible.
- Les secrets reels viennent d'un fournisseur externe au runtime, jamais d'un fichier versionne.

