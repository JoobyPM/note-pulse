#!/bin/bash
set -euo pipefail

# gen-dev-env.sh - Generate .env file with random secrets for development

ENV_FILE=".env"

# Exit early if .env already exists
if [[ -f "$ENV_FILE" ]]; then
    echo "✓ $ENV_FILE already exists, skipping generation"
    exit 0
fi

echo "🔧 Generating $ENV_FILE with random secrets..."

# Generate a 32-byte base64 JWT secret
JWT_SECRET=$(openssl rand -base64 32)

# Generate a random MongoDB password
MONGO_PASSWORD=$(openssl rand -base64 16)

# Create .env file
cat > "$ENV_FILE" << EOF
# Auto-generated development environment
# Generated on $(date)

# Application Configuration
APP_PORT=8080
LOG_LEVEL=info
LOG_FORMAT=json

# MongoDB Configuration
MONGO_URI=mongodb://root:${MONGO_PASSWORD}@mongo:27017/?authSource=admin
MONGO_DB_NAME=notepulse
MONGO_INITDB_ROOT_PASSWORD=${MONGO_PASSWORD}

# Authentication Configuration
JWT_SECRET=${JWT_SECRET}
JWT_ALGORITHM=HS256
ACCESS_TOKEN_MINUTES=15

# Refresh Token Configuration
REFRESH_TOKEN_DAYS=30
REFRESH_TOKEN_ROTATE=true

# Security Configuration
BCRYPT_COST=8
AUTH_RATE_PER_MIN=10000
APP_RATE_PER_MIN=0

# WebSocket Configuration
WS_MAX_SESSION_SEC=900
WS_OUTBOX_BUFFER=256

# Development Mode
DEV_MODE=true

# Monitoring
ROUTE_METRICS_ENABLED=true
PPROF_ENABLED=false
PYROSCOPE_ENABLED=false
PYROSCOPE_SERVER_ADDR=http://pyroscope:4040
PYROSCOPE_APP_NAME=notepulse-server
EOF

echo "✓ Generated $ENV_FILE with random secrets"
echo "📝 JWT_SECRET: ${#JWT_SECRET} characters"
echo "📝 MONGO_PASSWORD: ${#MONGO_PASSWORD} characters"
