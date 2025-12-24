#!/usr/bin/env python3
"""
Microphone test script for Speech Service (STT only).

This script tests STT functionality in isolation by:
1. Recording audio from the microphone
2. Transcribing it using RealtimeSTT

Usage:
    python -m tests.test_microphone [--model MODEL]
    
Options:
    --model MODEL   Whisper model to use (tiny, base, small, medium, large-v2)
    --duration SEC  Max recording duration in seconds (default: 10)
"""

import argparse
import sys
import time
import threading
from pathlib import Path

# Add parent directory to path for imports
sys.path.insert(0, str(Path(__file__).parent.parent))


def test_realtime_stt(model: str = "base", duration: float = 10.0) -> str:
    """
    Test RealtimeSTT with microphone input.
    
    Uses the library's built-in microphone handling and VAD.
    """
    print(f"\n{'='*60}")
    print("Testing RealtimeSTT with Microphone")
    print(f"Model: {model}")
    print(f"Max duration: {duration}s")
    print(f"{'='*60}\n")
    
    try:
        from RealtimeSTT import AudioToTextRecorder
    except ImportError:
        print("ERROR: RealtimeSTT not installed.")
        print("Install with: pip install RealtimeSTT")
        return ""
    
    transcription_result = []
    recording_done = threading.Event()
    
    def on_transcription(text: str):
        """Called when transcription is complete."""
        transcription_result.append(text)
        print(f"\n[Final Transcription]: {text}")
        recording_done.set()
    
    def on_realtime_update(text: str):
        """Called for real-time transcription updates."""
        print(f"\r[Live]: {text}", end="", flush=True)
    
    print("Initializing recorder (this may take a moment for model download)...")
    
    recorder = AudioToTextRecorder(
        model=model,
        language="en",
        silero_sensitivity=0.6,
        post_speech_silence_duration=0.6,  # Wait 0.6s of silence before finalizing
        min_length_of_recording=0.5,
        spinner=False,
        on_realtime_transcription_update=on_realtime_update,
    )
    
    print("\n🎤 Speak now! (Recording will stop after silence or timeout)")
    print("   Press Ctrl+C to stop early\n")
    
    try:
        # Start recording and wait for completion
        start_time = time.time()
        
        # The text() method blocks until speech is detected and finalized
        text = recorder.text()
        
        if text:
            print(f"\n\n✅ Transcription complete!")
            print(f"   Result: {text}")
            print(f"   Duration: {time.time() - start_time:.1f}s")
            return text
        else:
            print("\n\n⚠️  No speech detected")
            return ""
            
    except KeyboardInterrupt:
        print("\n\n⏹️  Recording stopped by user")
        return ""
    finally:
        recorder.shutdown()


def test_manual_audio_feed():
    """
    Test the session manager with manually fed audio chunks.
    This simulates how the gRPC server would receive audio.
    """
    print(f"\n{'='*60}")
    print("Testing Session Manager with Manual Audio Feed")
    print(f"{'='*60}\n")
    
    try:
        import pyaudio
    except ImportError:
        print("ERROR: pyaudio not installed")
        return
    
    from src.session_manager import session_manager
    
    # Audio parameters matching LiveKit
    SAMPLE_RATE = 16000
    CHANNELS = 1
    CHUNK_SIZE = 1024
    FORMAT = pyaudio.paInt16
    RECORD_SECONDS = 5
    
    session_id = "test:user1"
    session = session_manager.get_or_create(session_id)
    
    print(f"Session created: {session_id}")
    print(f"Recording for {RECORD_SECONDS} seconds...")
    print("🎤 Speak now!\n")
    
    audio = pyaudio.PyAudio()
    
    try:
        stream = audio.open(
            format=FORMAT,
            channels=CHANNELS,
            rate=SAMPLE_RATE,
            input=True,
            frames_per_buffer=CHUNK_SIZE
        )
        
        chunks_fed = 0
        for _ in range(0, int(SAMPLE_RATE / CHUNK_SIZE * RECORD_SECONDS)):
            data = stream.read(CHUNK_SIZE, exception_on_overflow=False)
            session.feed_audio(data)
            chunks_fed += 1
            print(f"\r  Chunks fed: {chunks_fed}", end="", flush=True)
        
        print(f"\n\nFinalizing transcription...")
        transcription = session.finalize_transcription()
        
        if transcription:
            print(f"✅ Transcription: {transcription}")
        else:
            print("⚠️  No transcription returned")
        
    finally:
        stream.stop_stream()
        stream.close()
        audio.terminate()
        session_manager.cleanup(session_id)


def main():
    parser = argparse.ArgumentParser(
        description="Test Speech Service (STT) with microphone input"
    )
    parser.add_argument(
        "--model",
        type=str,
        default="base",
        choices=["tiny", "base", "small", "medium", "large-v2"],
        help="Whisper model to use (default: base)"
    )
    parser.add_argument(
        "--duration",
        type=float,
        default=10.0,
        help="Max recording duration in seconds (default: 10)"
    )
    parser.add_argument(
        "--test-session",
        action="store_true",
        help="Test the session manager with manual audio feed"
    )
    
    args = parser.parse_args()
    
    print("\n" + "="*60)
    print("  Speech Service - STT Microphone Test")
    print("="*60)
    
    if args.test_session:
        test_manual_audio_feed()
    else:
        # Test STT
        test_realtime_stt(
            model=args.model,
            duration=args.duration
        )
    
    print("\n" + "="*60)
    print("  Test Complete")
    print("="*60 + "\n")


if __name__ == "__main__":
    main()
