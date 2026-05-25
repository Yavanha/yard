# Yard

Yard est une CLI pour provisionner et piloter des environnements de developpement isoles depuis la machine hote.

Les textes visibles par les utilisateurs dans la CLI restent en anglais: aide, erreurs, prompts, confirmations et tables.

## Etat

Yard est la surface produit et CLI canonique.

Capacites actuelles:
- registre projets cote hote;
- runtime local VM via Lima;
- runtime remote server via SSH;
- scaffold de config projet;
- import projet avec identite SSH cote hote;
- setup, status et exec sur la cible runtime;
- start, stop et logs des services de dev;
- inspection des fingerprints de host key remote.

Le support remote server est implemente mais doit encore passer un smoke end-to-end sur un vrai serveur SSH avant d'etre considere completement valide.

## Depuis Les Sources

Pendant le developpement, Yard se lance depuis ce repo:

```bash
go run ./cmd/yard --help
```

Dans la documentation utilisateur, les commandes sont ecrites `yard ...`. Depuis les sources, remplacer `yard` par `go run ./cmd/yard`.

## Quick Start

Creer une config projet:

```bash
yard init web-app --yes --config .yard.yml
```

Enregistrer un projet local VM:

```bash
yard project add web-app /path/to/web-app --runtime local-vm
yard use web-app
```

Configurer et demarrer le projet:

```bash
yard setup web-app
yard start web-app
yard status web-app
yard process logs web-app web --tail 80
```

Arreter les services du projet:

```bash
yard stop web-app
```

## Remote Server Quick Start

Recuperer le fingerprint de host key remote:

```bash
yard ssh host-key dev.example.com --port 22
```

Enregistrer un projet remote:

```bash
yard project add api /path/to/api \
  --runtime remote-server \
  --remote-host dev.example.com \
  --remote-user ubuntu \
  --remote-workdir /home/ubuntu/workspaces/api \
  --remote-host-key SHA256:...
```

Verifier et utiliser la cible remote:

```bash
yard setup api
yard exec api -- pwd
yard process list api
yard start api
yard stop api
```

## Project Config

Le fichier de config projet canonique est `.yard.yml`.

Exemple:

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

Un exemple reutilisable vit dans `examples/web-app.yard.yml`.

## Host Registry

Yard stocke les choix locaux dans `~/.config/yard/config.yaml`.

Le registre peut contenir:
- projet courant;
- chemin local du projet;
- chemin de config;
- runtime target: `local-vm` ou `remote-server`;
- mode et nom de VM locale;
- metadonnees SSH remote;
- metadonnees d'identite Git cote hote.

Les secrets reels ne doivent jamais etre stockes dans le repo, `.yard.yml`, le registre hote, une Dev VM ou un Remote Server par Yard. Les secrets runtime doivent venir d'un fournisseur externe.

## Developpement

Avant commit:

```bash
./scripts/check.sh
```

Le check valide l'aide Yard, l'exemple de config et tous les tests Go.

## Direction Documentation

La documentation utilisateur vit dans la CLI Command Gallery: [`docs/cli/`](docs/cli/), organisee par scenarios d'abord et reference commandes ensuite.

Statuts de features:
- `Available`: implemente et couvert par les checks automatises.
- `Experimental`: implemente mais pas encore valide en conditions reelles.
- `Planned`: pas encore implemente.
