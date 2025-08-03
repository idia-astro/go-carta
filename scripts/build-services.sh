#!/bin/bash

if [ ! -d "services" ] || [ ! -d "scripts" ]; then
    echo -e "Error: Could not find the 'services' folder. Please run the script from the project root directory."
    exit 1
fi

mkdir -p build

# list of services
SERVICES=("controller" "worker" "spawner" "api")

# Loop through each service and build it
for SERVICE_NAME in "${SERVICES[@]}"; do
    echo "Building ${SERVICE_NAME}..."
    
    if ! go build -o "./build/${SERVICE_NAME}" "./services/${SERVICE_NAME}/"; then
        echo -e "Error: Failed to build ${SERVICE_NAME}."
        exit 1
    fi
    
    echo "${SERVICE_NAME} built successfully."
    echo
done

echo "All services built successfully!"
