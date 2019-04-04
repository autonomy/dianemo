SHA = $(shell gitmeta git sha)
TAG = $(shell gitmeta image tag)
BUILT = $(shell gitmeta built)
PUSH = $(shell gitmeta pushable)

VPATH = $(PATH)
KERNEL_IMAGE ?= autonomy/kernel:acdffda
TOOLCHAIN_IMAGE ?= autonomy/toolchain:989387e
DOCKER_ARGS ?=
BUILDKIT_VERSION ?= v0.3.3
BUILDKIT_IMAGE ?= moby/buildkit:$(BUILDKIT_VERSION)
BUILDKIT_HOST ?= tcp://0.0.0.0:1234
BUILDKIT_CONTAINER_NAME ?= talos-buildkit
BUILDKIT_CONTAINER_STOPPED := $(shell docker ps --filter name=$(BUILDKIT_CONTAINER_NAME) --filter status=exited --format='{{.Names}}' 2>/dev/null)
BUILDKIT_CONTAINER_RUNNING := $(shell docker ps --filter name=$(BUILDKIT_CONTAINER_NAME) --filter status=running --format='{{.Names}}' 2>/dev/null)
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
BUILDCTL_ARCHIVE := https://github.com/moby/buildkit/releases/download/$(BUILDKIT_VERSION)/buildkit-$(BUILDKIT_VERSION).linux-amd64.tar.gz
endif
ifeq ($(UNAME_S),Darwin)
BUILDCTL_ARCHIVE := https://github.com/moby/buildkit/releases/download/$(BUILDKIT_VERSION)/buildkit-$(BUILDKIT_VERSION).darwin-amd64.tar.gz
endif
BINDIR ?= /usr/local/bin
CONFORM_VERSION ?= c539351

COMMON_ARGS = --progress=plain
COMMON_ARGS += --frontend=dockerfile.v0
COMMON_ARGS += --local context=.
COMMON_ARGS += --local dockerfile=.
COMMON_ARGS += --frontend-opt build-arg:KERNEL_IMAGE=$(KERNEL_IMAGE)
COMMON_ARGS += --frontend-opt build-arg:TOOLCHAIN_IMAGE=$(TOOLCHAIN_IMAGE)
COMMON_ARGS += --frontend-opt build-arg:SHA=$(SHA)
COMMON_ARGS += --frontend-opt build-arg:TAG=$(TAG)

all: ci kernel initramfs rootfs osctl-linux-amd64 osctl-darwin-amd64 osinstall-linux-amd64 test lint installer

.PHONY: builddeps
builddeps: gitmeta buildctl

gitmeta:
	GO111MODULE=off go get github.com/talos-systems/gitmeta

buildctl:
	@wget -qO - $(BUILDCTL_ARCHIVE) | \
		sudo tar -zxf - -C $(BINDIR) --strip-components 1 bin/buildctl

.PHONY: buildkitd
buildkitd:
ifeq (tcp://0.0.0.0:1234,$(findstring tcp://0.0.0.0:1234,$(BUILDKIT_HOST)))
ifeq ($(BUILDKIT_CONTAINER_STOPPED),$(BUILDKIT_CONTAINER_NAME))
	@echo "Removing exited talos-buildkit container"
	@docker rm $(BUILDKIT_CONTAINER_NAME)
endif
ifneq ($(BUILDKIT_CONTAINER_RUNNING),$(BUILDKIT_CONTAINER_NAME))
	@echo "Starting talos-buildkit container"
	@docker run \
		--name $(BUILDKIT_CONTAINER_NAME) \
		-d \
		--privileged \
		 -p 1234:1234 \
		$(BUILDKIT_IMAGE) \
		--addr $(BUILDKIT_HOST)
	@echo "Wait for buildkitd to become available"
	@sleep 5
endif
endif

enforce:
	@docker run --rm -v $(PWD):/src -w /src autonomy/conform:$(CONFORM_VERSION)

.PHONY: ci
ci: builddeps buildkitd

base: buildkitd
	@buildctl --addr $(BUILDKIT_HOST) \
		build \
		--exporter=docker \
		--exporter-opt output=build/$@.tar \
		--exporter-opt name=docker.io/autonomy/$@:$(TAG) \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)
	@docker load < build/$@.tar

kernel: buildkitd
	@buildctl --addr $(BUILDKIT_HOST) \
		build \
		--exporter=local \
		--exporter-opt output=build \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

initramfs: buildkitd
	@buildctl --addr $(BUILDKIT_HOST) \
		build \
		--exporter=local \
		--exporter-opt output=build \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

