#!/bin/sh
# Run DB migrations before booting the API. Both read config from the
# environment (cleanenv), so no .env file is needed in the container.
set -e

echo "core: applying database migrations..."
migrate up

echo "core: starting server..."

exec server
