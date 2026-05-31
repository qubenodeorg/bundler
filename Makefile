install-dev:
	go install github.com/air-verse/air@latest
	go mod tidy

generate-environment:
	go run ./scripts/genenv

generate-entrypoint-pkg:
	abigen --abi=./abi/entrypoint.json --pkg=entrypoint --out=./pkg/entrypoint/bindings.go

generate-entrypoint-v09-pkg:
	abigen --abi=./abi/entrypoint.v09.json --pkg=entrypointv09 --out=./pkg/entrypoint/v09/bindings.go

fetch-entrypoint-v09-abi:
	curl -sL -o /tmp/v0.9.0.zip "https://github.com/eth-infinitism/account-abstraction/archive/refs/tags/v0.9.0.zip" && \
	unzip -p /tmp/v0.9.0.zip "account-abstraction-0.9.0/deployments/ethereum/EntryPoint.json" > ./abi/entrypoint.v09.json

fetch-wallet:
	go run ./scripts/fetchwallet

dev-private-mode:
	air -c .air.private-mode.toml

dev-searcher-mode:
	air -c .air.searcher-mode.toml

dev-reset-default-data-dir:
	rm -rf /tmp/qubenode_bundler

build:
	go build -v -o qubenode-bundler

docs:
	go install golang.org/x/pkgsite/cmd/pkgsite@latest
	pkgsite -open .
