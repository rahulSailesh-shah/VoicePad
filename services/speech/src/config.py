"""Configuration for the Speech Service (STT only)."""

import os
from dataclasses import dataclass
from dotenv import load_dotenv

load_dotenv()


@dataclass
class STTConfig:
    """Speech-to-Text configuration."""
    
    model: str = "base"
    language: str | None = "en"
    silero_sensitivity: float = 0.5  # VAD threshold (0.0-1.0)
    post_speech_silence_duration: float = 0.5  # Seconds of silence before finalizing
    min_speech_duration: float = 0.2  # Minimum speech duration to consider valid (seconds)
    sample_rate: int = 16000  # Audio sample rate (Hz)


@dataclass
class ServerConfig:
    """gRPC server configuration."""
    
    host: str = "0.0.0.0"
    port: int = 50051
    max_workers: int = 10


@dataclass
class AppConfig:
    """Application configuration."""
    
    stt: STTConfig
    server: ServerConfig
    
    @classmethod
    def from_env(cls) -> "AppConfig":
        """Load configuration from environment variables."""
        return cls(
            stt=STTConfig(
                model=os.getenv("STT_MODEL", "base"),
                language=os.getenv("STT_LANGUAGE", "en") or None,
                silero_sensitivity=float(os.getenv("STT_SILERO_SENSITIVITY", "0.5")),
                post_speech_silence_duration=float(os.getenv("STT_SILENCE_DURATION", "0.8")),
                min_speech_duration=float(os.getenv("STT_MIN_SPEECH_DURATION", "0.3")),
                sample_rate=int(os.getenv("STT_SAMPLE_RATE", "16000")),
            ),
            server=ServerConfig(
                host=os.getenv("GRPC_HOST", "0.0.0.0"),
                port=int(os.getenv("GRPC_PORT", "50051")),
                max_workers=int(os.getenv("GRPC_MAX_WORKERS", "10")),
            ),
        )


# Global config instance
config = AppConfig.from_env()
