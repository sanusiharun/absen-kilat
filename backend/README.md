# Absen Kilat — Backend API

API REST untuk aplikasi absensi **Absen Kilat** (Go + chi + SQLite).

## Struktur

```
backend/
├── cmd/api/            # entrypoint server
├── internal/
│   ├── config/         # konfigurasi dari environment
│   ├── domain/         # model & interfaces            (B2)
│   ├── repo/           # implementasi SQLite           (B2)
│   ├── service/        # business logic                (B3)
│   └── transport/      # HTTP handlers & middleware (chi)
├── pkg/
│   ├── errors/         # domain error types
│   └── httpx/          # response/error helpers (kontrak API)
├── migrations/         # file .sql                      (B1)
├── uploads/            # foto absen (gitignored)        (B8)
└── go.mod
```

## Konvensi

- Layered: Transport → Service → Repo → DAO
- Router `chi` + stdlib, response pakai `pkg/httpx`
- Logging `slog` (bukan `fmt.Println`)
- Error wrapping `fmt.Errorf("pkg.Func: %w", err)` + domain error di `pkg/errors`
- Modul: `github.com/sanusi/absen-kilat/backend`

## Kontrak Response

- Sukses: `{ "data": ... }`
- Error: `{ "error": { "code": "...", "message": "..." } }`
- JWT dikirim via header `Authorization: Bearer <token>`

Lihat `~/obsidian-vault/02-Projects/absen-kilat/plan/BUILD-PLAN.md` untuk kontrak lengkap.

## Menjalankan

```sh
cd backend
go run ./cmd/api          # default PORT=8090, APP_ENV=development
```

Health check: `curl localhost:8090/healthz`
