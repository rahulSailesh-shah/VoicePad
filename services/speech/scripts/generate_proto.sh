#!/bin/bash
# Generate Python protobuf files from proto definition

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_DIR="$(dirname "$SCRIPT_DIR")"
PROTO_DIR="$SERVICE_DIR/proto"
OUTPUT_DIR="$SERVICE_DIR/src"

echo "Generating Python protobuf files..."
echo "  Proto dir: $PROTO_DIR"
echo "  Output dir: $OUTPUT_DIR"

python -m grpc_tools.protoc \
    --proto_path="$PROTO_DIR" \
    --python_out="$OUTPUT_DIR" \
    --grpc_python_out="$OUTPUT_DIR" \
    "$PROTO_DIR/speech.proto"

# Fix imports in generated files (grpc_tools generates relative imports incorrectly)
if [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS
    sed -i '' 's/import speech_pb2/from . import speech_pb2/' "$OUTPUT_DIR/speech_pb2_grpc.py"
else
    # Linux
    sed -i 's/import speech_pb2/from . import speech_pb2/' "$OUTPUT_DIR/speech_pb2_grpc.py"
fi

echo "✅ Proto files generated successfully!"
echo "   - $OUTPUT_DIR/speech_pb2.py"
echo "   - $OUTPUT_DIR/speech_pb2_grpc.py"

