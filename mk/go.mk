# Build arguments for the docker image.
has_image = true

# override app.Name, if needed.
name ?= $(pkg)

ldflags = -X 'fleece/src/go/app.Version=${version}' -X 'fleece/src/go/app.Name=${name}'
ifdef static
ldflags += -extldflags=-static
endif

# D-M09 — main package location.
#
# Go requires "one directory = one package". Some packages (src/api,
# src/core-processor, src/intelligence-processor) are structured as an
# importable library at src/<pkg> ("package <pkg>") PLUS a composition root
# at src/<pkg>/cmd/<pkg> ("package main"). `go build ./src/<pkg>` on such a
# package silently produces a library archive with no entrypoint and no
# error — `go build -o x ./src/api` exits 0 and writes an `ar archive`, not
# an executable. Legacy services (messaging, routing, provider, wallet,
# webhook, contact-intelligence, campaign, analytics — still present until
# M-023) keep main.go at the package root and must keep building from
# ./src/<pkg> directly.
#
# Simplest reliable fix: detect the cmd/<pkg> layout via $(wildcard) and pick
# the build path accordingly — no per-package variable to maintain, and it
# stays correct automatically once M-023 removes the legacy services.
gobuildpath = $(if $(wildcard ./src/${pkg}/cmd/${pkg}),./src/${pkg}/cmd/${pkg},./src/${pkg})

build:: ## build the package
	$(info building ${pkg} (source: ${gobuildpath}))
	go build -o ./bin/${pkg} -ldflags="${ldflags}" ${gobuildpath}

test:: ## test the package
	$(info testing ${pkg})
	go test ./src/${pkg} ${test_args}

fmt:: deps ## format code
	${info formatting ${pkg}}
	go fmt src/${pkg}
