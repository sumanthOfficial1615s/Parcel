# Parcel

A backend file-sharing service — upload a file, get back a link that's password-protected, expires automatically, and self-destructs after one download.

Built to explore the backend/infra concepts that come up in real systems: streaming large files without buffering them in memory, object storage via the S3 API, atomic database operations under concurrent access, and async job processing — rather than as a tutorial clone.

## Features

- **Streamed uploads** — files are never fully buffered in memory, even for large uploads; the server hashes and uploads in a single pass over the data
- **Object storage** — files are stored in MinIO (S3-compatible), not on the server's own disk
- **Password protection** — a link can require a password before the file is downloadable
- **Auto expiry** — links stop working after a set time
- **One-time download** — a link is invalidated after exactly one successful download, enforced atomically to stay correct under concurrent requests
- **Content hashing** — every upload is SHA-256 hashed on the fly, with no extra pass over the file

### Planned

- Virus scanning (ClamAV) before a file becomes downloadable
- Rate limiting and abuse prevention
- Chunked, resumable uploads for very large files
- Optional P2P transfer mode (WebRTC), where two browsers exchange a file directly with zero bandwidth cost to the server

## Tech stack

| Layer | Choice |
|---|---|
| Language | Go |
| HTTP router | [Gin](https://github.com/gin-gonic/gin) |
| Object storage | [MinIO](https://min.io/) (S3-compatible), self-hosted via Docker |
| Database | PostgreSQL |
| Rate limiting | Redis (token-bucket) |

## Design notes

**No user accounts.** Security is per-link rather than per-user — a password, an expiry, and a one-time-use flag protect each file individually. This matches how the product is actually used (anyone can send a file without signing up), and avoids adding an authentication layer the product doesn't need.

**Streaming architecture.** The upload handler reads the incoming multipart request directly via `MultipartReader`, rather than buffering it with `ParseMultipartForm`. Each file part is piped through `io.MultiWriter` into both a SHA-256 hasher and MinIO's `PutObject` simultaneously — one pass over the data, constant memory usage regardless of file size.

**Project layout** follows a `cmd/` + `internal/` structure: only `cmd/` folders are executable entrypoints (the real server, plus a throwaway MinIO connectivity check); everything else under `internal/` is a reusable package, not tied to any one entrypoint.

```
parcel/
├── cmd/
│   ├── server/        — main HTTP server
│   └── minio-test/     — standalone MinIO connectivity check
├── internal/
│   ├── storage/        — MinIO client + Postgres FileStore
│   └── controllers/     — HTTP handlers
└── client/              — minimal HTML test frontend
```

## Running locally

```bash
docker compose up -d       # starts MinIO
go run ./cmd/minio-test    # optional: verify the MinIO connection in isolation
go run ./cmd/server        # starts the API on :8080
```

Open `client/upload.html` in a browser and point it at `http://localhost:8080/upload`.

## Status

Core streaming upload pipeline is working end-to-end: a file streams into MinIO while being hashed, with password and expiry required before the file is accepted. Persistence (saving upload metadata to Postgres) and the download side (password check, expiry check, one-time-use enforcement) are in progress.
