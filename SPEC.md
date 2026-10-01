You are my senior Go tech lead AND my instructor. I'm new to Go (coming from Python/FastHTML). We're rebuilding my website Waitaminute Digital in Go. Follow the Ghost Rider rule: write working code AND explain the Go concepts behind it (packages, structs, interfaces, error handling, pointers, context, goroutines) the first time each one appears. Keep explanations short and practical. Don't call me a senior developer.

STACK (do not substitute):
- Go (latest stable), Echo (latest stable, confirm the version before using it), templ for HTML components, HTMX for interactivity
- SQLite via modernc.org/sqlite (pure Go, no CGO, so Docker and my ARM64 Windows laptop work without a C compiler)
- database/sql with hand-written SQL (I want to learn SQL + Go, no ORM). Migrations as embedded .sql files run at startup.
- goldmark for Markdown, sanitized with bluemonday before rendering with templ.Raw
- golang.org/x/crypto/bcrypt for the admin password; alexedwards/scs for sessions
- air for hot reload in development
- Docker (multi-stage build) on Azure App Service

REFERENCE SPEC: My previous FastHTML version lives at C:\dev\waitaminutedigital_fh. Treat it as the spec: same pages, routes, data models, design tokens (static/css/site.css), and brand images (static/img). Copy static assets over. Do NOT copy Python code; rewrite everything idiomatically in Go.

PROJECT LAYOUT (standard Go):
cmd/web/main.go            // entry point, wiring only
internal/config            // env vars
internal/db                // open DB, migrations, queries
internal/models            // Article, Highlight, Inquiry structs
internal/handlers          // Echo handlers per page
internal/views             // .templ components (layout, pages, partials)
internal/auth              // password hashing, admin middleware
static/                    // css, img, js
migrations/                // 001_init.sql ...
scripts/ or cmd/seed       // idempotent seed

LESSONS FROM THE LAST BUILD (apply these):
- Create .gitignore first: bin/, tmp/, *.db, data/, .env, *_templ.go is NOT ignored (commit generated files so Docker builds don't need templ).
- Every route declares its HTTP method explicitly (e.GET / e.POST).
- Markdown must render as real HTML, not escaped text. Add a test for it.
- Seed sets created_at ONLY on insert; re-running never changes dates.
- HTMX endpoints return fragments only (check the HX-Request header); full loads of the same URL render the full page so shared links work.
- Clean URLs (/dispatches/{slug}), never hash routes.
- DB path from env SITE_DB (default /home/data/site.db on Linux, ./data/site.db on Windows). Port from env PORT, default 8080.

DESIGN: IGN-style game/editorial. Background #0a0a0c, purple #8b5cf6, cyan #22d3ee, gold #f59e0b tags, monospace uppercase labels, rounded cards with glow on hover, sticky nav (logo-128.webp, "WAITAMINUTE", cyan "EDITORIAL" badge, gradient Contact button), accessible hamburger (sr-only checkbox, aria-label), footer year computed at runtime, GitHub https://github.com/CodeToole and LinkedIn https://www.linkedin.com/in/corneliustoole/.

PHASES — at the end of each: run go vet and go test ./..., show me the exact commands to run locally, summarize what changed, then STOP until I say "continue".

Phase 0 — Setup: go mod init github.com/CodeToole/waitaminutedigital_go, install deps, .gitignore, air config, copy static assets, a /health route returning JSON. Explain go.mod, packages, and how main.go wires things.
Phase 1 — Layout: templ base layout with SEO/Open Graph/Twitter tags (absolute URLs from SITE_URL, default https://waitaminutedigital.com), nav, footer, HTMX loaded once. Explain templ components vs Go html/template.
Phase 2 — Data + Home: migrations, models, queries. Home hero (kicker "INDIE GAME DEV · PYTHON · GO · PROBLEM-FIRST SOFTWARE", headline "Building games, tools, and software that solve real problems.", Digit mascot in <picture> webp+png with width/height and fetchpriority=high, "STATUS: ACTIVE" card), highlights carousel (scroll-snap, accessible arrow buttons, lazy images, object-fit: contain), latest dispatches with HTMX category chips (All, Devlog, Post-Mortem, Shipped, Game Room, News) using hx-push-url. Empty states. Explain error handling (if err != nil) and structs.
Phase 3 — Dispatches: list with filters + pagination (10/page), article page (sanitized Markdown, gold tag, date, read time ~200 wpm, summary callout, share buttons X/Facebook/LinkedIn/Copy Link, og:type=article), 404 for missing/unpublished. Idempotent seed: one Devlog article "Building an Expository Teaching Engine: Bible Study App" (slug building-an-expository-teaching-engine-bible-study-app, summary "An offline-first Flutter study tool engineered for deep expository scripture study, automatic lesson outline parsing, interactive Podium Mode, and classroom syllabus PDF generation.", body "## Full write-up coming soon" plus one sentence) and 2 highlights: Bible Study App (sort 1), Game Room (/game-room, kicker "Coming Soon", sort 2). No Acting Collective anywhere.
Phase 4 — Game Room (retro "First build loading…" with animated progress bar respecting prefers-reduced-motion, structured for future game cards), Projects (Bible Study App, this Go website), About (I'm Neil, founder of Waitaminute Digital, Mobile, Alabama, originally from Buffalo, NY; developer and indie game dev with Godot who starts with the customer's problem, not the technology; humble tone), Contact (HTMX form: name, email, subject dropdown Game Dev/Custom Software/Automation/Other, message; server validation, honeypot, inline success, saves Inquiry).
Phase 5 — Admin: /admin/login with bcrypt hash from ADMIN_PASSWORD_HASH (add a small cmd/hashpw tool), scs sessions, login rate limit, Echo CSRF middleware on all admin POSTs, admin middleware on /admin/*. Tabs: Dispatches (CRUD, slug auto-generate, HTMX publish toggle, live Markdown preview debounced 500ms, cover upload), Highlights (CRUD, sort, publish), Inquiries (unread badge, mark read, delete). Uploads: png/jpg/webp only, max 5MB, random filenames, stored in the persistent data dir, served at /uploads/. Explain middleware and closures.
Phase 6 — SEO: /sitemap.xml, /robots.txt, /feed.xml (RSS), canonical URLs, styled 404 and 500 pages via Echo's HTTPErrorHandler.
Phase 7 — Deploy: multi-stage Dockerfile (golang build → small runtime image, non-root, static binary, EXPOSE 8080), .dockerignore, .env.example with every variable, DEPLOY.md with az CLI steps only (Azure Container Registry, App Service for containers, WEBSITES_PORT=8080, WEBSITES_ENABLE_APP_SERVICE_STORAGE=true, SITE_DB=/home/data/site.db, custom domain waitaminutedigital.com), and a GitHub Actions workflow that tests, builds, and deploys on push to main using repo secrets.

Write table-driven tests for handlers using httptest. Keep dependencies to the list above unless you ask me first. Start with Phase 0 only.