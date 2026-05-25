# Agents Guide - Yard

## Communication

- Reponses courtes, techniques, en francais.
- Les textes visibles par les utilisateurs de la CLI (`help`, erreurs, prompts, tables, confirmations) doivent etre en anglais.
- Ne jamais stocker de secret reel dans le repo.

## Git

- Branches: `feat/<topic>`, `fix/<topic>`, `docs/<topic>`, `chore/<topic>`.
- Commits: Conventional Commits.
- Avant commit: executer `./scripts/check.sh`.

## Scope

- `yard` est une CLI generique pour provisionner et piloter des environnements dev isoles.
- La configuration projet canonique vit dans `.yard.yml` dans le repo cible.
- Les secrets reels viennent d'un fournisseur externe au runtime, jamais d'un fichier versionne.

## Documentation

- La documentation utilisateur CLI vit dans `docs/cli/`.
- Mettre a jour les stories de la Command Gallery quand le comportement CLI change.
- Utiliser `yard` et `.yard.yml` dans la documentation utilisateur.
- Marquer les features non finalisees comme `Experimental` ou `Planned`.
