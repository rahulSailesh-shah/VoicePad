"""Session manager for per-user speech processing isolation (STT only)."""

import threading
import logging
import tempfile
import os
from dataclasses import dataclass, field
from io import BytesIO

from .config import config, STTConfig

logger = logging.getLogger(__name__)


@dataclass
class SpeechSession:
    """Represents a single user's speech processing session (STT only)."""
    
    session_id: str
    stt_config: STTConfig
    
    # Audio buffer for accumulating chunks before transcription
    _audio_buffer: BytesIO = field(default_factory=BytesIO)
    _lock: threading.Lock = field(default_factory=threading.Lock)
    _transcription: str = ""
    _is_recording: bool = False
    
    # Lazy-loaded Whisper model
    _whisper_model: object | None = None
    
    def __post_init__(self):
        logger.info(f"Created speech session: {self.session_id}")
    
    def _get_whisper_model(self):
        """Lazy initialize the Whisper model."""
        if self._whisper_model is None:
            try:
                from faster_whisper import WhisperModel
                
                # Use faster-whisper for transcription
                self._whisper_model = WhisperModel(
                    self.stt_config.model,
                    device="cpu",
                    compute_type="int8"
                )
                logger.info(f"Initialized Whisper model ({self.stt_config.model}) for session {self.session_id}")
            except ImportError:
                logger.warning("faster-whisper not available, using fallback")
                self._whisper_model = FallbackTranscriber()
        return self._whisper_model
    
    def feed_audio(self, audio_chunk: bytes) -> None:
        """Feed audio chunk to the session buffer."""
        with self._lock:
            self._audio_buffer.write(audio_chunk)
            self._is_recording = True
    
    def finalize_transcription(self) -> str:
        """
        Finalize the transcription and return the result.
        This is called when the user mutes or signals end of speech.
        """
        with self._lock:
            if not self._is_recording:
                return ""
            
            # Get accumulated audio
            audio_data = self._audio_buffer.getvalue()
            self._audio_buffer = BytesIO()  # Reset buffer
            self._is_recording = False
            
            if len(audio_data) == 0:
                return ""
            
            try:
                model = self._get_whisper_model()
                
                if isinstance(model, FallbackTranscriber):
                    self._transcription = model.transcribe(audio_data)
                else:
                    # Save audio to temp file for faster-whisper
                    with tempfile.NamedTemporaryFile(suffix='.wav', delete=False) as tmp_file:
                        tmp_path = tmp_file.name
                        # Write WAV header + audio data
                        self._write_wav(tmp_file, audio_data)
                    
                    try:
                        segments, info = model.transcribe(
                            tmp_path,
                            language=self.stt_config.language,
                            beam_size=5
                        )
                        self._transcription = " ".join([seg.text for seg in segments]).strip()
                    finally:
                        os.remove(tmp_path)
                
                logger.info(f"Session {self.session_id} transcribed: {self._transcription[:50] if self._transcription else '(empty)'}...")
                return self._transcription
                
            except Exception as e:
                logger.error(f"Transcription error for session {self.session_id}: {e}")
                return ""
    
    def _write_wav(self, file, audio_data: bytes, sample_rate: int = 16000, channels: int = 1, bits_per_sample: int = 16):
        """Write raw PCM audio data as a WAV file."""
        import struct
        
        byte_rate = sample_rate * channels * bits_per_sample // 8
        block_align = channels * bits_per_sample // 8
        data_size = len(audio_data)
        
        # WAV header
        file.write(b'RIFF')
        file.write(struct.pack('<I', 36 + data_size))  # File size - 8
        file.write(b'WAVE')
        file.write(b'fmt ')
        file.write(struct.pack('<I', 16))  # Subchunk1Size (16 for PCM)
        file.write(struct.pack('<H', 1))   # AudioFormat (1 = PCM)
        file.write(struct.pack('<H', channels))
        file.write(struct.pack('<I', sample_rate))
        file.write(struct.pack('<I', byte_rate))
        file.write(struct.pack('<H', block_align))
        file.write(struct.pack('<H', bits_per_sample))
        file.write(b'data')
        file.write(struct.pack('<I', data_size))
        file.write(audio_data)
    
    def cleanup(self) -> None:
        """Clean up session resources."""
        logger.info(f"Cleaning up session: {self.session_id}")
        
        with self._lock:
            self._audio_buffer = BytesIO()
            self._transcription = ""
            self._is_recording = False
            self._whisper_model = None  # Release model memory


class FallbackTranscriber:
    """Fallback transcriber when faster-whisper is not available."""
    
    def transcribe(self, audio_data: bytes) -> str:
        logger.warning("Using fallback transcription (returns placeholder)")
        return "[Transcription unavailable - faster-whisper not installed]"


class SessionManager:
    """
    Manages speech sessions for multiple users (STT only).
    Each session is isolated by session_id (format: "boardId:participantId").
    """
    
    def __init__(self):
        self._sessions: dict[str, SpeechSession] = {}
        self._lock = threading.Lock()
        logger.info("SessionManager initialized")
    
    def get_or_create(self, session_id: str) -> SpeechSession:
        """Get existing session or create new one."""
        with self._lock:
            if session_id not in self._sessions:
                self._sessions[session_id] = SpeechSession(
                    session_id=session_id,
                    stt_config=config.stt,
                )
            return self._sessions[session_id]
    
    def get(self, session_id: str) -> SpeechSession | None:
        """Get session by ID, returns None if not found."""
        with self._lock:
            return self._sessions.get(session_id)
    
    def cleanup(self, session_id: str) -> bool:
        """Cleanup and remove a session."""
        with self._lock:
            if session_id in self._sessions:
                self._sessions[session_id].cleanup()
                del self._sessions[session_id]
                return True
            return False
    
    def cleanup_all(self) -> None:
        """Cleanup all sessions (for shutdown)."""
        with self._lock:
            for session in self._sessions.values():
                session.cleanup()
            self._sessions.clear()
        logger.info("All sessions cleaned up")
    
    @property
    def active_session_count(self) -> int:
        """Get count of active sessions."""
        with self._lock:
            return len(self._sessions)


# Global session manager instance
session_manager = SessionManager()
