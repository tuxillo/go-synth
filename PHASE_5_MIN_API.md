# Phase 5: Minimal REST API (Optional)

## Goals
- Provide simple automation: start builds, query status, list builds.

## Endpoints
- `POST /api/v1/builds`
  - Body: `{ "packages": ["editors/vim"], "profile": "default" }`
  - 201: `{ "build_id": "uuid" }`
- `GET /api/v1/builds/:id`
  - 200: `{ "status": "running|success|failed", "start_time": "...", "end_time": "..." }`
- `GET /api/v1/builds`
  - 200: `{ "items": [ {"uuid": "...", "portdir": "...", "status": "..."} ], "next": "cursor" }`

## Auth
- Header `X-API-Key: <key>`; compare against config value.

## Implementation Notes
- Use a simple router; no WebSocket/SSE; polling only.
- Return errors with `{ status:"error", code, message }`.

## Tasks
- Define handlers and wire to builder/builddb.
- Add API key middleware.
- Write minimal integration tests.

## Exit Criteria
- Can start a build and poll its status via HTTP.

## Dependencies
- Phases 1–3 complete.