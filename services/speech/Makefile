.PHONY: install proto server test-mic test-grpc clean help

# Default Python (use python3.11+ for best compatibility)
PYTHON ?= python3

help:
	@echo "Speech Service (STT) - Available commands:"
	@echo ""
	@echo "  make install      Install Python dependencies"
	@echo "  make proto        Generate Python protobuf files"
	@echo "  make server       Start the gRPC server"
	@echo "  make test-mic     Test STT with microphone (standalone)"
	@echo "  make test-session Test session manager with manual audio feed"
	@echo "  make test-grpc    Test gRPC client (requires server running)"
	@echo "  make clean        Remove generated files"
	@echo ""

# Install dependencies
install:
	$(PYTHON) -m pip install -r requirements.txt

# Generate protobuf files
proto:
	chmod +x scripts/generate_proto.sh
	./scripts/generate_proto.sh

# Start gRPC server
server: proto
	$(PYTHON) -m src.server

# Test microphone STT (standalone, no server needed)
test-mic:
	$(PYTHON) -m tests.test_microphone --model base

# Test with session manager (simulates gRPC flow)
test-session:
	$(PYTHON) -m tests.test_microphone --test-session

# Test gRPC client (requires server running in another terminal)
test-grpc: proto
	$(PYTHON) -m tests.test_grpc_client --duration 5 --cleanup

# Clean generated files
clean:
	rm -f src/speech_pb2.py src/speech_pb2_grpc.py
	rm -rf __pycache__ src/__pycache__ tests/__pycache__
	rm -rf .pytest_cache
