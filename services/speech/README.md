# Speech Service (STT Only)

A Python gRPC service providing Speech-to-Text (STT) transcription using [RealtimeSTT](https://github.com/KoljaB/RealtimeSTT).

> **Note:** TTS (Text-to-Speech) is handled client-side using the browser's Web Speech API for better performance and reliability.

## Features

- **STT**: Whisper-based transcription with built-in VAD (Voice Activity Detection)
- **Session Isolation**: Each user session has isolated audio buffers
- **gRPC Interface**: For integration with Go backend

## Quick Start

### 1. Install Dependencies

```bash
cd services/speech
python3 -m venv venv
source venv/bin/activate  # On Windows: venv\Scripts\activate
make install
```

### 2. Generate Proto Files

```bash
make proto
```

### 3. Test with Microphone (Standalone)

Test STT without starting the gRPC server:

```bash
make test-mic
```

### 4. Run the gRPC Server

```bash
make server
```

### 5. Test gRPC Client

In another terminal:

```bash
make test-grpc
```

## Configuration

Configuration via environment variables:

| Variable                 | Default   | Description                                         |
| ------------------------ | --------- | --------------------------------------------------- |
| `STT_MODEL`              | `base`    | Whisper model (tiny, base, small, medium, large-v2) |
| `STT_LANGUAGE`           | `en`      | Language for transcription                          |
| `STT_SILERO_SENSITIVITY` | `0.6`     | VAD sensitivity (0.0-1.0)                           |
| `STT_SILENCE_DURATION`   | `0.4`     | Seconds of silence to end recording                 |
| `GRPC_HOST`              | `0.0.0.0` | Server bind address                                 |
| `GRPC_PORT`              | `50051`   | Server port                                         |

## Proto Definition

The gRPC service is defined in `proto/speech.proto`:

```protobuf
service SpeechService {
  // Stream audio chunks, get transcription on end_of_stream
  rpc StreamTranscribe(stream TranscribeRequest) returns (TranscribeResponse);

  // Cleanup session resources
  rpc CleanupSession(CleanupRequest) returns (CleanupResponse);
}
```

## Session Management

Sessions are identified by `session_id` (format: `boardId:participantId`).

Each session maintains:

- Isolated audio buffer
- Lazy-loaded STT recorder (Whisper model)

Sessions are created on first audio chunk and should be cleaned up when users disconnect.

## Browser TTS

For Text-to-Speech, use the browser's built-in Web Speech API:

```javascript
function speak(text) {
  const utterance = new SpeechSynthesisUtterance(text);
  utterance.rate = 1.0;
  utterance.pitch = 1.0;
  window.speechSynthesis.speak(utterance);
}

// When you receive transcription from the server
speak("Hello, how are you?");
```

This approach is faster, more reliable, and doesn't require server resources.

## Testing

### Standalone Tests (no server needed)

```bash
# Test RealtimeSTT with microphone
python -m tests.test_microphone --model base

# Test session manager
python -m tests.test_microphone --test-session
```

### gRPC Tests (requires server running)

```bash
# Start server
make server

# In another terminal
python -m tests.test_grpc_client --duration 5 --cleanup
```

## Directory Structure

```
services/speech/
├── proto/
│   └── speech.proto          # gRPC service definition
├── src/
│   ├── __init__.py
│   ├── config.py             # Configuration
│   ├── session_manager.py    # Per-session STT handling
│   ├── server.py             # gRPC server
│   ├── speech_pb2.py         # Generated
│   └── speech_pb2_grpc.py    # Generated
├── tests/
│   ├── __init__.py
│   ├── test_microphone.py    # Standalone microphone test
│   └── test_grpc_client.py   # gRPC client test
├── scripts/
│   └── generate_proto.sh     # Proto generation script
├── Makefile
├── requirements.txt
└── README.md
```
