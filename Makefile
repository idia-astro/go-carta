## Makefile inspired by https://github.com/gofiber/fiber/blob/main/Makefile

## help: 💡 Display available commands
.PHONY: help
help:
	@echo 'GoCarta Development:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'

## audit: 🚀 Conduct quality checks
.PHONY: audit
audit:
	go mod verify
	go vet ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

## format: 🎨 Format code
.PHONY: format
format:
	gofmt -l -s -w .

## lint: 🚨 Run lint checks
.PHONY: lint
lint:
	@which golangci-lint > /dev/null || $(MAKE) install-lint
	golangci-lint run

## modernize: 🛠 Run gopls modernize
.PHONY: modernize
modernize:
	go run golang.org/x/tools/gopls/internal/analysis/modernize/cmd/modernize@latest -fix -test=false ./...

## proto: 📦 Compile protobuf files
.PHONY: proto
proto:
	./scripts/build-proto.sh
	./scripts/build-carta-proto.sh

## services: 📦 Compile services
.PHONY: services
services:
	./scripts/build-services.sh

## tidy: 📌 Clean and tidy dependencies
.PHONY: tidy
tidy:
	go mod tidy -v
