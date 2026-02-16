# Test Bot

A simple Go REST API server for managing AI models with SQLite persistence and Prometheus metrics.

## Features

- **CRUD Operations**: Create, read, update, and delete AI model configurations
- **SQLite Storage**: Persistent storage using SQLite
- **Prometheus Metrics**: Built-in observability with counters for all operations
- **RESTful API**: Clean API design using Gorilla Mux router

## API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/getkey` | Retrieve JWT access token for a user |
| GET | `/api/users` | List all users |
| POST | `/api/users` | Create a new user |
| GET | `/api/users/{name}` | Get a specific user |
| PUT | `/api/users/{name}` | Update a user |
| DELETE | `/api/users/{name}` | Delete a user |
| GET | `/api/models` | List all models |
| POST | `/api/models` | Create a new model |
| GET | `/api/models/{name}` | Get a specific model |
| PUT | `/api/models/{name}` | Update a model |
| DELETE | `/api/models/{name}` | Delete a model |
| GET | `/metrics` | Prometheus metrics endpoint |

## Quick Start

### Prerequisites

- Go 1.21+
- GCC and SQLite3 development libraries

### Installation

```bash
# Install dependencies (Ubuntu/Debian)
sudo apt-get install -y gcc libsqlite3-dev

# Clone and build
git clone https://github.com/Fei-Guo/test-bot.git
cd test-bot
go mod download
CGO_ENABLED=1 go build -o model-server .
```

### Run the Server

```bash
# Default (uses models.db in current directory)
./model-server

# With custom database path
DB_PATH=/path/to/custom.db ./model-server
```

The server starts on port `8080`.

### Authentication

The model CRUD APIs are protected by a JWT access token that encodes the user name.

- The server uses the `API_TOKEN` environment variable as the JWT signing secret if set.
- If `API_TOKEN` is not set, a random signing secret is generated at startup.
- The current token can be retrieved from the `/getkey` endpoint by providing a user name.
- An optional `role` query parameter is included in the token when provided.
- Model CRUD requests require the token's user name to be registered in the users table.

```bash
# Get a JWT token for a user
curl "http://localhost:8080/getkey?name=alice"
# => {"token":"<your-token>","name":"alice"}
```

All model CRUD requests must include the token using the `X-API-Key` header
(or `Authorization: Bearer <token>`):

```bash
TOKEN=$(curl -s "http://localhost:8080/getkey?name=alice" | jq -r .token)
```

## Example Usage

```bash
# Create a user
curl -X POST http://localhost:8080/api/users \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $TOKEN" \
  -d '{"name": "alice", "email": "alice@example.com"}'

# List all users
curl http://localhost:8080/api/users \
  -H "X-API-Key: $TOKEN"

# Get a specific user
curl http://localhost:8080/api/users/alice \
  -H "X-API-Key: $TOKEN"

# Update a user
curl -X PUT http://localhost:8080/api/users/alice \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $TOKEN" \
  -d '{"name": "alice", "email": "alice+updated@example.com"}'

# Delete a user
curl -X DELETE http://localhost:8080/api/users/alice \
  -H "X-API-Key: $TOKEN"

# Create a model
curl -X POST http://localhost:8080/api/models \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $TOKEN" \
  -d '{"name": "gpt-4", "url": "https://api.openai.com/v1/models/gpt-4"}'

# List all models
curl http://localhost:8080/api/models \
  -H "X-API-Key: $TOKEN"

# Get a specific model
curl http://localhost:8080/api/models/gpt-4 \
  -H "X-API-Key: $TOKEN"

# Update a model
curl -X PUT http://localhost:8080/api/models/gpt-4 \
  -H "Content-Type: application/json" \
  -H "X-API-Key: $TOKEN" \
  -d '{"name": "gpt-4", "url": "https://api.openai.com/v1/models/gpt-4-turbo"}'

# Delete a model
curl -X DELETE http://localhost:8080/api/models/gpt-4 \
  -H "X-API-Key: $TOKEN"

# View Prometheus metrics
curl http://localhost:8080/metrics
```

## Metrics

The following Prometheus counters are exposed:

- `models_created_total` - Total number of models created
- `models_read_total` - Total number of read operations
- `models_updated_total` - Total number of models updated
- `models_deleted_total` - Total number of models deleted

## Testing

```bash
# Run unit tests
CGO_ENABLED=1 go test -v ./... -run '^Test[^E]'

# Run E2E tests (requires building the server first)
CGO_ENABLED=1 go test -v -run TestE2E ./...

# Run all tests
CGO_ENABLED=1 go test -v ./...
```

## Project Structure

```
.
├── main.go           # Main application with HTTP handlers
├── main_test.go      # Unit tests
├── e2e_test.go       # End-to-end integration tests
├── go.mod            # Go module definition
└── go.sum            # Go module checksums
```

## CI/CD

The project uses GitHub Actions for continuous integration:

- **E2E Tests**: Automated integration tests on PR/push to main
- **Build verification**: Ensures the server compiles successfully

## License

[Your License Here]
