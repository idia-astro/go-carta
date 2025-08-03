## Required tools and packages

### Golang

### Protocol buffer compiler and grpc plugin

The protocol buffer compiler and grpc plugin can be installed in a number of ways:

golang:
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

homebrew (macOS):
```bash
brew install brew install protoc-gen-go protoc-gen-go-grpc
```

apt-get (ubuntu):
```bash
sudo apt install protoc-gen-go protoc-gen-go-grpc
```

Tested with: 
- `protoc-gen-go v1.36.6` and `protoc-gen-go-grpc v1.5.1` (macOS)
- `protoc-gen-go v1.32.0` and `protoc-gen-go-grpc v1.0` (ubuntu 24.04.2 LTS)

### cfitsio

TODO, would need to install `libcfitsio-dev` (ubuntu) or `cfitsio` (macOS homebrew)

## Build

gRPC files need to be built first, followed by the services.

```bash
# From the project root
./scripts/build-proto.sh
./scripts/build-services.sh
```

## Run
Compiled services are found in the `build` folder.

```bash
# From the project root
./build/controller --port 8080
./build/worker --remoteAddress localhost:8080
```