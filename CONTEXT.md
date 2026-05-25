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
_Avoid_: `.yard.yml` pour les preferences propres a une machine

**Project Config**:
Le fichier versionne `.yard.yml` qui declare les metadonnees et services vendor-neutral d'un **Project**.
_Avoid_: Project Registry

**Dev VM**:
Une VM de developpement isolee qui execute les outils runtime sans credentials persistants.
_Avoid_: host, devcontainer

**Remote Server**:
Un serveur de developpement reel, accessible a distance, que Yard pourra piloter comme cible runtime au lieu d'une **Dev VM** locale.
_Avoid_: repository source, fournisseur Git

**Runtime Target**:
La cible d'execution d'un **Environment**. Aujourd'hui une **Dev VM**; plus tard possiblement un **Remote Server** via SSH.
_Avoid_: supposer VM partout

**Environment**:
L'ensemble actif forme par un ou plusieurs **Projects**, un **Runtime Target** et les services/processus necessaires au developpement.
_Avoid_: stack quand on parle aussi de VM et processus app

**Start**:
Une action idempotente et non destructive qui amene un **Environment** configure a l'etat actif.
_Avoid_: reset, rebuild, restart

**Process**:
Un service de developpement lance pour un **Project**, supervise dans la **Dev VM** et observable depuis le **Host Controller**.
_Avoid_: terminal bloque, commande ad hoc

**Service**:
Une declaration vendor-neutral dans la config projet qui decrit une commande de dev a superviser comme **Process**.
_Avoid_: adapter framework, container obligatoire

**Repository Source**:
Un fournisseur host-side qui permet de decouvrir et recuperer des repos accessibles, par exemple GitHub via `gh`.
_Avoid_: credentials GitHub dans la VM

**Adapter**:
Une integration optionnelle activee par un **Project** pour un outil specifique comme Supabase, Infisical, Vite ou un backend particulier.
_Avoid_: dependance coeur obligatoire

## Relationships

- Un **Host Controller** pilote un ou plusieurs **Projects**.
- Les textes visibles dans la CLI `yard` sont en anglais: aide, erreurs, prompts interactifs, confirmations et tables.
- Un **Project** est declare dans un **Project Registry** local a la machine hote.
- Un **Project** peut avoir un **Project Config** versionne dans `.yard.yml`.
- `.yard.yml` est le seul **Project Config** supporte.
- Le **Project Registry** vit par defaut dans `~/.config/yard/config.yaml`.
- Un **Project** peut utiliser une **Dev VM** partagee ou dediee selon `vm.mode` dans le **Project Registry**.
- Un **Project** devra proposer un choix explicite de **Runtime Target**: **Dev VM** locale ou **Remote Server**, pour les usages de travail a distance ou de machine de dev partagee.
- Le choix **Dev VM** locale vs **Remote Server** est une preference host-local dans le **Project Registry**, pas une option versionnee dans `.yard.yml`.
- Les metadonnees **Remote Server** non secretes vivent dans le **Project Registry** sous `remote`: host, user, port SSH, repertoire de travail distant, et chemin host-local optionnel vers une identite SSH.
- Le coeur ne doit pas supposer que le **Runtime Target** est toujours une VM locale; les operations `start`, `stop`, `status`, `exec` et `process` doivent pouvoir passer par une interface cible.
- Un **Remote Server** reste une cible d'execution, pas une source de repo: la decouverte Git et les credentials Git restent host-side via **Repository Source**.
- Les secrets reels ne doivent pas etre stockes dans le **Project Registry**, dans `.yard.yml`, dans la **Dev VM** ou sur un **Remote Server** par Yard.
- Un **Project Registry** peut stocker une identite Git host-side (`git.identity_file`, `git.fingerprint`) pour tester et cloner un repo sans l'inscrire dans `.yard.yml`.
- Un **Environment** peut etre mono-project maintenant et multi-project plus tard pour composer front, backend, workers ou services dans des repos differents.
- **Start** reutilise les ressources deja demarrees au lieu de dupliquer ou detruire des processus.
- Un **Process** expose au minimum un etat, un PID ou identifiant equivalent, des ports et des logs consultables depuis le **Host Controller**.
- Un **Service** peut representer un front, un backend, un worker ou tout autre serveur de dev dans le meme repo; si le backend vit dans un autre repo, il devient un autre **Project**.
- Un **Repository Source** tourne cote host et reutilise les credentials host existants.
- Le coeur de Yard reste vendor-neutral; les outils specifiques front/backend/secrets/services passent par des **Adapters**.
- Les commandes `yard vm ...` pilotent une **Dev VM** existante; la creation/provision restent des actions de setup separees.
- `yard status` affiche une vue tableau dense des **Projects** et de l'etat des **Dev VMs**, style `docker ps`.
- `yard setup` cree la **Dev VM** manquante de maniere idempotente; le provisionnement logiciel restera une etape separee.
- `yard start` orchestre la **Dev VM** et les **Services** configures sans doubler les **Processes** deja actifs.
- `yard stop` arrete les **Services**; une **Dev VM** partagee reste active sauf demande explicite avec `--vm`.
- `yard init` cree une config projet vendor-neutral avec **Services**, sans secrets ni adapters obligatoires; l'ecrasement requiert `--force`.
- `yard project import` sans arguments lance un wizard SSH: selection de cle existante ou creation host-side via `ssh-keygen`, avec upload optionnel par `gh`; les chemins `yes` et `not sure` testent la cle choisie et proposent une creation si elle echoue.
- `yard project import` teste l'acces au repo avec une identite SSH host-side, clone dans un dossier vide ou manquant, puis enregistre le **Project** dans le **Project Registry**.
- `yard project inspect` affiche les chemins locaux, la **Dev VM** cible et l'identite Git host-side enregistree pour un **Project**.
- `yard project remove` supprime uniquement l'entree du **Project Registry**; il ne supprime pas le repo local ni la **Dev VM**.
- `yard ssh host-key <host>` affiche les fingerprints publics d'un **Remote Server** pour renseigner `remote.host_key_fingerprint`.
- `yard project add/import --runtime remote-server` exige les metadonnees SSH non secretes via prompts ou flags `--remote-*`.
- `yard exec <remote-project> -- <command>`, `yard process ...`, `yard start` et `yard stop` utilisent SSH directement pour `remote-server`. `remote.workdir` remplace `repo_dir` pour les scripts de process distants.
- `yard setup <remote-project>` verifie la reachability SSH et l'existence de `remote.workdir`; aucun bootstrap destructif, creation de repertoire ou installation d'outils n'est fait.
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

