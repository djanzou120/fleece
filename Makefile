.SILENT:

# Target selection. pkg is required on the command line, while version is
# retrieved from either the tags matching the package name or the current
# commit version.
tag = $(shell git describe --tags --match="${pkg}-*" --dirty="*" 2>/dev/null)
version ?= $(subst ${pkg}-,,${tag})
ifeq ($(strip $(version)),)
version = $(shell git rev-parse --short HEAD)
endif

# Include the package specific rules and configuration. The ${pkg}/pkg should
# contain the type definition.
ifdef pkg
include ./src/${pkg}/pkg

ifndef type
$(error missing type definition in package ${pkg})
endif
include ./mk/${type}.mk
endif

ifdef has_image
dockerfile ?= docker/${type}.dockerfile
target ?= dist
image_build_args += --build-arg PKG='${pkg}'
image_build_args += --build-arg VERSION='${version}'
image:: ## build the docker image for the package
	$(info building ${pkg} image)
	docker buildx build . -f ${dockerfile} --platform linux/amd64 --target ${target} -t anthill/backend:${pkg}-v${version} ${image_build_args}

publish:: ## publish the package image to docker
	$(info building ${pkg}:${version} image)
	make image pkg=${pkg} version=${version}
	$(info publishing ${pkg}:${version} image)
	docker push anthill/backend:${pkg}-v${version}
endif