#!make
.DEFAULT_GOAL := build

ifndef REGISTRY
	override REGISTRY = halkeye
endif
NAME    := incrementals-publisher
VERSION := latest
TAGNAME := $(REGISTRY)/$(NAME):$(VERSION)

.PHONY: build
build: ## Build docker image
	docker build -t $(TAGNAME) .

.PHONY: push
push: ## Push to registry
	docker push $(TAGNAME)

.PHONY: kill
kill: ## Kill running container
	docker kill $(NAME)

.PHONY: run
run: ## Run via docker
	docker run \
		-it \
		--rm \
		-p 3000:3000 \
		--name $(NAME) \
		$(TAGNAME)

.PHONY: test
test: ## Run tests
	go test ./...

.PHONY: check
check: test ## Alias for test

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
