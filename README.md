# Yard

CLI generique pour creer et piloter des environnements de developpement isoles.

Le repo et la CLI Node V1 portent encore le nom historique `devctl`. Le produit et le prototype Go V2 utilisent le nom cible `yard`.

Les textes visibles par les utilisateurs de la CLI sont en anglais: aide, erreurs, prompts, confirmations et sorties tableau.

Objectif V1:
- Lima pour une VM par projet sur macOS, Linux et WSL2.
- Ansible pour provisionner la VM.
- Docker uniquement dans la VM.
- Devcontainer CLI execute dans la VM.
- Git dans la VM via SSH agent forwarding.
- Infisical cote hote pour injecter les secrets runtime sans fichier `.env` reel.

## Usage

Verification locale:

```bash
pnpm run check
```

Depuis un repo projet contenant `.devctl.yml`:

```bash
devctl doctor
devctl config
devctl init
devctl up
devctl hosts sync
devctl ssh config
devctl provision
devctl repo sync
devctl container up
devctl container rebuild
devctl app dev
devctl down
```

Si `.devctl.yml` manque, `devctl` affiche un warning et propose `devctl init`.

Commandes MVP:
- `doctor`: verifie les prerequis hote et affiche les commandes d'installation par OS.
- `up`: cree la VM Lima si elle n'existe pas, apres confirmation ressources.
- `hosts sync`: ajoute ou met a jour `127.0.0.1 <host>` dans `/etc/hosts` pour les ports forwardes par Lima.
- `ssh config`: imprime le bloc SSH Lima avec `ForwardAgent yes`.
- `provision`: lance le playbook Ansible embarque dans `devctl`.
- `repo sync`: clone le repo dans la VM ou fait un fast-forward si le repo est propre.
- `container up`: demarre le devcontainer depuis le repo dans la VM.
- `container rebuild`: recree le devcontainer existant.
- `app dev`: demarre Supabase local si configure, exporte les secrets Infisical cote hote et lance la commande dev dans le devcontainer sans ecrire de fichier `.env`.
- `down`: stoppe le devcontainer et le stack Supabase du projet. Avec `--vm`, stoppe aussi la VM Lima.

## Installation globale

Prerequis macOS:

```bash
brew install lima ansible pnpm git
```

Installer les CLIs d'adapters seulement pour les projets qui les utilisent, par exemple Infisical cote host ou Supabase CLI cote VM.

V1 locale:

```bash
pnpm link --global
devctl --help
```

Plus tard, `devctl` pourra etre publie comme package prive et installe avec:

```bash
pnpm add --global devctl
```

## Securite supply-chain

`devctl` peut etre compromis comme tout outil de developpement s'il tire du code non maitrise. En V1, la CLI reduit ce risque:

- aucune dependance runtime;
- aucun script `postinstall`;
- configuration projet explicite dans `.devctl.yml`;
- secrets reels jamais stockes dans le repo;
- versions d'outils externes a pinner avant installation dans la VM.

Quand des dependances seront ajoutees, elles devront etre limitees, pinnees dans le lockfile et auditees avant publication.

## Config projet

Exemple:

```yaml
org: lmdlp
project: lmdlp-client
repo: git@github.com:lmdlp/lmdlp_client.git
vm_name: lmdlp-dev
host: lmdlp-dev.test
vm_user: ubuntu
repo_dir: /home/ubuntu/workspaces/lmdlp_client
vm:
  provider: auto
  type: vz
supabase:
  enabled: true
  seed: start
infisical:
  project_path: /lmdlp
  default_env: dev
app:
  dev_command: pnpm dev --host 0.0.0.0
services:
  app:
    command: pnpm dev --host 0.0.0.0
    workdir: .
    port: 3000
resources:
  cpus: 4
  memory: 6G
  disk: 50G
ports:
  app: 3000
  preview: 4173
  supabase_api: 54321
  supabase_db: 54322
  supabase_studio: 54323
  mailpit: 54324
```

