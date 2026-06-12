# Build arguments for the docker image.
has_image = true

# override app.Name, if needed.
name ?= $(pkg)

ldflags = -X 'fleece/src/go/app.Version=${version}' -X 'fleece/src/go/app.Name=${name}'
ifdef static
ldflags += -extldflags=-static
endif

build:: ## build the package
	$(info building ${pkg})
	go build -o ./bin/${pkg} -ldflags="${ldflags}" ./src/${pkg}

test:: ## test the package
	$(info testing ${pkg})
	go test ./src/${pkg} ${test_args}

fmt:: deps ## format code
	${info formatting ${pkg}}
	go fmt src/${pkg}
