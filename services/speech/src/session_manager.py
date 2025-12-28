"""Session manager for per-user speech processing isolation (STT only)."""

import threading
import logging
import tempfile
import os
import queue
import time
from dataclasses import dataclass, field
from io import BytesIO
from typing import Callable, Optional

from .config import config, STTConfig

logger = logging.getLogger(__name__)


@dataclass
class SpeechSession:
    session_id: str
    stt_config: STTConfig
    transcription_callback: Optional[Callable[[str], None]] = None
    
    _speech_buffer: BytesIO = field(default_factory=BytesIO)  # Buffer for current speech segment
    _lock: threading.Lock = field(default_factory=threading.Lock)
    
    _vad_model: object | None = None
    _vad_utils: object | None = None
    _whisper_model: object | None = None
    _vad_audio_buffer: list = field(default_factory=list)  # Buffer for VAD processing (512 sample frames)
    _is_speaking: bool = False
    _silence_start_time: float | None = None
    _speech_start_time: float | None = None
    _closed: bool = False
    
    _processing_thread: threading.Thread | None = None
    _stop_event: threading.Event = field(default_factory=threading.Event)
    
    def __post_init__(self):
        self._initialize_models()
        self._start_processing_thread()
    
    def _initialize_models(self):
        """Initialize VAD and Whisper models."""
        try:
            import torch
            self._vad_model, utils = torch.hub.load(
                repo_or_dir='snakers4/silero-vad',
                model='silero_vad',
                force_reload=False,
                onnx=False
            )
            self._vad_model.eval()
            self._vad_utils = utils
            logger.info(f"Initialized Silero VAD for session {self.session_id}")
        except Exception as e:
            logger.error(f"Failed to initialize Silero VAD for session {self.session_id}: {e}", exc_info=True)
            self._vad_model = None
            self._vad_utils = None
        
    def _get_whisper_model(self):
        if self._whisper_model is None:
            try:
                from faster_whisper import WhisperModel
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
    
    def _start_processing_thread(self):
        if self._processing_thread is not None:
            return
        
        self._stop_event.clear()
        self._processing_thread = threading.Thread(
            target=self._process_audio_loop,
            daemon=True,
            name=f"SpeechSession-{self.session_id}"
        )
        self._processing_thread.start()
        logger.debug(f"Started processing thread for session {self.session_id}")
    
    def _process_audio_loop(self):
        while not self._stop_event.is_set():
            try:
                with self._lock:
                    if self._is_speaking and self._silence_start_time is not None:
                        silence_duration = time.time() - self._silence_start_time
                        
                        if silence_duration >= self.stt_config.post_speech_silence_duration:
                            if self._speech_start_time is not None:
                                speech_duration = self._silence_start_time - self._speech_start_time
                                
                                if speech_duration >= self.stt_config.min_speech_duration:
                                    audio_data = self._speech_buffer.getvalue()
                                    if len(audio_data) > 0:
                                        self._transcribe_and_send(audio_data)
                                
                                self._speech_buffer = BytesIO()
                                self._is_speaking = False
                                self._speech_start_time = None
                                self._silence_start_time = None
                            else:
                                pass
                                self._speech_buffer = BytesIO()
                                self._is_speaking = False
                                self._silence_start_time = None
                
                self._stop_event.wait(0.1)
                
            except Exception as e:
                logger.error(f"Error in processing loop for session {self.session_id}: {e}")
                self._stop_event.wait(0.1)
    
    def _transcribe_and_send(self, audio_data: bytes):
        """Transcribe audio data and send via callback."""
        if self._closed:
            return
        
        try:
            model = self._get_whisper_model()
            
            if isinstance(model, FallbackTranscriber):
                transcription = model.transcribe(audio_data)
            else:
                with tempfile.NamedTemporaryFile(suffix='.wav', delete=False) as tmp_file:
                    tmp_path = tmp_file.name
                    self._write_wav(tmp_file, audio_data)
                
                try:
                    segments, info = model.transcribe(
                        tmp_path,
                        language=self.stt_config.language,
                        beam_size=5
                    )
                    transcription = " ".join([seg.text for seg in segments]).strip()
                finally:
                    try:
                        os.remove(tmp_path)
                    except Exception as e:
                        logger.warning(f"Failed to remove temp file {tmp_path}: {e}")
            
            if transcription and self.transcription_callback:
                logger.info(f"Session {self.session_id} transcribed: {transcription[:]}...")
                try:
                    self.transcription_callback(transcription)
                except Exception as e:
                    logger.error(f"Error in transcription callback for session {self.session_id}: {e}")
            elif transcription:
                logger.info(f"Session {self.session_id} transcribed: {transcription[:]}... (no callback)")
                
        except Exception as e:
            logger.error(f"Transcription error for session {self.session_id}: {e}")
    
    def feed_audio(self, audio_chunk: bytes) -> None:
        if self._closed or not audio_chunk:
            return
        
        with self._lock:
            # Fallback: buffer all audio if VAD not available
            if self._vad_model is None:
                self._speech_buffer.write(audio_chunk)
                if not self._is_speaking:
                    self._is_speaking = True
                    self._speech_start_time = time.time()
                self._silence_start_time = None
                return
            
            # Process audio chunk through VAD
            try:
                import torch
                import numpy as np
                self._speech_buffer.write(audio_chunk)
                audio_array = np.frombuffer(audio_chunk, dtype=np.int16).astype(np.float32) / 32768.0
                self._vad_audio_buffer.extend(audio_array.tolist())
                VAD_FRAME_SIZE = 512
                
                while len(self._vad_audio_buffer) >= VAD_FRAME_SIZE:
                    frame_samples = self._vad_audio_buffer[:VAD_FRAME_SIZE]
                    self._vad_audio_buffer = self._vad_audio_buffer[VAD_FRAME_SIZE:]
                    audio_tensor = torch.FloatTensor(frame_samples).unsqueeze(0)
                    with torch.no_grad():
                        speech_prob = self._vad_model(audio_tensor, self.stt_config.sample_rate).item()
                    is_speech = speech_prob >= self.stt_config.silero_sensitivity
                    
                    if is_speech:
                        if not self._is_speaking:
                            self._is_speaking = True
                            self._speech_start_time = time.time()
                            self._silence_start_time = None
                            logger.debug(f"Speech started for session {self.session_id}")
                    else:
                        if self._is_speaking:
                            if self._silence_start_time is None:
                                self._silence_start_time = time.time()
                                logger.debug(f"Silence started for session {self.session_id}")
                        else:
                            self._silence_start_time = None
                
            except Exception as e:
                logger.error(f"VAD processing error for session {self.session_id}: {e}", exc_info=True)
                self._speech_buffer.write(audio_chunk)
                if not self._is_speaking:
                    self._is_speaking = True
                    self._speech_start_time = time.time()
                self._silence_start_time = None
    
    def finalize_transcription(self) -> str:
        with self._lock:
            if self._is_speaking and len(self._speech_buffer.getvalue()) > 0:
                audio_data = self._speech_buffer.getvalue()
                self._speech_buffer = BytesIO()
                self._is_speaking = False
                self._speech_start_time = None
                self._silence_start_time = None
                
                transcription_result = [None]
                
                def callback(text: str):
                    transcription_result[0] = text
                
                old_callback = self.transcription_callback
                self.transcription_callback = callback
                self._transcribe_and_send(audio_data)
                self.transcription_callback = old_callback
                
                return transcription_result[0] or ""
            return ""
    
    def _write_wav(self, file, audio_data: bytes, sample_rate: int = 16000, channels: int = 1, bits_per_sample: int = 16):
        import struct
        
        byte_rate = sample_rate * channels * bits_per_sample // 8
        block_align = channels * bits_per_sample // 8
        data_size = len(audio_data)
        
        file.write(b'RIFF')
        file.write(struct.pack('<I', 36 + data_size))
        file.write(b'WAVE')
        file.write(b'fmt ')
        file.write(struct.pack('<I', 16))
        file.write(struct.pack('<H', 1))
        file.write(struct.pack('<H', channels))
        file.write(struct.pack('<I', sample_rate))
        file.write(struct.pack('<I', byte_rate))
        file.write(struct.pack('<H', block_align))
        file.write(struct.pack('<H', bits_per_sample))
        file.write(b'data')
        file.write(struct.pack('<I', data_size))
        file.write(audio_data)
    
    def cleanup(self) -> None:
        logger.info(f"Cleaning up session: {self.session_id}")
        
        self._closed = True
        
        if self._processing_thread is not None:
            self._stop_event.set()
            self._processing_thread.join(timeout=2.0)
            if self._processing_thread.is_alive():
                logger.warning(f"Processing thread for session {self.session_id} did not stop gracefully")
            self._processing_thread = None
        
        with self._lock:
            if self._is_speaking and len(self._speech_buffer.getvalue()) > 0:
                audio_data = self._speech_buffer.getvalue()
                self._transcribe_and_send(audio_data)
            
            # Clear buffers
            self._speech_buffer = BytesIO()
            self._is_speaking = False
            self._speech_start_time = None
            self._silence_start_time = None
            
            # Release models
            self._vad_model = None
            self._vad_utils = None
            self._vad_audio_buffer = []
            self._whisper_model = None
            self.transcription_callback = None
        
        logger.debug(f"Session {self.session_id} cleaned up")


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
    
    def get_or_create(self, session_id: str, transcription_callback: Optional[Callable[[str], None]] = None) -> SpeechSession:
        """Get existing session or create new one."""
        with self._lock:
            if session_id not in self._sessions:
                self._sessions[session_id] = SpeechSession(
                    session_id=session_id,
                    stt_config=config.stt,
                    transcription_callback=transcription_callback,
                )
            elif transcription_callback:
                # Update callback if provided
                self._sessions[session_id].transcription_callback = transcription_callback
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
                logger.info(f"Session {session_id} removed from manager")
                return True
            return False
    
    def cleanup_all(self) -> None:
        """Cleanup all sessions (for shutdown)."""
        with self._lock:
            session_ids = list(self._sessions.keys())
            for session_id in session_ids:
                self._sessions[session_id].cleanup()
            self._sessions.clear()
        logger.info("All sessions cleaned up")
    
    @property
    def active_session_count(self) -> int:
        """Get count of active sessions."""
        with self._lock:
            return len(self._sessions)


# Global session manager instance
session_manager = SessionManager()
