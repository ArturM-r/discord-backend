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

### 1. Настрой .env

```bash
cp .env.example .env
```

`.env`:
```env
DATABASE_URL=postgres://<user>:<password>@postgres:5432/discord?sslmode=disable
HMAC_KEY=<случайная строка>
POSTGRES_USER=<user>
POSTGRES_PASSWORD=<password>
POSTGRES_DB=discord
ADDR=:8080
```

### 2. Подними Docker

```bash
docker compose up --build -d
```

### 3. Примени миграцию

```bash
cat migrations/001_init.sql | docker compose exec -T postgres psql -U <user> -d discord
```

> Нужно сделать один раз. При повторном запуске таблицы уже есть.

### 4. Проверь

```bash
curl -s -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{"username":"alex","password":"secret"}' | jq .
```

---

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

---

## Примеры

```bash
# register
curl -s -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{"username":"alex","password":"secret"}' | jq .

# login → сохрани токен
TOKEN=$(curl -s -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"username":"alex","password":"secret"}' | jq -r .token)

# список каналов
curl -s http://localhost:8080/api/channels \
  -H "Authorization: Bearer $TOKEN" | jq .

# история сообщений
curl -s http://localhost:8080/api/channels/1/messages \
  -H "Authorization: Bearer $TOKEN" | jq .

# websocket
websocat "ws://localhost:8080/ws/channels/1?token=$TOKEN"
```

---

## Полезные команды

```bash
# логи
docker compose logs -f api

# перезапустить апи после изменений
docker compose up --build -d api

# зайти в базу
docker compose exec postgres psql -U <user> -d discord

# снести всё включая данные
docker compose down -v
```
