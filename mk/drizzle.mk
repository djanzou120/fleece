# Paquet de définitions de schéma Drizzle (@fleece/model, M-027/M-028).
#
# Pas de `has_image` : ce paquet n'est pas déployable, c'est une bibliothèque
# de définitions consommée par les services TS.
#
# RÈGLE CARDINALE — Atlas est la SEULE source de vérité des migrations
# (CLAUDE.md). Drizzle est un query builder + un typage. La cible `migrate`
# ci-dessous REFUSE donc délibérément de s'exécuter : elle transforme une règle
# de documentation en garde-fou exécutable. Voir son commentaire.

deps:: ## install the dependencies
	npm install

# `generate` et `build` NE DEPENDENT VOLONTAIREMENT PAS DE `deps`, contrairement
# a mk/esbuild.mk et mk/node.mk.
#
# Raison (D-M47, constatee pendant M-028) : `npm install` ET `npm ci` echouent
# aujourd'hui a la racine du depot — le lockfile porte graphql@17.0.1 alors que
# @graphql-codegen/cli@5.0.2 plafonne son peer a graphql@16, et npm >= 7 fait
# respecter les peers. Enchainer `deps` rendrait ces deux cibles inutilisables
# pour une raison SANS AUCUN RAPPORT avec le schema. Ce sont de toute facon des
# etapes hors-ligne (codegen et verification de types) : elles n'ont besoin que
# d'un node_modules deja present, pas d'une resolution complete.
# Retablir la dependance a `deps` le jour ou D-M47 sera resolue.

# Produit le DDL correspondant aux définitions Drizzle, dans le répertoire de
# travail jetable déclaré par drizzle.config.ts (jamais migrations/).
# Sert à la vérification de parité (M-028), pas à alimenter une base.
generate:: ## generate the DDL from the Drizzle definitions (parity check input)
	$(info generating DDL from ${pkg} Drizzle definitions)
	npm exec -- drizzle-kit generate --config=src/${pkg}/drizzle.config.ts

build:: ## type-check the schema definitions
	$(info type-checking ${pkg})
	npm exec -- tsc -p ./src/${pkg}/tsconfig.json --noEmit

test:: ## test the package
	$(info testing ${pkg})
	npm exec -- jest --passWithNoTests --testPathPattern './src/${pkg}/.*'

# GARDE-FOU VOLONTAIRE, PAS UNE CIBLE INACHEVÉE.
#
# M-028 demandait « intégrer dans le Makefile (make migrate pkg=model) ». Mais
# une cible `migrate` qui migrerait réellement via Drizzle violerait la règle
# du dépôt et casserait `atlas.sum` : les deux générateurs se disputeraient le
# même schéma. Le nom est donc conservé — c'est celui que quelqu'un tapera par
# réflexe — mais il échoue en expliquant où aller. Échouer bruyamment vaut
# mieux que ne pas exister : une cible absente donne « No rule to make target »,
# qui n'apprend rien à qui l'a tapée.
migrate:: ## refuses: Atlas owns migrations (see message)
	$(info )
	$(info Les migrations Fleece appartiennent a Atlas, pas a Drizzle.)
	$(info   Appliquer  : atlas migrate apply --dir file://migrations --url "$$DATABASE_URL")
	$(info   Verifier   : make generate pkg=${pkg}   puis comparer au schema Atlas)
	$(info Drizzle ne sert ici qu'au typage et au query building (voir src/${pkg}/README.md).)
	$(info )
	$(error cible `migrate` volontairement desactivee pour ${pkg})

fmt-check:: deps ## check code formatting
	${info checking formatting ${pkg}}
	npm exec -- prettier src/${pkg} --check

fmt:: deps ## format code
	${info formatting ${pkg}}
	npm exec -- prettier src/${pkg} --write
