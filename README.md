# devctl

CLI generique pour creer et piloter des environnements de developpement isoles.

Objectif V1:
- Multipass pour une VM par projet.
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
```

Si `.devctl.yml` manque, `devctl` affiche un warning et propose `devctl init`.

Commandes MVP:
- `doctor`: verifie les prerequis hote et affiche les commandes d'installation par OS.
- `up`: cree la VM Multipass si elle n'existe pas, apres confirmation ressources.
- `hosts sync`: ajoute ou met a jour `<vm-ip> <host>` dans `/etc/hosts`.
- `ssh config`: imprime le bloc SSH avec `ForwardAgent yes`.
- `provision`: lance le playbook Ansible embarque dans `devctl`.

## Installation globale

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
infisical:
  project_path: /lmdlp
  default_env: dev
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

## Etat

Scaffold initial. Les commandes d'orchestration Multipass/Ansible seront ajoutees apres validation du plan.
