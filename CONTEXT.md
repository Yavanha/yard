# Yard

Yard pilote des environnements de developpement isoles depuis la machine hote, sans stocker de secrets reels dans les repos ou les VMs.

## Language

**Host Controller**:
Le processus `yard` execute sur la machine hote, responsable de l'orchestration et de l'acces aux credentials locaux.
_Avoid_: agent remote, CLI VM

**Project**:
Un repo applicatif enregistre et pilotable par Yard.
_Avoid_: app, workspace

**Project Registry**:
La configuration globale hote qui associe un nom de projet a son repo local et a ses choix de runtime locaux.
_Avoid_: `.devctl.yml` pour les preferences propres a une machine

**Dev VM**:
Une VM de developpement isolee qui execute les outils runtime sans credentials persistants.
_Avoid_: host, devcontainer

**Environment**:
L'ensemble actif forme par un ou plusieurs **Projects**, une **Dev VM** et les services/processus necessaires au developpement.
_Avoid_: stack quand on parle aussi de VM et processus app

**Start**:
Une action idempotente et non destructive qui amene un **Environment** configure a l'etat actif.
_Avoid_: reset, rebuild, restart

**Process**:
Un service de developpement lance pour un **Project**, supervise dans la **Dev VM** et observable depuis le **Host Controller**.
_Avoid_: terminal bloque, commande ad hoc

**Repository Source**:
Un fournisseur host-side qui permet de decouvrir et recuperer des repos accessibles, par exemple GitHub via `gh`.
_Avoid_: credentials GitHub dans la VM

**Adapter**:
Une integration optionnelle activee par un **Project** pour un outil specifique comme Supabase, Infisical, Vite ou un backend particulier.
_Avoid_: dependance coeur obligatoire

## Relationships

- Un **Host Controller** pilote un ou plusieurs **Projects**.
- Un **Project** est declare dans un **Project Registry** local a la machine hote.
- Le **Project Registry** vit par defaut dans `~/.config/yard/config.yaml`.
- Un **Project** peut utiliser une **Dev VM** partagee ou dediee selon `vm.mode` dans le **Project Registry**.
- Un **Environment** peut etre mono-project maintenant et multi-project plus tard pour composer front, backend, workers ou services dans des repos differents.
- **Start** reutilise les ressources deja demarrees au lieu de dupliquer ou detruire des processus.
- Un **Process** expose au minimum un etat, un PID ou identifiant equivalent, des ports et des logs consultables depuis le **Host Controller**.
- Un **Repository Source** tourne cote host et reutilise les credentials host existants.
- Le coeur de Yard reste vendor-neutral; les outils specifiques front/backend/secrets/services passent par des **Adapters**.
- Les commandes `yard vm ...` pilotent une **Dev VM** existante; la creation/provision restent des actions de setup separees.
- `yard status` affiche une vue tableau dense des **Projects** et de l'etat des **Dev VMs**, style `docker ps`.
- `yard setup` cree la **Dev VM** manquante de maniere idempotente; le provisionnement logiciel restera une etape separee.
- Les commandes interactives doivent toujours conserver un mode non interactif equivalent via arguments ou fichiers.

## Example dialogue

> **Dev:** "Est-ce qu'un projet doit toujours avoir sa VM dediee ?"
> **Domain expert:** "Non. C'est un choix local dans le **Project Registry** via `vm.mode`; le repo de projet ne doit pas imposer ca a toutes les machines."
>
> **Dev:** "Quand `start` lance le serveur app, est-ce que mon terminal reste bloque ?"
> **Domain expert:** "Non. Le **Process** est supervise dans la **Dev VM**, et le **Host Controller** permet de voir son etat et ses logs."
>
> **Dev:** "Si le backend est dans un autre repo, est-ce encore le meme projet ?"
> **Domain expert:** "Non. Chaque repo reste un **Project**; plus tard un **Environment** pourra composer plusieurs **Projects**."
>
> **Dev:** "Pour recuperer un repo d'une organisation GitHub, est-ce que la VM doit avoir mes credentials GitHub ?"
> **Domain expert:** "Non. GitHub est une **Repository Source** cote host, probablement via `gh`, puis Yard clone/synchronise sans persister de credentials dans la **Dev VM**."

## Flagged ambiguities

- "dedicated" signifie maintenant `vm.mode: dedicated` dans le **Project Registry**, pas une option versionnee dans `.devctl.yml`.
- Les noms comme `lmdlp` sont des exemples de **Project**, jamais des cas hardcodes dans Yard.
- `start` signifie demarrage idempotent et non destructif; les actions destructives appartiennent a `reset` ou a des commandes explicitement confirmees.
- "process ouvert" signifie un **Process** observable et controle par `yard status/logs`, pas un terminal interactif laisse ouvert.
- "backend" n'est pas un type special de **Project**; c'est souvent un **Process** dans le meme repo, ou un autre **Project** compose plus tard dans un **Environment** multi-project.
- "GitHub org" est une capacite de **Repository Source**, pas une hypothese hardcodee dans le coeur de Yard.
- Supabase, Infisical et Vite sont des **Adapters** optionnels, pas des preconditions pour tous les **Projects**.

## Registry shape

```yaml
current_project: example
projects:
  example:
    path: /Users/me/workspaces/example
    config: /Users/me/workspaces/example/.devctl.yml
    vm:
      mode: shared
      name: yard-shared
```

`config` est optionnel et vaut `<path>/.devctl.yml` par defaut pendant la migration. `vm.mode` vaut `shared` par defaut, et `vm.name` vaut `yard-shared` quand le mode est partage.
