# OpenAPI-first Go Backend

A Go backend project using OpenAPI-first approach with automatic code generation and Swagger UI documentation.

## Quick Start

```bash
# Install oapi-codegen (first time only)
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

# Run the server
make run

# Or regenerate code and run
make dev
```

Server starts at http://localhost:8080

## Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /hello?name={name}` | Returns greeting message |
| `GET /docs/` | Swagger UI |
| `GET /api/v1/openapi.yaml` | OpenAPI specification |

## Example

```bash
curl 'http://localhost:8080/hello?name=World'
# {"message":"Hello, World World"}
```

## Project Structure

```
.
├── api/v1/
│   ├── openapi.yaml   # API specification (source of truth)
│   ├── cfg.yaml       # Code generator config
│   ├── gen.go         # Generated code (do not edit)
│   └── impl.go        # Handler implementations
├── cmd/server/
│   └── main.go        # Server entry point
├── docs/swagger-ui/   # Swagger UI static files
└── Makefile
```

## Development Workflow

1. **Define API** - Edit `api/v1/openapi.yaml`
2. **Generate code** - Run `make generate`
3. **Implement handlers** - Add handler logic in `api/v1/impl.go`
4. **Run & test** - Run `make run` and test via Swagger UI or curl

## Available Commands

| Command | Description |
|---------|-------------|
| `make run` | Start the server |
| `make generate` | Regenerate code from OpenAPI spec |
| `make dev` | Regenerate and run |
| `make test` | Run tests |
| `make clean` | Remove generated files |

## Tech Stack

- **Go 1.22+** with `net/http` standard library
- **oapi-codegen** for OpenAPI code generation
- **Swagger UI** for API documentation

## Documentation

See [openapi-first-go-guide.md](openapi-first-go-guide.md) for detailed guide on OpenAPI-first development approach.
