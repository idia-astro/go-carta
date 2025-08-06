#!/bin/bash

if [ ! -d "proto" ] || [ ! -d "scripts" ]; then
    echo -e "Error: Could not find the 'proto' folder. Please run the script from the project root directory."
    exit 1
fi

# Clean up old generated files
rm -rf pkg/grpc
mkdir -p pkg/grpc

# Generate Go code from proto files
protoc \
    --proto_path=proto \
    --go_out=pkg/grpc \
    --go_opt=paths=source_relative \
    --go-grpc_out=pkg/grpc \
    --go-grpc_opt=paths=source_relative \
    proto/*.proto


if [ $? -ne 0 ]; then
    echo -e "Error: Failed to generate Go code from proto files."
    exit 1
fi

echo -e "Successfully generated Go code from proto files!"
echo -e "Generated files are in pkg/grpc/"

# List generated files
echo -e "Generated files:"
find pkg/grpc -name "*.go" -type f | sort
