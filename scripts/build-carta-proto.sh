#!/bin/bash

if [ ! -d "proto" ] || [ ! -d "scripts" ]; then
    echo -e "Error: Could not find the 'proto' folder. Please run the script from the project root directory."
    exit 1
fi

# Clean up old generated files
rm -rf pkg/cartaDefinitions
mkdir -p pkg/cartaDefinitions

# Generate Go code from proto files
protoc \
    --proto_path=proto/carta-protobuf/control \
    --proto_path=proto/carta-protobuf/request \
    --proto_path=proto/carta-protobuf/shared \
    --proto_path=proto/carta-protobuf/stream \
    --go_out=pkg/cartaDefinitions \
    --go_opt=paths=source_relative \
    proto/carta-protobuf/control/*.proto \
    proto/carta-protobuf/request/*.proto \
    proto/carta-protobuf/shared/*.proto \
    proto/carta-protobuf/stream/*.proto


if [ $? -ne 0 ]; then
    echo -e "Error: Failed to generate Go code from proto files."
    exit 1
fi

echo -e "Successfully generated Go code from proto files!"
echo -e "Generated files are in pkg/carta-protobuf/"