- "dedicated" signifie maintenant `vm.mode: dedicated` dans le **Project Registry**, pas une option versionnee dans `.yard.yml`.
- "local VM ou serveur distant" signifie un choix de **Runtime Target** dans le **Project Registry**; `vm.mode` reste le detail du choix **Dev VM**.
- Les noms comme `lmdlp` sont des cas projet historiques; les exemples utilisateur doivent utiliser des noms generiques comme `web-app`, `api` ou `worker`.
- `start` signifie demarrage idempotent et non destructif; les actions destructives appartiennent a `reset` ou a des commandes explicitement confirmees.
- "process ouvert" signifie un **Process** observable et controle par `yard status/logs`, pas un terminal interactif laisse ouvert.
- "backend" n'est pas un type special de **Project**; c'est souvent un **Process** dans le meme repo, ou un autre **Project** compose plus tard dans un **Environment** multi-project.
- `services` decrit des commandes generiques; les choix NestJS, PHP, Vite, Supabase ou autres restent dans la commande/adapters, pas dans le coeur.
- "GitHub org" est une capacite de **Repository Source**, pas une hypothese hardcodee dans le coeur de Yard.
- L'identite SSH choisie pour un import est un choix host-side; elle ne doit pas etre copiee dans la **Dev VM** ni dans `.yard.yml`.
- "serveur distant" signifie **Remote Server** comme **Runtime Target**, pas remplacement du **Host Controller** ni stockage de secrets sur le serveur.
- Supabase, Infisical et Vite sont des **Adapters** optionnels, pas des preconditions pour tous les **Projects**.

## Registry shape

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

`config` est optionnel et vaut `<path>/.yard.yml` par defaut. `git` est optionnel et reste local a la machine hote. `runtime.type` vaut `local-vm` par defaut et peut etre `remote-server` pour une cible SSH. `remote` est optionnel, host-local, et ne stocke que des metadonnees non secretes; `remote.identity_file` est un chemin vers une cle privee hote, jamais le contenu de la cle, et `remote.host_key_fingerprint` est une empreinte de cle hote non secrete verifiee avant connexion quand elle est fournie. `vm.mode` vaut `shared` par defaut, et `vm.name` vaut `yard-shared` quand le mode est partage.
