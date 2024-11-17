#!/bin/bash

SETTINGS_FILE=".vscode/settings.json"

if [ -f "$SETTINGS_FILE" ]; then
    MONGO_DATABASE=$(awk -F'"' '/MONGO_DATABASE/ {print $4}' "$SETTINGS_FILE")
    MONGO_URI=$(awk -F'"' '/MONGO_URI/ {print $4}' "$SETTINGS_FILE")
    REDIS_ADDR=$(awk -F'"' '/REDIS_ADDR/ {print $4}' "$SETTINGS_FILE")
    LOG_LEVEL=$(awk -F'"' '/LOG_LEVEL/ {print $4}' "$SETTINGS_FILE")

    export MONGO_DATABASE
    export MONGO_URI
    export REDIS_ADDR
    export LOG_LEVEL

    echo "Environment variables exported:"
    echo "MONGO_DATABASE: $MONGO_DATABASE"
    echo "MONGO_URI: $MONGO_URI"
    echo "REDIS_ADDR: $REDIS_ADDR"
    echo "LOG_LEVEL: $LOG_LEVEL"

    # Run go test after exporting variables
    go test -v ./...
else
    echo "Error: $SETTINGS_FILE not found."
fi
