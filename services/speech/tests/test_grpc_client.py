#!/usr/bin/env python3
"""
gRPC client test for Speech Service (STT only).

This script tests the gRPC server by:
1. Recording audio from the microphone
2. Streaming it to the gRPC server
3. Getting back the transcription

Usage:
    # First start the server in another terminal:
    python -m src.server
    
    # Then run this test:
    python -m tests.test_grpc_client [--session SESSION_ID]
"""

import argparse
import sys
from pathlib import Path

import grpc

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent))

try:
    from src import speech_pb2
    from src import speech_pb2_grpc
except ImportError:
    print("ERROR: Proto files not generated.")
    print("Run: ./scripts/generate_proto.sh")
    sys.exit(1)


def record_and_stream(stub, session_id: str, duration: float = 5.0) -> str:
    """
    Record audio from microphone and stream to gRPC server.
    Returns transcription.
    """
    try:
        import pyaudio
    except ImportError:
        print("ERROR: pyaudio not installed. Install with: pip install pyaudio")
        return ""
    
    # Audio parameters matching what the server expects
    SAMPLE_RATE = 16000
    CHANNELS = 1
    CHUNK_SIZE = 1024  # ~64ms chunks at 16kHz
    FORMAT = pyaudio.paInt16
    
    audio = pyaudio.PyAudio()
    
    def audio_generator():
        """Generate audio chunks for streaming."""
        stream = audio.open(
            format=FORMAT,
            channels=CHANNELS,
            rate=SAMPLE_RATE,
            input=True,
            frames_per_buffer=CHUNK_SIZE
        )
        
        try:
            chunks_sent = 0
            total_chunks = int(SAMPLE_RATE / CHUNK_SIZE * duration)
            
            print(f"\n🎤 Recording for {duration}s... Speak now!")
            
            for i in range(total_chunks):
                data = stream.read(CHUNK_SIZE, exception_on_overflow=False)
                chunks_sent += 1
                
                # Progress indicator
                progress = int((i / total_chunks) * 20)
                bar = "█" * progress + "░" * (20 - progress)
                print(f"\r   [{bar}] {i+1}/{total_chunks} chunks", end="", flush=True)
                
                yield speech_pb2.TranscribeRequest(
                    session_id=session_id,
                    audio_chunk=data,
                    end_of_stream=False
                )
            
            # Send final request to signal end of stream
            print(f"\n\n📤 Sending end-of-stream signal...")
            yield speech_pb2.TranscribeRequest(
                session_id=session_id,
                audio_chunk=b"",
                end_of_stream=True
            )
            
        finally:
            stream.stop_stream()
            stream.close()
    
    try:
        print(f"\n{'='*60}")
        print(f"Testing gRPC StreamTranscribe")
        print(f"Session ID: {session_id}")
        print(f"{'='*60}")
        
        response = stub.StreamTranscribe(audio_generator())
        
        if response.success:
            print(f"\n✅ Transcription successful!")
            print(f"   Result: {response.transcription}")
            return response.transcription
        else:
            print(f"\n❌ Transcription failed: {response.error}")
            return ""
            
    except grpc.RpcError as e:
        print(f"\n❌ gRPC error: {e.code()} - {e.details()}")
        return ""
    finally:
        audio.terminate()


def test_cleanup(stub, session_id: str):
    """Test session cleanup."""
    print(f"\n🧹 Cleaning up session: {session_id}")
    
    try:
        response = stub.CleanupSession(
            speech_pb2.CleanupRequest(session_id=session_id)
        )
        
        if response.success:
            print("✅ Session cleaned up successfully")
        else:
            print("⚠️  Session cleanup returned false (may not exist)")
            
    except grpc.RpcError as e:
        print(f"❌ gRPC error: {e.code()} - {e.details()}")


def main():
    parser = argparse.ArgumentParser(
        description="Test Speech Service (STT) gRPC server with microphone"
    )
    parser.add_argument(
        "--host",
        type=str,
        default="localhost",
        help="gRPC server host (default: localhost)"
    )
    parser.add_argument(
        "--port",
        type=int,
        default=50051,
        help="gRPC server port (default: 50051)"
    )
    parser.add_argument(
        "--session",
        type=str,
        default="test-board:test-user",
        help="Session ID (default: test-board:test-user)"
    )
    parser.add_argument(
        "--duration",
        type=float,
        default=5.0,
        help="Recording duration in seconds (default: 5)"
    )
    parser.add_argument(
        "--cleanup",
        action="store_true",
        help="Cleanup session after test"
    )
    
    args = parser.parse_args()
    
    address = f"{args.host}:{args.port}"
    
    print("\n" + "="*60)
    print("  Speech Service (STT) - gRPC Client Test")
    print(f"  Server: {address}")
    print("="*60)
    
    # Create gRPC channel and stub
    channel = grpc.insecure_channel(address)
    stub = speech_pb2_grpc.SpeechServiceStub(channel)
    
    try:
        # Test connection
        print(f"\n📡 Connecting to {address}...")
        grpc.channel_ready_future(channel).result(timeout=5)
        print("✅ Connected!")
        
    except grpc.FutureTimeoutError:
        print(f"❌ Could not connect to server at {address}")
        print("   Make sure the server is running: python -m src.server")
        sys.exit(1)
    
    try:
        # Test STT
        transcription = record_and_stream(
            stub,
            session_id=args.session,
            duration=args.duration
        )
        
        if transcription:
            print(f"\n💡 To speak this text in the browser, use:")
            print(f'   speechSynthesis.speak(new SpeechSynthesisUtterance("{transcription}"))')
        
        # Optionally cleanup
        if args.cleanup:
            test_cleanup(stub, args.session)
            
    finally:
        channel.close()
    
    print("\n" + "="*60)
    print("  Test Complete")
    print("="*60 + "\n")


if __name__ == "__main__":
    main()
