# Chirpy

A small Twitter-like HTTP API written in Go. Users can register, log in, post short
"chirps" (max 140 characters), edit their account, and manage refresh tokens. Certain
words are automatically censored, chirps can be fetched/sorted by author, and users can
be upgraded to "Chirpy Red" via a webhook.

## Why care?

Chirpy is a compact, dependency-light reference for a real backend: JWT auth, hashed
passwords (argon2id), refresh-token rotation/revocation, PostgreSQL persistence with
`sqlc`, and goose migrations. It's a good starting point or learning project for a
production-shaped Go API.

## Requirements

- Go 1.27+
- PostgreSQL
- [goose](https://github.com/pressly/goose) (migrations)
- [sqlc](https://sqlc.dev) (optional, for regenerating query code)

## Setup

1. Clone the repo and install dependencies:

   ```sh
   git clone <repo-url> chirpy-app
   cd chirpy-app
   go mod download
   ```

2. Create a `.env` file in the project root:

   ```env
   DB_URL=postgres://user:password@localhost:5432/chirpy?sslmode=disable
   TOKEN_SECRET=your-jwt-signing-secret
   POLKA_KEY=your-polka-webhook-api-key
   PLATFORM=dev
   ```

   `PLATFORM=dev` is required for the `/admin/reset` endpoint.

3. Run the database migrations:

   ```sh
   goose -dir sql/schema postgres "$DB_URL" up
   ```

4. (Optional) regenerate database code after changing queries:

   ```sh
   sqlc generate
   ```

5. Run the server:

   ```sh
   go run .
   ```

   The API listens on `http://localhost:8080`.

## API

Base URL: `http://localhost:8080`

Authenticated endpoints expect `Authorization: Bearer <access_token>`.
The Polka webhook expects `Authorization: ApiKey <polka_key>`.

### Health & admin

| Method | Path | Description |
| ------ | ---- | ----------- |
| GET | `/api/healthz` | Health check. Returns `200 OK` with `OK` (text/plain). |
| GET | `/admin/metrics` | HTML page with the file-server hit count. |
| POST | `/admin/reset` | Deletes all users (dev only; requires `PLATFORM=dev`). Returns `200`, `403` outside dev. |

### Users

**Create user** — `POST /api/users`

```json
{ "email": "user@example.com", "password": "supersecret" }
```

`201 Created`:

```json
{
  "id": "uuid",
  "email": "user@example.com",
  "created_at": "2026-01-01T00:00:00Z",
  "updated_at": "2026-01-01T00:00:00Z",
  "is_chirpy_red": false
}
```

**Update user** — `PUT /api/users` (auth required)

```json
{ "email": "new@example.com", "password": "newsecret" }
```

`200 OK` with the updated user.

### Auth

**Login** — `POST /api/login`

```json
{ "email": "user@example.com", "password": "supersecret" }
```

`200 OK`:

```json
{
  "id": "uuid",
  "email": "user@example.com",
  "created_at": "...",
  "updated_at": "...",
  "token": "<access_token>",
  "refresh_token": "<refresh_token>",
  "is_chirpy_red": false
}
```

Access tokens expire after 1 hour. Refresh tokens expire after 24 hours.

**Refresh** — `POST /api/refresh` (auth header: `Bearer <refresh_token>`)

`200 OK`:

```json
{ "token": "<new_access_token>" }
```

**Revoke** — `POST /api/revoke` (auth header: `Bearer <refresh_token>`)

`204 No Content`.

### Chirps

**List chirps** — `GET /api/chirps`

Query params:
- `author_id` — filter by author UUID
- `sort=desc` — newest first (default is ascending)

`200 OK`:

```json
[
  {
    "id": "uuid",
    "created_at": "...",
    "updated_at": "...",
    "body": "hello world",
    "user_id": "uuid"
  }
]
```

**Get a chirp** — `GET /api/chirps/{chirpID}`

`200 OK` with a single chirp, or `404` if not found.

**Create a chirp** — `POST /api/chirps` (auth required)

```json
{ "body": "hello world" }
```

- Body must be ≤ 140 characters (`400` otherwise).
- Censored words: `kerfuffle`, `sharbert`, `fornax` are replaced with `****`.

`201 Created` with the created chirp.

**Delete a chirp** — `DELETE /api/chirps/{chirpID}` (auth required)

Only the chirp's author may delete it. `204 No Content` on success, `403` if not the author, `404` if missing.

### Webhooks

**Polka** — `POST /api/polka/webhooks`

Header: `Authorization: ApiKey <polka_key>`

```json
{
  "event": "user.upgraded",
  "data": { "user_id": "uuid" }
}
```

Marks the user as Chirpy Red. Unknown events return `204 No Content`.

## Project layout

```
main.go                 HTTP server and route handlers
handler/                JSON helpers
internal/auth/          password hashing, JWT, tokens, header parsing
internal/database/      sqlc-generated DB code
sql/queries/            SQL queries
sql/schema/             goose migrations
```

## Tests

```sh
go test ./...
```
