# discord-backend

Mini Discord backend on Go — REST API + WebSocket realtime, PostgreSQL.

## Stack

- **net/http** (stdlib router, Go 1.22+)
- **pgx/v5** — PostgreSQL driver
- **gorilla/websocket** — WS
- **golang-jwt/jwt** — JWT auth
- **bcrypt** — password hashing

## Structure

```
cmd/server/        → entrypoint
internal/
  handler/         → HTTP + WS handlers
  middleware/      → JWT auth middleware
  model/           → data types
  store/           → PostgreSQL queries
  ws/              → WebSocket hub & client
migrations/        → SQL schema
```

## Quickstart

```bash
# 1. DB
createdb discord
psql discord -f migrations/001_init.sql

# 2. Env
cp .env.example .env
# edit .env if needed

# 3. Run
go mod tidy
go run ./cmd/server
```

## API

### Auth
| Method | Path | Body | Response |
|--------|------|------|----------|
| POST | `/api/register` | `{username, password}` | `{user, token}` |
| POST | `/api/login` | `{username, password}` | `{user, token}` |

### Channels (Bearer token required)
| Method | Path | Response |
|--------|------|----------|
| GET | `/api/channels` | список каналов |
| POST | `/api/channels` | `{name, description}` → новый канал |

### Messages (Bearer token required)
| Method | Path | Response |
|--------|------|----------|
| GET | `/api/channels/{id}/messages?limit=50` | история |

### WebSocket
```
GET /ws/channels/{id}?token=<jwt>
```

После подключения отправляй JSON:
```json
{ "content": "Привет!" }
```

Получаешь события:
```json
{ "type": "message", "payload": { ... } }
{ "type": "presence", "payload": { "username": "alex", "online": true } }
```

## Example (httpie)

```bash
# register
http POST :8080/api/register username=alex password=secret

# login → save token
TOKEN=$(http POST :8080/api/login username=alex password=secret | jq -r .token)

# channels
http :8080/api/channels Authorization:"Bearer $TOKEN"

# history
http :8080/api/channels/1/messages Authorization:"Bearer $TOKEN"

# websocket
websocat "ws://localhost:8080/ws/channels/1?token=$TOKEN"
```
