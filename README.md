# Waitaminute Digital

Waitaminute Digital is an editorial-style website for indie game development and problem-first software. It is built with Go, Echo, templ, HTMX, and SQLite, with an admin area for managing dispatches, highlights, and contact inquiries.

## Prerequisites

- Go version specified in [`go.mod`](./go.mod)
- [templ](https://templ.guide/quick-start/installation) v0.3.1020
- [Air](https://github.com/air-verse/air) for live reload (optional)

Install the Go tools:

```powershell
go install github.com/a-h/templ/cmd/templ@v0.3.1020
go install github.com/air-verse/air@latest
```

Make sure the Go `bin` directory is on your `PATH`.

## Run locally

Copy `.env.example` to `.env` if you need to override local defaults. On Windows, the database defaults to `./data/site.db`; the server listens on port `8080`.

Generate the templ components and start the web server:

```powershell
templ generate
go run ./cmd/web
```

For automatic rebuilds during development:

```powershell
air
```

Open `http://localhost:8080`. The health endpoint is at `http://localhost:8080/health`.

## Admin password

Generate a bcrypt hash using the interactive tool:

```powershell
go run ./cmd/hashpw
```

Copy the printed hash into `ADMIN_PASSWORD_HASH` in your untracked `.env` file. Do not commit the hash or a real admin password.

## Seed the database

Seed the local SQLite database with the initial article and highlights:

```powershell
go run ./cmd/seed
```

The seed is safe to rerun. Deployment seeding is a separate, explicit operation; see [`DEPLOY.md`](./DEPLOY.md).

## Tests and static checks

```powershell
templ generate
go vet ./...
go test ./...
```

## Deploy

See [`DEPLOY.md`](./DEPLOY.md) for the Azure App Service container build, staging, verification, and cutover instructions.
