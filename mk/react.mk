# Build d'un package Next.js (App Router).
# Paramétré par ${pkg} (ex: platform-app). Miroir de mk/node.mk.
#
# CONTRAINTE OFFLINE : les cibles deps/build/test/fmt appellent npm install.
# Sans accès réseau, `make build pkg=platform-app` échouera sur la phase deps.
# Dette outillage documentée dans MEMORY.md §6 — à exécuter une fois online :
#   1. npm install                          (racine, depuis le répertoire du dépôt)
#   2. make build pkg=platform-app          (type-check + next build)
#
# CHOIX next build : Next 14/15 attend que le répertoire courant soit la racine
# du projet Next (là où se trouvent next.config.js, package.json, etc.).
# Or le shell de l'environnement est fish, qui ne supporte pas heredocs/word-split
# de la même façon que bash. On utilise donc `bash -c 'cd ... && ...'` pour garantir
# la portabilité du changement de répertoire.
#
# PRÉREQUIS FRONTEND : le frontend-engineer doit s'assurer que src/${pkg}/next.config.js
# (ou .ts) contient `output: 'standalone'` pour que le stage Docker `dist` fonctionne.

has_image = true
dockerfile ?= docker/react.dockerfile
target ?= dist

deps:: ## install the dependencies
	npm install

build:: deps ## build the package (type-check + next build)
	$(info building ${pkg})
	npm exec -- tsc -p ./src/${pkg}/tsconfig.json --noEmit
	bash -c 'cd ./src/${pkg} && npm exec -- next build'

test:: deps ## test the package
	$(info testing ${pkg})
	npm exec -- jest --passWithNoTests --testPathPattern './src/${pkg}/.*'

fmt-check:: deps ## check code formatting
	${info checking formatting ${pkg}}
	npm exec -- prettier src/${pkg} --check

fmt:: deps ## format code
	${info formatting ${pkg}}
	npm exec -- prettier src/${pkg} --write
