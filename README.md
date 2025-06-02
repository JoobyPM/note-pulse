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

### Authentication Configuration
- `JWT_SECRET` - JWT signing secret (must be at least 32 characters for HS256)
- `JWT_ALGORITHM` - JWT signing algorithm: HS256, RS256 (default: HS256)
- `JWT_EXPIRY_MINUTES` - JWT token expiry time in minutes (default: 60)
- `BCRYPT_COST` - Bcrypt cost factor for password hashing (default: 12, range: 10-16)
- `SIGNIN_RATE_PER_MIN` - Rate limit for sign-in attempts per minute (default: 5)

### WebSocket Configuration
- `WS_MAX_SESSION_SEC` - Maximum WebSocket session duration in seconds (default: 900, 15 minutes)

WebSocket sessions are automatically terminated after the configured duration to prevent long-lived connections with expired tokens. Clients should reconnect with fresh JWT tokens when sessions expire.
