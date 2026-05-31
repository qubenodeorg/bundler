install-dev:
	go install github.com/air-verse/air@latest
	go mod tidy

generate-environment:
	go run ./scripts/genenv

generate-entrypoint-pkg:
	abigen --abi=./abi/entrypoint.json --pkg=entrypoint --out=./pkg/entrypoint/bindings.go

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
