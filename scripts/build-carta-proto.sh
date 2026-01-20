#!/bin/bash

if [ ! -d "proto" ] || [ ! -d "scripts" ]; then
    echo -e "Error: Could not find the 'proto' folder. Please run the script from the project root directory."
    exit 1
fi

# Clean up old generated files
rm -rf pkg/cartaDefinitions
mkdir -p pkg/cartaDefinitions

# Generate go_opt flags dynamically for each proto file, rather than specifying the go_package option in each source file
GO_PACKAGE="./cartaDefinitions"
GO_OPT_FLAGS=""
for dir in control request shared stream; do
    for proto_file in proto/carta-protobuf/$dir/*.proto; do
        if [ -f "$proto_file" ]; then
            filename=$(basename "$proto_file")
            GO_OPT_FLAGS="$GO_OPT_FLAGS --go_opt=M$filename=$GO_PACKAGE"
        fi
    done
done

# Generate Go code from proto files
protoc \
    --proto_path=proto/carta-protobuf/control \
    --proto_path=proto/carta-protobuf/request \
    --proto_path=proto/carta-protobuf/shared \
    --proto_path=proto/carta-protobuf/stream \
    --go_out=pkg/cartaDefinitions \
    --go_opt=paths=source_relative \
    $GO_OPT_FLAGS \
    proto/carta-protobuf/control/*.proto \
    proto/carta-protobuf/request/*.proto \
    proto/carta-protobuf/shared/*.proto \
    proto/carta-protobuf/stream/*.proto


if [ $? -ne 0 ]; then
    echo -e "Error: Failed to generate Go code from proto files."
    exit 1
fi

echo -e "Successfully generated Go code from proto files!"
