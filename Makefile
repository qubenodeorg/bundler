install-dev:
	go install github.com/air-verse/air@latest
	go mod tidy

generate-environment:
	go run ./scripts/genenv

generate-entrypoint-pkg: generate-entrypoint-v07 generate-entrypoint-v08 generate-entrypoint-v09

fetch-entrypoint-abis:
	curl -sL "https://registry.npmjs.org/@account-abstraction/contracts/-/contracts-0.7.0.tgz" | tar -xz -C /tmp/pkg_v07 2>/dev/null && python3 -c "import json;d=json.load(open('/tmp/pkg_v07/package/artifacts/EntryPoint.json'));json.dump(d['abi'],open('abi/entrypoint_v07.json','w'),indent=2)" && rm -rf /tmp/pkg_v07 && \
	curl -sL "https://registry.npmjs.org/@account-abstraction/contracts/-/contracts-0.8.0.tgz" | tar -xz -C /tmp/pkg_v08 2>/dev/null && python3 -c "import json;d=json.load(open('/tmp/pkg_v08/package/artifacts/EntryPoint.json'));json.dump(d['abi'],open('abi/entrypoint_v08.json','w'),indent=2)" && rm -rf /tmp/pkg_v08 && \
	curl -sL "https://registry.npmjs.org/@account-abstraction/contracts/-/contracts-0.9.0-rc.1.tgz" | tar -xz -C /tmp/pkg_v09 2>/dev/null && python3 -c "import json;d=json.load(open('/tmp/pkg_v09/package/artifacts/EntryPoint.json'));json.dump(d['abi'],open('abi/entrypoint_v09.json','w'),indent=2)" && rm -rf /tmp/pkg_v09

generate-entrypoint-v07:
	abigen --abi=./abi/entrypoint_v07.json --pkg=v07 --out=./pkg/entrypoint/v07/bindings.go

generate-entrypoint-v08:
	abigen --abi=./abi/entrypoint_v08.json --pkg=v08 --out=./pkg/entrypoint/v08/bindings.go

generate-entrypoint-v09:
	abigen --abi=./abi/entrypoint_v09.json --pkg=v09 --out=./pkg/entrypoint/v09/bindings.go

fetch-wallet:
	go run ./scripts/fetchwallet

dev-private-mode:
	air -c .air.private-mode.toml

dev-searcher-mode:
	air -c .air.searcher-mode.toml

dev-reset-default-data-dir:
	rm -rf /tmp/qubenode_bundler

lint:
	golangci-lint run ./...

build:
	go build -v -o qubenode-bundler

docs:
	go install golang.org/x/pkgsite/cmd/pkgsite@latest
	pkgsite -open .

e2e-fetch-artifacts:
	cd e2e && npm install --silent && npm run --silent fetch-artifacts

e2e:
	scripts/e2e/run.sh
