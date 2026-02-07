# Copyright 2022 Jeremy Edwards
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

GO = go
DOCKER = DOCKER_CLI_EXPERIMENTAL=enabled docker

SHORT_SHA = $(shell git rev-parse --short=7 HEAD | tr -d [:punct:])
DIRTY_VERSION = v0.0.0-$(SHORT_SHA)
VERSION = $(shell git describe --tags || (echo $(DIRTY_VERSION) && exit 1))
BUILD_DATE = $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
TAG := $(VERSION)

export PATH := $(PWD)/build/toolchain/bin:$(PATH):/root/go/bin:/usr/local/go/bin:/usr/go/bin
SOURCE_DIRS=$(shell go list ./... | grep -v '/vendor/')

REGISTRY = ghcr.io/jeremyje
GOMAIN_EXAMPLE_IMAGE = $(REGISTRY)/gomain-example

# https://go.dev/doc/install/source#environment
LINUX_PLATFORMS = linux_386 linux_amd64 linux_arm_v5 linux_arm_v6 linux_arm_v7 linux_arm64 linux_loong64 linux_s390x linux_ppc64 linux_ppc64le linux_riscv64 linux_mips64le linux_mips linux_mipsle linux_mips64
ANDROID_PLATFORMS = android_arm64 # android_386 android_amd64 android_arm android_arm_v5 android_arm_v6 android_arm_v7
WINDOWS_PLATFORMS = windows_386 windows_amd64 windows_arm64 windows_arm_v5 windows_arm_v6 windows_arm_v7
MAIN_PLATFORMS = windows_amd64 linux_amd64 linux_arm64
IOS_PLATFORMS = #ios_amd64 ios_arm64
DARWIN_PLATFORMS = darwin_amd64 darwin_arm64
DRAGONFLY_PLATFORMS = dragonfly_amd64
FREEBSD_PLATFORMS = freebsd_386 freebsd_amd64 freebsd_arm_v5 freebsd_arm_v6 freebsd_arm_v7 freebsd_arm64
NETBSD_PLATFORMS = netbsd_386 netbsd_amd64 netbsd_arm_v5 netbsd_arm_v6 netbsd_arm_v7 netbsd_arm64
OPENBSD_PLATFORMS = openbsd_386 openbsd_amd64 openbsd_arm_v5 openbsd_arm_v6 openbsd_arm_v7 openbsd_arm64 # openbsd_mips64
PLAN9_PLATFORMS = plan9_386 plan9_amd64 plan9_arm_v5 plan9_arm_v6 plan9_arm_v7
NICHE_PLATFORMS = wasip1_wasm js_wasm solaris_amd64 illumos_amd64 aix_ppc64 $(ANDROID_PLATFORMS) $(DARWIN_PLATFORMS) $(IOS_PLATFORMS) $(DRAGONFLY_PLATFORMS) $(FREEBSD_PLATFORMS) $(NETBSD_PLATFORMS) $(OPENBSD_PLATFORMS) $(PLAN9_PLATFORMS)
ALL_PLATFORMS = $(LINUX_PLATFORMS) $(WINDOWS_PLATFORMS) $(NICHE_PLATFORMS)
ALL_APPS = example

MAIN_BINARIES = $(foreach app,$(ALL_APPS),$(foreach platform,$(MAIN_PLATFORMS),build/bin/$(platform)/$(app)$(if $(findstring windows_,$(platform)),.exe,)))
ALL_BINARIES = $(foreach app,$(ALL_APPS),$(foreach platform,$(ALL_PLATFORMS),build/bin/$(platform)/$(app)$(if $(findstring windows_,$(platform)),.exe,)))

WINDOWS_VERSIONS = 1709 1803 1809 1903 1909 2004 20H2 ltsc2022 ltsc2025
BUILDX_BUILDER = buildx-builder
ifeq ($(CI),true)
	DOCKER_BUILDER_FLAG =
else
	DOCKER_BUILDER_FLAG = --builder $(BUILDX_BUILDER)
endif

