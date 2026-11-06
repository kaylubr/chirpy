# Chirpy

A Twitter-style REST API written in Go. Users post short messages ("chirps"), authenticate with JWTs, and can upgrade to "Chirpy Red" via a webhook.

Built as part of the [Boot.dev](https://boot.dev) "Learn HTTP Servers in Go" course.

## Features

- User registration, login, and profile updates
- JWT access tokens with revocable refresh tokens
- Password hashing (never stored in plain text)
- Create, list, filter, sort, and delete chirps
- 140-character limit and profanity filtering
- Webhook endpoint (API-key protected) for upgrading users to Chirpy Red
- Request metrics and a dev-only reset endpoint

## Tech Stack

- Go (`net/http` standard library)
- PostgreSQL
- [sqlc](https://sqlc.dev) for type-safe queries
- [goose](https://github.com/pressly/goose) for migrations

## Getting Started

**Prerequisites:** Go 1.22+, PostgreSQL, `goose`, `sqlc`

```bash
# 1. Clone
git clone https://github.com/<your-username>/chirpy.git
cd chirpy

# 2. Create a database
createdb chirpy

# 3. Configure environment
cat > .env <<EOF
DB_URL="postgres://user:password@localhost:5432/chirpy?sslmode=disable"
PLATFORM="dev"
JWT_SECRET="<generate with: openssl rand -base64 64>"
POLKA_KEY="<webhook api key>"
EOF

# 4. Run migrations
goose -dir sql/schema postgres "<DB_URL>" up

# 5. Start the server (listens on :8080)
go run .
```

## API Reference

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| GET | `/api/healthz` | – | Readiness check |
| POST | `/api/users` | – | Create a user |
| PUT | `/api/users` | Bearer JWT | Update email/password |
| POST | `/api/login` | – | Returns access + refresh token |
| POST | `/api/refresh` | Refresh token | Issue a new access token |
| POST | `/api/revoke` | Refresh token | Revoke a refresh token |
| POST | `/api/chirps` | Bearer JWT | Create a chirp |
| GET | `/api/chirps` | – | List chirps (`?author_id=<uuid>&sort=asc\|desc`) |
| GET | `/api/chirps/{chirpID}` | – | Get one chirp |
| DELETE | `/api/chirps/{chirpID}` | Bearer JWT | Delete your own chirp |
| POST | `/api/polka/webhooks` | `ApiKey` header | Upgrade a user to Chirpy Red |
| GET | `/admin/metrics` | – | File server hit count |
| POST | `/admin/reset` | – | Wipe users (dev only) |

### Example

```bash
# Register
curl -X POST localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -d '{"email": "me@example.com", "password": "hunter2"}'

# Log in
curl -X POST localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"email": "me@example.com", "password": "hunter2"}'

# Post a chirp
curl -X POST localhost:8080/api/chirps \
  -H "Authorization: Bearer <access_token>" \
  -H "Content-Type: application/json" \
  -d '{"body": "Hello, Chirpy!"}'
```

## Project Structure

```
.
├── main.go            # Server setup and routing
├── handler_*.go       # HTTP handlers
├── internal/
│   ├── auth/          # Password hashing, JWT, token helpers
│   └── database/      # sqlc-generated code
├── sql/
│   ├── schema/        # goose migrations
│   └── queries/       # sqlc queries
└── sqlc.yaml
```

## What I Learned

- Structuring a Go HTTP server with handlers, middleware, and shared config
- Authentication: password hashing, JWTs, and refresh-token rotation
- Writing migrations and type-safe SQL with goose and sqlc
- Securing webhooks with API keys