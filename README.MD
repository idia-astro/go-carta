# Required tools and packages
### Golang


### Protocol buffer compiler and grpc plugin
The protocol buffer compiler and grpc plugin can be installed by running the following commands:
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```
Tested with `protoc-gen-go v1.36.6` and `protoc-gen-go-grpc v1.5.1`.