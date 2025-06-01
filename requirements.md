# NotePulse project requirements

## Purpose
Build a small REST plus WebSocket service that lets authenticated users create sticky notes and receive live one-way update events when any note changes.

## Stack and versions
| Component         | Version             |
| ----------------- | ------------------- |
| Go                | 1.24.2              |
| Fiber             | 2.x                 |
| MongoDB           | 8.x                 |
| Viper             | latest              |
| slog              | built-in (log/slog) |
| swag              | v2                  |
| websocket contrib | latest compatible   |
| Testify           | latest              |

## Functional scope
* User can sign up and sign in.
* Authenticated user can add, list, update, delete own notes.
* Every note change is broadcast to all connected clients by WebSocket (server -> client only, no client messages).

## API endpoints
| Method | Path             | Description             |
| ------ | ---------------- | ----------------------- |
| POST   | /auth/sign-up    | New user registration   |
| POST   | /auth/sign-in    | Issue JWT               |
| POST   | /notes           | Create note             |
| GET    | /notes           | List notes              |
| PATCH  | /notes/:id       | Edit note               |
| DELETE | /notes/:id       | Delete note             |
| GET    | /ws/notes/stream | WebSocket event channel |

P.S. We have to make pagination for the list of notes.

Total REST endpoints: 6. One WebSocket endpoint.

## WebSocket
To able to use WebSocket in web, we have to pass JWT token in the GET request.

```bash
ws://localhost:8080/ws/notes/stream?token=your_jwt_token
```

We have to use JWT token to identify the user.
## Non-functional requirements
1. Project layout

```bash
.
├── cmd
│   └── server
│       ├── main.go
│       ├── handlers
│       ├── middlewares
│       └── utils
│
├── internal
│   ├── services
│   ├── clients
│   │   └── mongo
│   ├── config
│   └── utils
│
├── api            # swagger-generated docs will live here
│   └── docs
│
├── test           # testify unit tests
│
├── go.mod
├── Dockerfile
└── docker-compose.yaml
```

2. Configuration via Viper (env vars) - logger (log level, log format), mongo (uri, db name, credentials), port.
3. Logger implemented as singleton, JSON output (by default), reusable anywhere.
4. Mongo repository layer behind interfaces.
5. Swagger docs generated and served at /docs/.
6. Unit tests with Testify.
7. Dockerfile must build static binary, docker-compose spins up app, mongo, mongo-express.
8. Use CGO disabled static build suitable for distroless runtime.

## Build and run locally
```bash
# Build and start
docker compose up --build

# Swagger UI once running
open http://localhost:8080/docs/index.html
````