#!make
.DEFAULT_GOAL := build

ifndef REGISTRY
	override REGISTRY = halkeye
endif
NAME := incrementals-publisher
VERSION := latest
NAME_VERSION := $(NAME):$(VERSION)
TAGNAME := $(REGISTRY)/$(NAME_VERSION)

.PHONY: build
build: ## Build docker image
	docker build -t $(TAGNAME) .

.PHONY: push
push: ## push to docker hub
	docker push $(TAGNAME)

.PHONY: kill
kill: ## kill the running process
	docker kill $(NAME)

.SHELL := /bin/bash
.PHONY: run
run: ## run the docker hub
	docker run \
		-it \
		--rm \
		-p 3000:3000 \
		--name $(NAME) \
		$(TAGNAME)

.PHONY: test
test: ## run tests
	go test ./...

.PHONY: lint
lint: ## run go vet
	go vet ./...

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