binaries: $(MAIN_BINARIES)
all: $(ALL_BINARIES)

build/bin/%:
	GOOS=$(firstword $(subst _, ,$(notdir $(abspath $(dir $@))))) GOARCH=$(word 2, $(subst _, ,$(notdir $(abspath $(dir $@))))) GOARM=$(subst v,,$(word 3, $(subst _, ,$(notdir $(abspath $(dir $@)))))) CGO_ENABLED=0 $(GO) build -o $@ cmd/$(basename $(notdir $@))/$(basename $(notdir $@)).go
	touch $@

lint:
	$(GO) fmt ./...
	$(GO) vet ./...

test:
	$(GO) test -race ${SOURCE_DIRS} -cover -count 100

coverage.txt:
	for sfile in ${SOURCE_DIRS} ; do \
		go test -race "$$sfile" -coverprofile=package.coverage -covermode=atomic; \
		if [ -f package.coverage ]; then \
			cat package.coverage >> coverage.txt; \
			$(RM) package.coverage; \
		fi; \
	done

ensure-builder:
ifeq ($(CI),true)
	echo "Skipping creation of buildx context, running in CI."
else
	-$(DOCKER) buildx create --name $(BUILDX_BUILDER)
endif

ALL_IMAGES = $(GOMAIN_EXAMPLE_IMAGE)
# https://github.com/docker-library/official-images#architectures-other-than-amd64
images: DOCKER_PUSH = --push
images: linux-images windows-images
	-$(DOCKER) manifest rm $(GOMAIN_EXAMPLE_IMAGE):$(TAG)

	for image in $(ALL_IMAGES) ; do \
		$(DOCKER) manifest create $$image:$(TAG) $(foreach winver,$(WINDOWS_VERSIONS),$${image}:$(TAG)-windows_amd64-$(winver)) $(foreach platform,$(LINUX_PLATFORMS),$${image}:$(TAG)-$(platform)) ; \
		for winver in $(WINDOWS_VERSIONS) ; do \
			windows_version=`$(DOCKER) manifest inspect mcr.microsoft.com/windows/nanoserver:$${winver} | jq -r '.manifests[0].platform["os.version"]'`; \
			$(DOCKER) manifest annotate --os-version $${windows_version} $${image}:$(TAG) $${image}:$(TAG)-windows_amd64-$${winver} ; \
		done ; \
		$(DOCKER) manifest push $$image:$(TAG) ; \
	done

ALL_LINUX_IMAGES = $(foreach app,$(ALL_APPS),$(foreach platform,$(LINUX_PLATFORMS),linux-image-$(app)-$(platform)))
linux-images: $(ALL_LINUX_IMAGES)

linux-image-example-%: build/bin/%/example ensure-builder
	$(DOCKER) buildx build $(DOCKER_BUILDER_FLAG) --platform $(subst _,/,$*) --build-arg BINARY_PATH=$< -f cmd/example/Dockerfile -t $(GOMAIN_EXAMPLE_IMAGE):$(TAG)-$* . $(DOCKER_PUSH)

ALL_WINDOWS_IMAGES = $(foreach app,$(ALL_APPS),$(foreach winver,$(WINDOWS_VERSIONS),windows-image-$(app)-$(winver)))
windows-images: $(ALL_WINDOWS_IMAGES)

windows-image-example-%: build/bin/windows_amd64/example.exe ensure-builder
	$(DOCKER) buildx build $(DOCKER_BUILDER_FLAG) --platform windows/amd64 -f cmd/example/Dockerfile.windows --build-arg WINDOWS_VERSION=$* -t $(GOMAIN_EXAMPLE_IMAGE):$(TAG)-windows_amd64-$* . $(DOCKER_PUSH)

clean:
	-chmod -R +w build/
	rm -rf build/

upgrade-deps:
	$(GO) get -u ./...
	$(GO) mod tidy

presubmit: clean lint test all images

.PHONY: binaries all lint test ensure-builder images linux-images windows-images clean upgrade-deps presubmit
