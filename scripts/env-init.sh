#!/bin/sh
# Creates local .env files from their examples (idempotent).
# Invoked by `task env:init` on Linux/macOS; see env-init.ps1 for Windows.
set -eu

if [ ! -f .env ]; then
    secret=$(head -c 48 /dev/urandom | base64 | tr -d '\n=/+')
    sed "s|^JWT_SECRET=.*|JWT_SECRET=${secret}|" .env.example > .env
    echo "created .env (with a generated JWT secret)"
else
    echo ".env already exists"
fi

if [ ! -f infra/docker/.env ]; then
    cp infra/docker/.env.example infra/docker/.env
    echo "created infra/docker/.env"
else
    echo "infra/docker/.env already exists"
fi

if [ ! -f frontend/.env.local ]; then
    cp frontend/.env.example frontend/.env.local
    echo "created frontend/.env.local"
else
    echo "frontend/.env.local already exists"
fi
