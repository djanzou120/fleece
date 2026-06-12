# Migrations — base de données unifiée (Atlas)

Dossier **unique** des migrations de la base PostgreSQL unifiée `fleece`.
Voir `.ia/ARCHITECTURE.md` §2.1 (base unifiée) et §6.4 (migrations).

## Conventions
- Préfixe numérique **global croissant** + service : `0003_messaging.sql` → ordre déterministe.
- **Un fichier = un schéma** (`CREATE SCHEMA IF NOT EXISTS <service>; ...`). Ne jamais mélanger deux schémas.
- L'ORM (**Drizzle**) ne possède pas les migrations : **Atlas** est l'unique source de vérité.

## Commandes (Atlas)
```sh
# Recalculer le checksum après ajout d'un fichier
atlas migrate hash --dir file://migrations

# Linter les migrations (détection des changements destructeurs/verrouillants)
atlas migrate lint --dir file://migrations --dev-url "docker://postgres/16/dev" --latest 1

# Appliquer (exécuté en job d'init Kubernetes avant le démarrage des services)
atlas migrate apply --dir file://migrations --url "$DATABASE_URL"
```
