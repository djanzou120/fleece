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

# Produit le DDL correspondant aux définitions Drizzle, dans le répertoire de
# travail jetable déclaré par drizzle.config.ts (jamais migrations/).
# Sert à la vérification de parité (M-028), pas à alimenter une base.
#
# Ces cibles ont temporairement été découplées de `deps` tant que D-M47 rendait
# `npm install` inopérant a la racine ; D-M47 etant resolue (graphql ramene en
# ^16, la seule ligne que graphql-yoga supporte), la dependance est retablie
# comme dans mk/esbuild.mk et mk/node.mk.
generate:: deps ## generate the DDL from the Drizzle definitions (parity check input)
	$(info generating DDL from ${pkg} Drizzle definitions)
	npm exec -- drizzle-kit generate --config=src/${pkg}/drizzle.config.ts

build:: deps ## type-check the schema definitions
	$(info type-checking ${pkg})
	npm exec -- tsc -p ./src/${pkg}/tsconfig.json --noEmit

test:: deps ## test the package
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