rootfs: buildkitd hyperkube etcd coredns pause osd trustd proxyd blockd ntpd
	@buildctl --addr $(BUILDKIT_HOST) \
		build \
		--exporter=local \
		--exporter-opt output=build \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

container-os: buildkitd
	@buildctl --addr $(BUILDKIT_HOST) \
		build \
		--exporter=docker \
		--exporter-opt output=build/$@.tar \
		--exporter-opt name=docker.io/autonomy/talos-os:$(TAG) \
		--exporter-opt push=$(PUSH) \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)
	@docker load < build/$@.tar

installer: buildkitd
	@mkdir -p build
	@buildctl --addr $(BUILDKIT_HOST) \
		build \
		--exporter=docker \
		--exporter-opt output=build/$@.tar \
		--exporter-opt name=docker.io/autonomy/talos:$(TAG) \
		--exporter-opt push=$(PUSH) \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)
	@docker load < build/$@.tar

proto: buildkitd
	buildctl --addr $(BUILDKIT_HOST) \
		build \
		--exporter=local \
		--exporter-opt output=./ \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

image-gcloud: installer
	@docker run --rm -v /dev:/dev -v $(PWD)/build/gcloud:/out --privileged $(DOCKER_ARGS) talos-systems/talos:$(TAG) image -l \
	-f -p googlecloud -u none -e 'random.trust_cpu=on'
	@mv $(PWD)/build/gcloud/image.raw $(PWD)/build/gcloud/disk.raw
	@tar -C $(PWD)/build/gcloud -Sczf $(PWD)/build/gcloud/talos.tar.gz disk.raw

image-vanilla: installer
	@docker run --rm -v /dev:/dev -v $(PWD)/build:/out --privileged $(DOCKER_ARGS) talos-systems/talos:$(TAG) image -l

test: buildkitd
	@buildctl --addr $(BUILDKIT_HOST) \
		build \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

lint: buildkitd
	@buildctl --addr $(BUILDKIT_HOST) \
		build \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

osctl-linux-amd64: buildkitd
	@buildctl --addr $(BUILDKIT_HOST) \
		build \
		--exporter=local \
		--exporter-opt output=build \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

osctl-darwin-amd64: buildkitd
	@buildctl --addr $(BUILDKIT_HOST) \
		build \
		--exporter=local \
		--exporter-opt output=build \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

osinstall-linux-amd64: buildkitd
	@buildctl --addr $(BUILDKIT_HOST) \
		build \
		--exporter=local \
		--exporter-opt output=build \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

udevd: buildkitd
	@buildctl --addr $(BUILDKIT_HOST) \
		build \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

images:
	mkdir -p images

osd: buildkitd images
	@buildctl --addr $(BUILDKIT_HOST) \
		build \
		--exporter=docker \
		--exporter-opt output=images/$@.tar \
		--exporter-opt name=docker.io/autonomy/$@:$(TAG) \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

trustd: buildkitd images
	@buildctl --addr $(BUILDKIT_HOST) \
		build \
		--exporter=docker \
		--exporter-opt output=images/$@.tar \
		--exporter-opt name=docker.io/autonomy/$@:$(TAG) \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

proxyd: buildkitd images
	@buildctl --addr $(BUILDKIT_HOST) \
		build \
		--exporter=docker \
		--exporter-opt output=images/$@.tar \
		--exporter-opt name=docker.io/autonomy/$@:$(TAG) \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

blockd: buildkitd images
	@buildctl --addr $(BUILDKIT_HOST) \
		build \
		--exporter=docker \
		--exporter-opt output=images/$@.tar \
		--exporter-opt name=docker.io/autonomy/$@:$(TAG) \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

ntpd: buildkitd images
	@buildctl --addr $(BUILDKIT_HOST) \
		build \
		--exporter=docker \
		--exporter-opt output=images/$@.tar \
		--exporter-opt name=docker.io/autonomy/$@:$(TAG) \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

hyperkube: images
	@docker pull k8s.gcr.io/$@:v1.14.0
	@docker save k8s.gcr.io/$@:v1.14.0 -o ./images/$@.tar

etcd: images
	@docker pull k8s.gcr.io/$@:3.3.10
	@docker save k8s.gcr.io/$@:3.3.10 -o ./images/$@.tar

coredns: images
	@docker pull k8s.gcr.io/$@:1.3.1
	@docker save k8s.gcr.io/$@:1.3.1 -o ./images/$@.tar

pause: images
	@docker pull k8s.gcr.io/$@:3.1
	@docker save k8s.gcr.io/$@:3.1 -o ./images/$@.tar

clean:
	-go clean -modcache
	-rm -rf build vendor
