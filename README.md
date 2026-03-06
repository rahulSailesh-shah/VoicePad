# AI Real-Time Voice & Drawing Companion

This project is a real-time, multimodal AI application that enables interactive voice and drawing sessions. The system is built using a microservices architecture that handles real-time audio, gRPC-based streaming Speech-to-Text (STT), AI inference (LLMs), database persistence, and secure authentication.

## System Architecture

The project consists of multiple components working together:

### 1. Main Backend API (Go)

Located in `cmd/api`, `internal/`, and `pkg/`.

- **Framework**: Gin (HTTP Routing).
- **Real-Time Media**: Uses LiveKit Server SDK and Pion WebRTC for handling interactive voice sessions.
- **AI Inference**: Integrates with OpenAI and Ollama.
- **Database**: PostgreSQL (managed via SQLC and Goose for migrations).
- **Job Queuing**: Inngest for background workflows.
- Establishes a clean architecture (Transport -> Service -> App/Domain -> DB).

### 2. Web Frontend (React / Vite)

Located in `frontend/`.

- **Framework**: React via Vite.
- **Routing & State**: TanStack Router and React Query.
- **Package Manager**: Bun.
- Communicates with the Go Backend for application logic and the Auth Service for session management.

### 3. Auth Service (Node.js)

Located in `services/auth/`.

- **Framework**: Hono HTTP server running on Bun.
- **Auth Provider**: Better Auth for handling authentication flows.
- **Database ORM**: Drizzle ORM connecting to PostgreSQL.
- Responsible for all user registration, login, and session persistence.

### 4. Speech Service (Python)

Located in `services/speech/`.

- **Framework**: gRPC Python Server.
- **Functionality**: Provides utterance-level Speech-to-Text (STT) streaming. Emits transcriptions upon detecting complete utterances (post-VAD silence).
- Designed for safe ingestion into the main STT -> LLM -> TTS pipelines.

## Prerequisites

To run this project locally, ensure you have the following installed:

- Go (1.25.1+)
- Bun (latest)
- Python 3
- Docker and Docker Compose (to run PostgreSQL locally)
- [LiveKit Server](https://docs.livekit.io/realtime/server/installation/) (Requires access or local instance)
- Goose (for Go database migrations)
- Inngest CLI (for running local background jobs)

## Environment Configuration

You must configure the root `.env` file before starting the application. Expected core variables include:

- **Database**: `DB_URL`, `DB_PORT`, `DB_USERNAME`, `DB_PASSWORD`, `DB_DATABASE`
- **LiveKit**: `LIVEKIT_URL`, `LIVEKIT_API_KEY`, `LIVEKIT_API_SECRET`
- **AI Providers**: `OPENAI_API_KEY`

## Running the Application

The project includes a `Makefile` at the root directory to orchestrate running the different services.

### 1. Start Infrastructure (PostgreSQL)

```bash
make docker-up
```

### 2. Run Database Migrations

Runs the Goose migrations located in `pkg/database/migrations`:

```bash
make migrate-up
```

### 3. Start the Background Worker (Inngest)

Starts the local Inngest development server:

```bash
make run-inngest
```

### 4. Start the Microservices

You will need to run the following commands in separate terminal windows/tabs:

**Start the Auth Service (Node/Bun)**

```bash
make run-auth
```

**Start the Speech Service (Python gRPC)**
Initializes the Python virtual environment and starts the STT pipeline:

```bash
make run-speech
```

**Start the Main Backend API (Go)**

```bash
make run-backend
```

**Start the Web Frontend (React/Vite)**

```bash
make run-frontend
```

## Protocol Buffers (gRPC)

The contract between the Go Backend and the Python Speech service is defined via Protocol Buffers.
To regenerate the Go code after modifying `pkg/speech/proto/speech.proto`, run:

```bash
make proto-go
```
