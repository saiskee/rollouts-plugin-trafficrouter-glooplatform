CURRENT_DIR=$(shell pwd)
DIST_DIR=${CURRENT_DIR}/dist

.PHONY: release
release:
	make BIN_NAME=glooplatform-api-plugin-darwin-amd64 GOOS=darwin glooplatform-api-plugin-build
	make BIN_NAME=glooplatform-api-plugin-darwin-arm64 GOOS=darwin GOARCH=arm64 glooplatform-api-plugin-build
	make BIN_NAME=glooplatform-api-plugin-linux-amd64 GOOS=linux glooplatform-api-plugin-build
	make BIN_NAME=glooplatform-api-plugin-linux-arm64 GOOS=linux GOARCH=arm64 glooplatform-api-plugin-build
	make BIN_NAME=glooplatform-api-plugin-windows-amd64.exe GOOS=windows glooplatform-api-plugin-build

.PHONY: glooplatform-api-plugin-build
glooplatform-api-plugin-build:
	CGO_ENABLED=0 GOOS=${GOOS} GOARCH=${GOARCH} go build -v -o ${DIST_DIR}/${BIN_NAME} .

.PHONY: dev
dev:
	kubectl create ns argo-rollouts || true
	skaffold dev -n argo-rollouts 

.PHONY: install-rollouts
install-rollouts:
	kubectl create ns argo-rollouts || true
	kubectl apply -k ./deploy -n argo-rollouts

.PHONY: demo
demo:
	make install-rollouts
	kubectl apply -f ./examples/demo-api-initial-state
	kubectl apply -f ./examples/0-rollout-initial
