# Copyright 2018 The Kubernetes Authors.
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
#
IMAGE=juicedata/juicefs-csi-driver
JUICEFS_IMAGE=juicefs/juicefs-fuse
REGISTRY=docker.io
ACR_REGISTRY=registry.cn-hangzhou.aliyuncs.com
VERSION=$(shell git describe --tags --match 'v*' --always --dirty)
GIT_BRANCH?=$(shell git rev-parse --abbrev-ref HEAD)
GIT_COMMIT?=$(shell git rev-parse HEAD)
DEV_TAG=dev-$(shell git describe --always --dirty)
BUILD_DATE?=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
PKG=github.com/juicedata/juicefs-csi-driver
LDFLAGS?="-X ${PKG}/pkg/driver.driverVersion=${VERSION} -X ${PKG}/pkg/driver.gitCommit=${GIT_COMMIT} -X ${PKG}/pkg/driver.buildDate=${BUILD_DATE} -s -w"
GO111MODULE=on
IMAGE_VERSION_ANNOTATED=$(IMAGE):$(VERSION)-juicefs$(shell docker run --entrypoint=/usr/bin/juicefs $(IMAGE):$(VERSION) version | cut -d' ' -f3)
JUICEFS_LATEST_VERSION=$(shell curl -fsSL https://api.github.com/repos/juicedata/juicefs/releases/latest | grep tag_name | grep -oE 'v[0-9]+\.[0-9][0-9]*(\.[0-9]+(-[0-9a-z]+)?)?')
JUICEFS_CE_LATEST_VERSION=$(shell curl -fsSL https://api.github.com/repos/juicedata/juicefs/releases/latest | grep tag_name | grep -oE 'v[0-9]+\.[0-9][0-9]*(\.[0-9]+(-[0-9a-z]+)?)?')
JUICEFS_EE_LATEST_VERSION=$(shell curl -sSL https://juicefs.com/static/juicefs -o juicefs-ee && chmod +x juicefs-ee && ./juicefs-ee version | cut -d' ' -f3)
JUICEFS_CSI_LATEST_VERSION=$(shell git describe --tags --match 'v*' | grep -oE 'v[0-9]+\.[0-9][0-9]*(\.[0-9]+(-[0-9a-z]+)?)?')

GOPROXY=https://goproxy.io
GOPATH=$(shell go env GOPATH)
GOOS=$(shell go env GOOS)
GOBIN=$(shell pwd)/bin

.PHONY: juicefs-csi-driver
juicefs-csi-driver:
	mkdir -p bin
	CGO_ENABLED=0 GOOS=linux go build -ldflags ${LDFLAGS} -o bin/juicefs-csi-driver ./cmd/

.PHONY: verify
verify:
	./hack/verify-all

.PHONY: test
test:
	go test -v -race ./pkg/...

.PHONY: test-sanity
test-sanity:
	go test -v ./tests/sanity/...

.PHONY: image-nightly
image-nightly:
	# Build image with newest juicefs-csi-driver and juicefs
	docker build --build-arg TARGETARCH=amd64 -t $(IMAGE):nightly .

.PHONY: push
push-nightly:
	docker tag $(IMAGE):nightly $(REGISTRY)/$(IMAGE):nightly
	docker push $(REGISTRY)/$(IMAGE):nightly

.PHONY: image-nightly-buildx
image-nightly-buildx:
	# Build image with newest juicefs-csi-driver and juicefs
	docker buildx build -t $(IMAGE):nightly --platform linux/amd64,linux/arm64 . --push

.PHONY: juicefs-image-nightly
juicefs-image-nightly:
	# Build image with newest juicefs-csi-driver and juicefs
	docker build -f juicefs.Dockerfile --build-arg TARGETARCH=amd64 -t $(ACR_REGISTRY)/$(JUICEFS_IMAGE):nightly .
	docker push $(ACR_REGISTRY)/$(JUICEFS_IMAGE):nightly

.PHONY: juicefs-image-nightly-buildx
juicefs-image-nightly-buildx:
	# Build image with newest juicefs
	docker buildx build -f juicefs.Dockerfile -t $(ACR_REGISTRY)/$(JUICEFS_IMAGE):nightly --platform linux/amd64,linux/arm64 . --push
#	docker build -f juicefs.Dockerfile --build-arg TARGETARCH=amd64 -t $(ACR_REGISTRY)/$(JUICEFS_IMAGE):nightly .
#	docker push $(ACR_REGISTRY)/$(JUICEFS_IMAGE):nightly

.PHONY: image-latest
image-latest:
	# Build image with latest stable juicefs-csi-driver and juicefs
	docker build --build-arg JUICEFS_CSI_REPO_REF=$(JUICEFS_CSI_LATEST_VERSION) \
		--build-arg JUICEFS_REPO_REF=$(JUICEFS_LATEST_VERSION) \
		--build-arg JFS_AUTO_UPGRADE=disabled \
		--build-arg TARGETARCH=amd64 \
		-t $(IMAGE):latest -f Dockerfile .

.PHONY: push-latest
push-latest:
	docker tag $(IMAGE):latest $(REGISTRY)/$(IMAGE):latest
	docker push $(REGISTRY)/$(IMAGE):latest

.PHONY: image-branch
image-branch:
	docker build --build-arg TARGETARCH=amd64 -t $(IMAGE):$(GIT_BRANCH) -f Dockerfile .

.PHONY: push-branch
push-branch:
	docker tag $(IMAGE):$(GIT_BRANCH) $(REGISTRY)/$(IMAGE):$(GIT_BRANCH)
	docker push $(REGISTRY)/$(IMAGE):$(GIT_BRANCH)

.PHONY: image-version
image-version:
	[ -z `git status --porcelain` ] || (git --no-pager diff && exit 255)
	docker buildx build -t $(IMAGE):$(VERSION) --build-arg JUICEFS_REPO_REF=$(JUICEFS_LATEST_VERSION) \
		--build-arg=JFS_AUTO_UPGRADE=disabled --platform linux/amd64,linux/arm64 . --push

.PHONY: juicefs-image-version
juicefs-image-version:
	#[ -z `git status --porcelain` ] || (git --no-pager diff && exit 255)
#	docker buildx build -f juicefs.Dockerfile -t $(ACR_REGISTRY)/$(JUICEFS_IMAGE):$(JUICEFS_LATEST_VERSION) --build-arg JUICEFS_REPO_REF=$(JUICEFS_LATEST_VERSION) \
#		--build-arg=JFS_AUTO_UPGRADE=disabled --platform linux/amd64,linux/arm64 . --push
	docker build -f juicefs.Dockerfile --build-arg TARGETARCH=amd64 -t $(ACR_REGISTRY)/$(JUICEFS_IMAGE):$(JUICEFS_CE_LATEST_VERSION)-$(JUICEFS_EE_LATEST_VERSION) \
	 --build-arg JUICEFS_REPO_REF=$(JUICEFS_CE_LATEST_VERSION) --build-arg=JFS_AUTO_UPGRADE=disabled .
	docker push $(ACR_REGISTRY)/$(JUICEFS_IMAGE):$(JUICEFS_CE_LATEST_VERSION)-$(JUICEFS_EE_LATEST_VERSION)

.PHONY: push-version
push-version:
	docker push $(IMAGE):$(VERSION)
	docker tag $(IMAGE):$(VERSION) $(IMAGE_VERSION_ANNOTATED)
	docker push $(IMAGE_VERSION_ANNOTATED)

deploy/k8s.yaml: deploy/kubernetes/base/*.yaml
	echo "# DO NOT EDIT: generated by 'kustomize build'" > $@
	kustomize build deploy/kubernetes/base >> $@
	cp $@ deploy/k8s_before_v1_18.yaml
	sed -i.orig 's@storage.k8s.io/v1@storage.k8s.io/v1beta1@g' deploy/k8s_before_v1_18.yaml

.PHONY: deploy
deploy: deploy/k8s.yaml
	kubectl apply -f $<

.PHONY: deploy-delete
uninstall: deploy/k8s.yaml
	kubectl delete -f $<

.PHONY: image-dev
image-dev: juicefs-csi-driver
	docker pull $(IMAGE):nightly
	docker build --build-arg TARGETARCH=amd64 -t $(IMAGE):$(DEV_TAG) -f dev.Dockerfile bin

.PHONY: push-dev
push-dev:
ifeq ("$(DEV_K8S)", "microk8s")
	docker image save -o juicefs-csi-driver-$(DEV_TAG).tar $(IMAGE):$(DEV_TAG)
	sudo microk8s.ctr image import juicefs-csi-driver-$(DEV_TAG).tar
	rm -f juicefs-csi-driver-$(DEV_TAG).tar
else ifeq ("$(DEV_K8S)", "kubeadm")
	docker tag $(IMAGE):$(DEV_TAG) $(DEV_REGISTRY):$(DEV_TAG)
	docker push $(DEV_REGISTRY):$(DEV_TAG)
else
	minikube cache add $(IMAGE):$(DEV_TAG)
endif

.PHONY: deploy-dev/kustomization.yaml
deploy-dev/kustomization.yaml:
	mkdir -p $(@D)
	touch $@
	cd $(@D); kustomize edit add resource ../deploy/kubernetes/base;
ifeq ("$(DEV_K8S)", "kubeadm")
	cd $(@D); kustomize edit set image juicedata/juicefs-csi-driver=$(DEV_REGISTRY):$(DEV_TAG)
else
	cd $(@D); kustomize edit set image juicedata/juicefs-csi-driver=:$(DEV_TAG)
endif

deploy-dev/k8s.yaml: deploy-dev/kustomization.yaml deploy/kubernetes/base/*.yaml
	echo "# DO NOT EDIT: generated by 'kustomize build $(@D)'" > $@
	kustomize build $(@D) >> $@
	# Add .orig suffix only for compactiblity on macOS
ifeq ("$(DEV_K8S)", "microk8s")
	sed -i 's@/var/lib/kubelet@/var/snap/microk8s/common/var/lib/kubelet@g' $@
endif
ifeq ("$(DEV_K8S)", "kubeadm")
	sed -i.orig 's@juicedata/juicefs-csi-driver.*$$@$(DEV_REGISTRY):$(DEV_TAG)@g' $@
else
	sed -i.orig 's@juicedata/juicefs-csi-driver.*$$@juicedata/juicefs-csi-driver:$(DEV_TAG)@g' $@
endif

.PHONY: deploy-dev
deploy-dev: deploy-dev/k8s.yaml
	kapp deploy --app juicefs-csi-driver --file $<

.PHONY: delete-dev
delete-dev: deploy-dev/k8s.yaml
	kapp delete --app juicefs-csi-driver

.PHONY: install-dev
install-dev: verify test image-dev push-dev deploy-dev/k8s.yaml deploy-dev

bin/mockgen: | bin
	go install github.com/golang/mock/mockgen@v1.5.0

mockgen: bin/mockgen
	./hack/update-gomock
