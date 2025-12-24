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
    silero_sensitivity: float = 0.6
    post_speech_silence_duration: float = 0.4
    min_length_of_recording: float = 0.5
    enable_realtime_updates: bool = False


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
                silero_sensitivity=float(os.getenv("STT_SILERO_SENSITIVITY", "0.6")),
                post_speech_silence_duration=float(os.getenv("STT_SILENCE_DURATION", "0.4")),
            ),
            server=ServerConfig(
                host=os.getenv("GRPC_HOST", "0.0.0.0"),
                port=int(os.getenv("GRPC_PORT", "50051")),
                max_workers=int(os.getenv("GRPC_MAX_WORKERS", "10")),
            ),
        )


# Global config instance
config = AppConfig.from_env()
