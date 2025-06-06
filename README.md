# NotePulse

Tiny note-taking service with live push updates over WebSocket.

For full specification, see the [project requirements](requirements.md).

## Configuration

The application can be configured using environment variables or a `.env` file:

### Server Configuration
- `APP_PORT` - Server port (default: 8080)
- `LOG_LEVEL` - Logging level: debug, info, warn, error (default: info)
- `LOG_FORMAT` - Log format: json, text (default: json)

### Database Configuration
- `MONGO_URI` - MongoDB connection URI (default: mongodb://mongo:27017)
- `MONGO_DB_NAME` - MongoDB database name (default: notepulse)

**MongoDB Compatibility**: Transactions optional; rotation degrades to create-then-revoke on stand-alone Mongo.

### Authentication Configuration
- `JWT_SECRET` - JWT signing secret (must be at least 32 characters for HS256)
- `JWT_ALGORITHM` - JWT signing algorithm: HS256 (default: HS256), in future plan to support RS256
- `ACCESS_TOKEN_MINUTES` - Access token expiry time in minutes (default: 15)
- `BCRYPT_COST` - Bcrypt cost factor for password hashing (default: 8, range: 8-16)
- `SIGNIN_RATE_PER_MIN` - Rate limit for sign-in attempts per minute (default: 5)

### Refresh Token Configuration
- `REFRESH_TOKEN_DAYS` - Refresh token expiry time in days (default: 30)
- `REFRESH_TOKEN_ROTATE` - Enable refresh token rotation for enhanced security (default: true)

### WebSocket Configuration
- `WS_MAX_SESSION_SEC` - Maximum WebSocket session duration in seconds (default: 900, 15 minutes)
- `WS_OUTBOX_BUFFER` - WebSocket channel buffer size (default: 256)

### Metrics Configuration
- `ROUTE_METRICS_ENABLED` - Enable route metrics (default: true)
- `REQUEST_LOGGING_ENABLED` - Enable HTTP request logs (default: true)

WebSocket sessions are automatically terminated after the configured duration to prevent long-lived connections with expired tokens. Clients should reconnect with fresh JWT tokens when sessions expire.
