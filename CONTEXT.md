# devctl

`devctl` pilote des environnements de developpement isoles depuis la machine hote, sans stocker de secrets reels dans les repos ou les VMs.

## Language

**Host Controller**:
Le processus `devctl` execute sur la machine hote, responsable de l'orchestration et de l'acces aux credentials locaux.
_Avoid_: agent remote, CLI VM

**Project**:
Un repo applicatif enregistre et pilotable par `devctl`.
_Avoid_: app, workspace

**Project Registry**:
La configuration globale hote qui associe un nom de projet a son repo local et a ses choix de runtime locaux.
_Avoid_: `.devctl.yml` pour les preferences propres a une machine

**Dev VM**:
Une VM de developpement isolee qui execute les outils runtime sans credentials persistants.
_Avoid_: host, devcontainer

**Environment**:
L'ensemble actif forme par un **Project**, une **Dev VM** et les services/processus necessaires au developpement.
_Avoid_: stack quand on parle aussi de VM et processus app

**Start**:
Une action idempotente et non destructive qui amene un **Environment** configure a l'etat actif.
_Avoid_: reset, rebuild, restart

**Process**:
Un service de developpement lance pour un **Project**, supervise dans la **Dev VM** et observable depuis le **Host Controller**.
_Avoid_: terminal bloque, commande ad hoc

## Relationships

- Un **Host Controller** pilote un ou plusieurs **Projects**.
- Un **Project** est declare dans un **Project Registry** local a la machine hote.
- Le **Project Registry** vit par defaut dans `~/.config/devctl/config.yaml`.
- Un **Project** peut utiliser une **Dev VM** partagee ou dediee selon `vm.mode` dans le **Project Registry**.
- Un **Environment** appartient a exactement un **Project**.
- **Start** reutilise les ressources deja demarrees au lieu de dupliquer ou detruire des processus.
- Un **Process** expose au minimum un etat, un PID ou identifiant equivalent, des ports et des logs consultables depuis le **Host Controller**.

## Example dialogue

> **Dev:** "Est-ce qu'un projet doit toujours avoir sa VM dediee ?"
> **Domain expert:** "Non. C'est un choix local dans le **Project Registry** via `vm.mode`; le repo de projet ne doit pas imposer ca a toutes les machines."
>
> **Dev:** "Quand `start` lance le serveur app, est-ce que mon terminal reste bloque ?"
> **Domain expert:** "Non. Le **Process** est supervise dans la **Dev VM**, et le **Host Controller** permet de voir son etat et ses logs."

## Flagged ambiguities

- "dedicated" signifie maintenant `vm.mode: dedicated` dans le **Project Registry**, pas une option versionnee dans `.devctl.yml`.
- Les noms comme `lmdlp` sont des exemples de **Project**, jamais des cas hardcodes dans `devctl`.
- `start` signifie demarrage idempotent et non destructif; les actions destructives appartiennent a `reset` ou a des commandes explicitement confirmees.
- "process ouvert" signifie un **Process** observable et controle par `devctl status/logs`, pas un terminal interactif laisse ouvert.

## Registry shape

```yaml
current_project: example
projects:
  example:
    path: /Users/me/workspaces/example
    config: /Users/me/workspaces/example/.devctl.yml
    vm:
      mode: shared
      name: devctl-shared
```

`config` est optionnel et vaut `<path>/.devctl.yml` par defaut. `vm.mode` vaut `shared` par defaut, et `vm.name` vaut `devctl-shared` quand le mode est partage.
