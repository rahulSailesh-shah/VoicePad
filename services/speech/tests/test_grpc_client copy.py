#!/usr/bin/env python3
"""
gRPC client test for Speech Service (STT only) with VAD.

This script tests the gRPC server by:
1. Recording audio from the microphone
2. Streaming it to the gRPC server (bidirectional streaming)
3. Receiving transcriptions automatically when VAD detects silence

Usage:
    # First start the server in another terminal:
    python -m src.server
    
    # Then run this test:
    python -m tests.test_grpc_client [--session SESSION_ID]
"""

import argparse
import sys
import threading
import time
from pathlib import Path
from queue import Queue, Empty

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


def record_and_stream(stub, session_id: str, duration: float = 10.0) -> list[str]:
    """
    Record audio from microphone and stream to gRPC server.
    Receives transcriptions automatically when VAD detects silence.
    Returns list of transcriptions (one per speech segment).
    """
    try:
        import pyaudio
    except ImportError:
        print("ERROR: pyaudio not installed. Install with: pip install pyaudio")
        return []
    
    # Audio parameters matching what the server expects
    SAMPLE_RATE = 16000
    CHANNELS = 1
    CHUNK_SIZE = 1024  # ~64ms chunks at 16kHz
    FORMAT = pyaudio.paInt16
    
    audio = pyaudio.PyAudio()
    transcriptions = []
    transcription_queue = Queue()
    send_done = threading.Event()
    receive_done = threading.Event()
    error_occurred = threading.Event()
    
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
            print("   (Transcriptions will appear automatically when VAD detects silence)\n")
            
            for i in range(total_chunks):
                if error_occurred.is_set():
                    break
                    
                data = stream.read(CHUNK_SIZE, exception_on_overflow=False)
                chunks_sent += 1
                
                # Progress indicator
                progress = int((i / total_chunks) * 20)
                bar = "█" * progress + "░" * (20 - progress)
                elapsed = (i * CHUNK_SIZE) / SAMPLE_RATE
                print(f"\r   [{bar}] {elapsed:.1f}s/{duration:.1f}s", end="", flush=True)
                
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
            send_done.set()
    
    def receive_transcriptions(stream):
        """Receive transcriptions from the server."""
        try:
            for response in stream:
                if response.success and response.transcription:
                    transcription = response.transcription.strip()
                    if transcription:
                        transcriptions.append(transcription)
                        transcription_queue.put(transcription)
                        print(f"\n✅ [Transcription #{len(transcriptions)}]: {transcription}")
                elif not response.success:
                    error_msg = f"Transcription error: {response.error}"
                    print(f"\n❌ {error_msg}")
                    error_occurred.set()
                    transcription_queue.put(None)
                    
        except grpc.RpcError as e:
            print(f"\n❌ gRPC receive error: {e.code()} - {e.details()}")
            error_occurred.set()
        except Exception as e:
            print(f"\n❌ Error receiving transcriptions: {e}")
            error_occurred.set()
        finally:
            receive_done.set()
    
    try:
        print(f"\n{'='*60}")
        print(f"Testing gRPC StreamTranscribe (Bidirectional Streaming)")
        print(f"Session ID: {session_id}")
        print(f"{'='*60}")
        
        # Create bidirectional stream
        stream = stub.StreamTranscribe(audio_generator())
        
        # Start receiving transcriptions in background
        receive_thread = threading.Thread(
            target=receive_transcriptions,
            args=(stream,),
            daemon=True
        )
        receive_thread.start()
        
        # Wait for send to complete and receive to finish
        send_done.wait(timeout=duration + 2)
        
        # Wait a bit more for any final transcriptions
        receive_thread.join(timeout=3.0)
        
        if not receive_done.is_set():
            print("\n⚠️  Receive thread did not finish cleanly")
        
        if transcriptions:
            print(f"\n{'='*60}")
            print(f"✅ Received {len(transcriptions)} transcription(s)")
            print(f"{'='*60}")
            for i, trans in enumerate(transcriptions, 1):
                print(f"  {i}. {trans}")
        else:
            print(f"\n⚠️  No transcriptions received")
            print("   This could mean:")
            print("   - No speech was detected by VAD")
            print("   - Speech was too short (< min_speech_duration)")
            print("   - Silence threshold not met")
        
        return transcriptions
            
    except grpc.RpcError as e:
        print(f"\n❌ gRPC error: {e.code()} - {e.details()}")
        return []
    except Exception as e:
        print(f"\n❌ Error: {e}")
        return []
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
        description="Test Speech Service (STT) gRPC server with microphone and VAD"
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
        default=10.0,
        help="Recording duration in seconds (default: 10)"
    )
    parser.add_argument(
        "--cleanup",
        action="store_true",
        help="Cleanup session after test"
    )
    
    args = parser.parse_args()
    
    address = f"{args.host}:{args.port}"
    
    print("\n" + "="*60)
    print("  Speech Service (STT) - gRPC Client Test with VAD")
    print(f"  Server: {address}")
    print("="*60)
    print("\n💡 Note: Transcriptions arrive automatically when VAD detects silence")
    print("   Speak naturally - pause between sentences to see transcriptions appear!")
    
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
        # Test STT with VAD
        transcriptions = record_and_stream(
            stub,
            session_id=args.session,
            duration=args.duration
        )
        
        if transcriptions:
            print(f"\n💡 All transcriptions received:")
            for i, trans in enumerate(transcriptions, 1):
                print(f"   {i}. {trans}")
        
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
