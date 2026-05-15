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

Depuis un repo projet contenant `.devctl.yml`:

```bash
devctl doctor
devctl config
devctl init
```

Si `.devctl.yml` manque, `devctl` affiche un warning et propose `devctl init`.

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

