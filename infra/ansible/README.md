# Ansible

Provision durable de la VM de developpement.

La VM est creee par `devctl up` avec Lima. Ansible se connecte ensuite via le fichier SSH genere par Lima (`ssh -F ~/.lima/<vm>/ssh.config lima-<vm>`).

V1 cible:
- Docker Engine officiel dans la VM.
- Devcontainer CLI dans `/opt/devtools`.
- Supabase CLI versionnee depuis GitHub Releases avec verification checksum.
- User `ubuntu` dans le groupe `docker`.
- `/proc` durci avec `hidepid=2` si compatible.
- Dossier `/home/ubuntu/workspaces`.

Les secrets runtime ne passent pas par Ansible.
