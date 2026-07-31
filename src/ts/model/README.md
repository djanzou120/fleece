# @fleece/model

Définitions **Drizzle** du schéma PostgreSQL `identity` — query builder et typage
pour les services TypeScript.

## La règle à ne pas enfreindre

> **Atlas est la seule source de vérité des migrations.**
> Ce paquet **reflète** `migrations/`, il ne le pilote pas.

Ne lancez jamais `drizzle-kit migrate` ni `drizzle-kit push` contre une base
Fleece. Deux générateurs sur le même schéma, c'est un `atlas.sum` cassé et une
base qui diverge silencieusement de ses migrations.

Le garde-fou est exécutable, pas seulement écrit ici :

- `drizzle.config.ts` ne déclare **aucun** `dbCredentials` — sans URL, les
  commandes qui écrivent dans une base ne peuvent pas démarrer.
- `out` pointe sur `.drizzle-out/` (ignoré par git), **jamais** sur `migrations/`.
- `make migrate pkg=ts/model` **refuse de s'exécuter** et rappelle où aller.

## Périmètre : `identity` seulement

`auth-api` est le seul service TypeScript propriétaire d'un schéma. Les schémas
`wallet`, `messaging`, `webhook`… appartiennent aux services Go ; `graphql-api`
les lit via les clients REST vers `src/api`, jamais en SQL (vérifié : aucun accès
base dans `src/graphql-api`). Les modéliser ici inviterait l'accès cross-schéma
que l'architecture interdit.

## Commandes

```sh
make build    pkg=ts/model   # vérification de types (tsc --noEmit)
make generate pkg=ts/model   # produit le DDL dans .drizzle-out/ (parité)
make migrate  pkg=ts/model   # refuse volontairement — voir ci-dessus
```

## Prouver la parité avec Atlas

C'est la vérification faite en M-028, à rejouer après toute modification du
schéma. Le principe : appliquer les deux sources sur deux bases neuves, puis
comparer leur **introspection** — et non le texte du SQL, qui diffère
légitimement (ordre des instructions, mise en forme).

```sh
# 1. Une base par source
docker run -d --name pg-parity -e POSTGRES_PASSWORD=qa -e POSTGRES_DB=atlasdb -p 55434:5432 postgres:16
docker exec pg-parity psql -U postgres -c "CREATE DATABASE drizzledb;"

# 2. Atlas d'un côté, DDL Drizzle de l'autre
atlas migrate apply --dir file://migrations --url "postgres://postgres:qa@localhost:55434/atlasdb?sslmode=disable"
make generate pkg=ts/model
sed 's/--> statement-breakpoint//' src/ts/model/.drizzle-out/*.sql | \
  docker exec -i pg-parity psql -U postgres -d drizzledb -v ON_ERROR_STOP=1

# 3. Comparer colonnes, contraintes, index et séquences du schéma identity
```

Résultat obtenu en M-028 : **28 colonnes, 8 contraintes, 7 index et 1 séquence
identiques**, aucune différence.

Deux détails qui rendent cette égalité possible, à préserver :

- Les contraintes `UNIQUE` sont **nommées explicitement** (`users_email_key`,
  `api_keys_hashed_key_key`) avec le nom que PostgreSQL génère pour une
  contrainte anonyme. Sans ça, Drizzle produirait `users_email_unique` : même
  contrainte, nom différent, comparaison bruyante.
- `uq_workspaces_slug` est un **index unique** (`uniqueIndex()`), pas une
  contrainte `UNIQUE` — parce que `0011_identity_schema.sql` le crée ainsi.

Les **noms des clés étrangères**, eux, diffèrent (`users_workspace_id_workspaces_id_fk`
côté Drizzle contre `users_workspace_id_fkey` côté PostgreSQL). C'est sans
conséquence : le nom d'une FK auto-générée n'est pas une propriété du schéma, et
la comparaison porte sur la relation (table, colonnes → cible).

## Résolution du paquet

`@fleece/model` est lié dans `node_modules/@fleece/model` par les workspaces npm.
Les `tsconfig.json` consommateurs déclarent en plus un mapping `paths`, comme
pour `@fleece/api-common` — ceinture et bretelles, utile tant qu'un
`node_modules` n'a pas été régénéré.

> Historique : à sa création, ce paquet n'était pas lié, `npm install` échouant
> à la racine pour une raison sans rapport (**D-M47** — `graphql` était fixé en
> `^17` alors que `graphql-yoga`, le seul consommateur réel, ne supporte que
> `^15 || ^16`). D-M47 est résolue ; le lien existe désormais.
