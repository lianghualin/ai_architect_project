# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Rules

- **Update documentation after every commit**: When making a git commit, review and update README.md and CLAUDE.md if the changes affect documentation (new endpoints, commands, architecture changes, etc.)

## Build & Run Commands

```bash
make run        # Start server on :8080
make generate   # Regenerate code from OpenAPI spec
make dev        # Regenerate and run
make test       # Run all tests
make clean      # Remove generated files
```

## Architecture

This is an **OpenAPI-first** Go backend using `oapi-codegen` for code generation.

### Key Workflow

1. Edit `api/v1/openapi.yaml` (the single source of truth)
2. Run `make generate` to regenerate `api/v1/gen.go`
3. Implement handlers in `api/v1/impl.go` by satisfying `ServerInterface`

### Project Structure

```
api/v1/
├── openapi.yaml   # OpenAPI 3.0 spec - edit this to add/modify endpoints
├── cfg.yaml       # oapi-codegen config
├── gen.go         # AUTO-GENERATED - do not edit
└── impl.go        # Handler implementations (implements ServerInterface)

cmd/server/
└── main.go        # HTTP server setup, serves API + Swagger UI

docs/swagger-ui/   # Static Swagger UI files
```

### Adding a New Endpoint

1. Define the endpoint in `api/v1/openapi.yaml` with `operationId`
2. Run `make generate`
3. Implement the new method in `api/v1/impl.go` matching the generated `ServerInterface`

### URLs

- API: `http://localhost:8080/hello?name=test`
- Swagger UI: `http://localhost:8080/docs/`
- OpenAPI Spec: `http://localhost:8080/api/v1/openapi.yaml`
