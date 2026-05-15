# Ansible

Provision durable de la VM de developpement.

V1 cible:
- Docker Engine officiel dans la VM.
- Devcontainer CLI dans `/opt/devtools`.
- Supabase CLI versionnee.
- User `ubuntu` dans le groupe `docker`.
- `/proc` durci avec `hidepid=2` si compatible.
- Dossier `/home/ubuntu/workspaces`.

Les secrets runtime ne passent pas par Ansible.

