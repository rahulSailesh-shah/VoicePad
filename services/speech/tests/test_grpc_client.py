#!/usr/bin/env python3
"""
Minimal gRPC client for SpeechService (STT only) that logs
latency between last speech audio chunk sent and transcription received.
Uses simple energy-based VAD to detect end of speech.
"""

import time
import threading
import uuid
from typing import Iterator

import grpc
import pyaudio
import numpy as np

from src import speech_pb2
from src import speech_pb2_grpc

# -------------------------------
# Audio config
# -------------------------------
RATE = 16000
CHANNELS = 1
FORMAT = pyaudio.paInt16
CHUNK = 320  # 20ms chunks at 16kHz

# -------------------------------
# VAD config (simple energy-based)
# -------------------------------
RMS_THRESHOLD = 0.02  # Tune this based on your mic/environment (0.01-0.05 typical)
SILENCE_DURATION_SEC = 0.5  # Seconds of silence to detect end of utterance
SILENCE_CHUNKS_THRESHOLD = int(SILENCE_DURATION_SEC * (RATE / CHUNK))

def run_client(server_addr: str):
    session_id = str(uuid.uuid4())
    print(f"\n🎤 Session ID: {session_id}")

    last_speech_ts = {"value": None}
    transcriptions = []

    # -------------------------------
    # Audio generator (send)
    # -------------------------------
    def audio_generator() -> Iterator[speech_pb2.TranscribeRequest]:
        audio = pyaudio.PyAudio()
        stream = audio.open(
            format=FORMAT,
            channels=CHANNELS,
            rate=RATE,
            input=True,
            frames_per_buffer=CHUNK,
        )

        print("🎙️ Speak naturally — will auto-stop after ~0.5s silence")

        silence_chunks = 0
        try:
            while True:
                data = stream.read(CHUNK, exception_on_overflow=False)
                current_ts = time.time()

                # Simple VAD: compute RMS energy
                audio_array = np.frombuffer(data, dtype=np.int16).astype(np.float32) / 32768.0
                rms = np.sqrt(np.mean(audio_array**2))

                if rms > RMS_THRESHOLD:
                    # Speech detected
                    last_speech_ts["value"] = current_ts
                    silence_chunks = 0
                else:
                    # Silence
                    silence_chunks += 1

                yield speech_pb2.TranscribeRequest(
                    session_id=session_id,
                    audio_chunk=data,
                    end_of_stream=False,
                )

                # Check if enough silence to end stream
                if silence_chunks >= SILENCE_CHUNKS_THRESHOLD and last_speech_ts["value"] is not None:
                    print("\n🛑 Silence detected — ending audio stream")
                    break

        except KeyboardInterrupt:
            print("\n🛑 Manual stop (Ctrl+C)")

        finally:
            stream.stop_stream()
            stream.close()
            audio.terminate()

            # Send end-of-stream
            yield speech_pb2.TranscribeRequest(
                session_id=session_id,
                audio_chunk=b"",
                end_of_stream=True,
            )

    # -------------------------------
    # Response receiver
    # -------------------------------
    def receive_responses(stream):
        for resp in stream:
            if not resp.success:
                print(f"❌ Server error: {resp.error}")
                continue

            if resp.transcription:
                recv_ts = time.time()
                sent_ts = last_speech_ts["value"]

                if sent_ts:
                    latency_ms = (recv_ts - sent_ts) * 1000
                    print(f"\n⏱️ Latency: {latency_ms:.1f} ms "
                          f"(end of speech → transcription)")

                text = resp.transcription.strip()
                transcriptions.append(text)
                print(f"\n✅ Transcription:\n{text}\n")

    # -------------------------------
    # gRPC connection
    # -------------------------------
    with grpc.insecure_channel(server_addr) as channel:
        stub = speech_pb2_grpc.SpeechServiceStub(channel)
        response_stream = stub.StreamTranscribe(audio_generator())

        recv_thread = threading.Thread(
            target=receive_responses,
            args=(response_stream,),
            daemon=True,
        )
        recv_thread.start()
        recv_thread.join()

    # -------------------------------
    # Summary
    # -------------------------------
    print("\n==============================")
    print("📊 Session Summary")
    print("==============================")

    if transcriptions:
        print("📝 Final Transcription:")
        print(" ".join(transcriptions))
    else:
        print("⚠️ No transcription received")

    print("==============================\n")


if __name__ == "__main__":
    run_client("localhost:50051")