# Build arguments for the docker image.
has_image = true
dockerfile ?= docker/node.dockerfile

deps:: ## install the dependencies
	npm install

build:: deps ## build the package
	$(info building ${pkg})
	rm -rf ./bin/${pkg}
	npm exec -- tsc -p ./src/${pkg}/tsconfig.json --noEmit
	npm exec -- esbuild ./src/${pkg}/index.ts --bundle --outfile=./bin/${pkg}/index.js --platform=node --target=node20 --loader:.graphql=text --sourcemap

test:: deps ## test the package
	$(info testing ${pkg})
	npm exec -- jest --passWithNoTests --testPathPattern './src/${pkg}/.*'

fmt-check:: deps ## check code formatting
	${info checking formatting ${pkg}}
	npm exec -- prettier src/${pkg} --check

fmt:: deps ## format code
	${info formatting ${pkg}}
	npm exec -- prettier src/${pkg} --write