`vm.provider: auto` choisit Lima sur macOS, Linux et WSL2. Windows natif n'est pas supporte en V1: lancer `devctl` depuis WSL2.

Sur macOS, `vm.type: vz` evite le chemin QEMU de Multipass et contourne l'erreur Apple Silicon `host-arm-cpu.sme`.

`supabase.seed: start` laisse `supabase start` appliquer migrations et `supabase/seed.sql` au premier demarrage local. Utiliser `reset` seulement pour forcer `supabase db reset --local` a chaque `app dev`, car cela detruit les donnees locales.

Pour fin de session:

```bash
devctl down --project .devctl.yml
```

Si un stack Supabase a ete lance hors du dossier projet, ajouter `--supabase-all`. Pour tout eteindre, VM incluse:

```bash
devctl down --project .devctl.yml --supabase-all --vm
```

## Etat

V1 locale en cours: orchestration Lima, provision Ansible, sync Git VM, devcontainer, app dev avec injection Infisical runtime.

## Migration Go V2 - Yard

La CLI Node reste la reference fonctionnelle pendant la migration.

Premier slice Go:

```bash
go run ./cmd/yard config --project examples/lmdlp.devctl.yml
go test ./...
```

Objectif: porter les commandes une par une, avec tests, avant de remplacer le binaire Node.

Registre projets host:

```bash
go run ./cmd/yard project add
go run ./cmd/yard project add example /path/to/repo
go run ./cmd/yard project import
go run ./cmd/yard project import example --repo git@github.com:acme/example.git --identity ~/.ssh/yard_acme --path /path/to/repo
go run ./cmd/yard project list
go run ./cmd/yard use example
go run ./cmd/yard init example
go run ./cmd/yard init example --yes --config /path/to/repo/.devctl.yml
go run ./cmd/yard config
go run ./cmd/yard config example
go run ./cmd/yard vm list
go run ./cmd/yard vm status example
go run ./cmd/yard vm start example
go run ./cmd/yard vm stop example
go run ./cmd/yard exec example -- uname -a
go run ./cmd/yard ssh keys
go run ./cmd/yard process list example
go run ./cmd/yard process start example app
go run ./cmd/yard process logs example app --tail 80
go run ./cmd/yard process stop example app
go run ./cmd/yard start example
go run ./cmd/yard stop example
go run ./cmd/yard stop example --vm
go run ./cmd/yard status
go run ./cmd/yard status example
go run ./cmd/yard setup example
```

Le registre vit par defaut dans `~/.config/yard/config.yaml`. Les choix locaux comme `vm.mode: shared|dedicated` restent dans ce registre, pas dans `.devctl.yml`.
Sans arguments, `project add` lance un wizard et affiche le YAML du registre avant ecriture.

Notes de cadrage:
- `Project` reste un repo enregistre. Un backend separe sera donc un autre `Project`.
- Un futur `Environment` pourra composer plusieurs `Projects` pour front, backend, workers ou services.
- Dans un meme repo, declarer plusieurs `services` vendor-neutral, par exemple `web`, `api` ou `worker`; Yard ne depend pas de Nest, PHP, Vite ou Supabase pour les piloter.
- `start` cree/demarre la VM puis lance les services sans doubler les processus deja ouverts; `stop` coupe les services et n'eteint une VM partagee qu'avec `--vm`.
- `init` genere une config projet sans secrets ni adapters obligatoires; `--yes` donne le mode non interactif et `--force` est requis pour ecraser.
- `ssh keys` liste les cles publiques detectees cote host avec fingerprint, commentaire et presence dans l'agent SSH, sans lire de cle privee.
- `project import` sans arguments lance un wizard de selection de cle SSH existante; le mode non interactif accepte `--repo`, `--identity` et `--path`.
- `project import` teste l'acces Git avec `GIT_SSH_COMMAND`, refuse un dossier cible non vide, clone, puis enregistre le projet et son identite Git dans le registre host.
- La decouverte GitHub/orgs doit rester cote host, probablement via `gh`, pour reutiliser les credentials locaux sans les persister dans la VM.
