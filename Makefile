DOCKER_REPO := public.ecr.aws/d9q4v8a4/apppack-build
TAG := builder

.PHONY: build
build:
	docker build --platform linux/amd64 -t $(DOCKER_REPO):$(TAG) .

.PHONY: build-and-tag
build-and-tag: build
	docker tag $(DOCKER_REPO):$(TAG) $(DOCKER_REPO):$(git describe --tags --always)

.PHONY: ecr-login
ecr-login:
	aws ecr-public get-login-password | docker login --username AWS --password-stdin $(DOCKER_REPO)

.PHONY: push
push:
	docker push $(DOCKER_REPO):$(TAG)

.PHONY: push-tag
push-tag:
	docker push $(DOCKER_REPO):$(git describe --tags --always)
