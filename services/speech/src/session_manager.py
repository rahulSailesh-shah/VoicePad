"""Optimized session manager."""

import threading
import logging
import time
import numpy as np
import torch
from collections import deque
from dataclasses import dataclass, field
from typing import Callable, Optional
from faster_whisper import WhisperModel

logger = logging.getLogger(__name__)

# Shared resources (loaded once)
_SHARED_WHISPER_MODEL = None
_SHARED_VAD_MODEL = None
_MODEL_LOCK = threading.Lock()


def get_shared_models():
    """Get or create shared Whisper and VAD models."""
    global _SHARED_WHISPER_MODEL, _SHARED_VAD_MODEL
    
    with _MODEL_LOCK:
        if _SHARED_WHISPER_MODEL is None:
            _SHARED_WHISPER_MODEL = WhisperModel(
                "base",
                device="cpu",
                compute_type="int8",
                cpu_threads=8,
                num_workers=1
            )
            logger.info("Loaded shared Whisper model")
        
        if _SHARED_VAD_MODEL is None:
            _SHARED_VAD_MODEL, _ = torch.hub.load(
                repo_or_dir='snakers4/silero-vad',
                model='silero_vad',
                force_reload=False,
                onnx=False
            )
            _SHARED_VAD_MODEL.eval()
            logger.info("Loaded shared VAD model")
        
        return _SHARED_WHISPER_MODEL, _SHARED_VAD_MODEL


@dataclass
class SpeechSession:
    session_id: str
    sample_rate: int = 16000
    silence_threshold: float = 0.5  # seconds
    min_speech_duration: float = 0.2  # seconds
    vad_sensitivity: float = 0.5
    transcription_callback: Optional[Callable[[str], None]] = None
    
    # Audio buffers (use list of chunks, not BytesIO)
    _speech_chunks: list[bytes] = field(default_factory=list)
    _vad_buffer: deque = field(default_factory=lambda: deque(maxlen=8000))  # 0.5s at 16kHz
    
    # State
    _is_speaking: bool = False
    _speech_start_time: float = 0.0
    _silence_start_time: float = 0.0
    
    # Threading
    _lock: threading.Lock = field(default_factory=threading.Lock)
    _closed: bool = False
    
    # Shared models (references, not copies)
    _whisper_model: WhisperModel = field(init=False)
    _vad_model: torch.nn.Module = field(init=False)
    
    def __post_init__(self):
        self._whisper_model, self._vad_model = get_shared_models()
    
    def feed_audio(self, audio_chunk: bytes) -> None:
        """Feed audio chunk for VAD and buffering."""
        if self._closed or not audio_chunk:
            return
        
        # Convert to float32 outside lock
        audio_array = np.frombuffer(audio_chunk, dtype=np.int16).astype(np.float32) / 32768.0
        
        with self._lock:
            # Always buffer audio
            self._speech_chunks.append(audio_chunk)
            
            # Process through VAD
            self._vad_buffer.extend(audio_array)
            
            # Process in 512-sample frames
            while len(self._vad_buffer) >= 512:
                # Get frame (efficient with deque)
                frame = np.array(list(self._vad_buffer)[:512])
                
                # Remove processed samples
                for _ in range(512):
                    self._vad_buffer.popleft()
                
                # VAD detection
                with torch.no_grad():
                    speech_prob = self._vad_model(
                        torch.FloatTensor(frame).unsqueeze(0),
                        self.sample_rate
                    ).item()
                
                is_speech = speech_prob >= self.vad_sensitivity
                current_time = time.time()
                
                if is_speech:
                    if not self._is_speaking:
                        # Speech started
                        self._is_speaking = True
                        self._speech_start_time = current_time
                        self._silence_start_time = 0.0
                        logger.debug(f"[{self.session_id}] Speech started")
                else:
                    if self._is_speaking:
                        # Potential silence
                        if self._silence_start_time == 0.0:
                            self._silence_start_time = current_time
                        
                        silence_duration = current_time - self._silence_start_time
                        
                        # End of utterance?
                        if silence_duration >= self.silence_threshold:
                            speech_duration = self._silence_start_time - self._speech_start_time
                            
                            if speech_duration >= self.min_speech_duration:
                                # Transcribe in background
                                audio_to_transcribe = b''.join(self._speech_chunks)
                                threading.Thread(
                                    target=self._transcribe_async,
                                    args=(audio_to_transcribe,),
                                    daemon=True
                                ).start()
                            
                            # Reset
                            self._speech_chunks.clear()
                            self._is_speaking = False
                            self._speech_start_time = 0.0
                            self._silence_start_time = 0.0
                            logger.debug(f"[{self.session_id}] Utterance complete")
    
    def _transcribe_async(self, audio_data: bytes):
        """Transcribe audio in background thread."""
        try:
            # Convert to numpy array
            audio_array = np.frombuffer(audio_data, dtype=np.int16).astype(np.float32) / 32768.0
            
            # Transcribe (no temp file!)
            segments, info = self._whisper_model.transcribe(
                audio_array,
                language="en",
                beam_size=1,
                best_of=1,
                temperature=0.0,
                without_timestamps=True
            )
            
            text = " ".join([seg.text for seg in segments]).strip()
            
            if text and self.transcription_callback:
                logger.info(f"[{self.session_id}] Transcribed: {text}")
                self.transcription_callback(text)
        
        except Exception as e:
            logger.error(f"[{self.session_id}] Transcription error: {e}", exc_info=True)
    

    def finalize_transcription(self) -> str:
        """Finalize transcription and return final text."""
        with self._lock:
            # Transcribe any remaining buffered audio
            if self._speech_chunks and self._is_speaking:
                audio_to_transcribe = b''.join(self._speech_chunks)
                try:
                    audio_array = np.frombuffer(audio_to_transcribe, dtype=np.int16).astype(np.float32) / 32768.0
                    segments, info = self._whisper_model.transcribe(
                        audio_array,
                        language="en",
                        beam_size=1,
                        best_of=1,
                        temperature=0.0,
                        without_timestamps=True
                    )
                    text = " ".join([seg.text for seg in segments]).strip()
                    # Reset session
                    self._speech_chunks.clear()
                    self._vad_buffer.clear()
                    self._is_speaking = False
                    self._speech_start_time = 0.0
                    self._silence_start_time = 0.0
            
                    return text
                except Exception as e:
                    logger.error(f"[{self.session_id}] Final transcription error: {e}")
            
            

    def cleanup(self):
        """Cleanup session resources."""
        with self._lock:
            self._closed = True
            self._speech_chunks.clear()
            self._vad_buffer.clear()
            # Don't delete shared models
        
        logger.info(f"[{self.session_id}] Cleaned up")


class SessionManager:
    def __init__(self):
        self._sessions: dict[str, SpeechSession] = {}
        self._lock = threading.Lock()
    
    def get_or_create(
        self,
        session_id: str,
        transcription_callback: Optional[Callable[[str], None]] = None
    ) -> SpeechSession:
        with self._lock:
            if session_id not in self._sessions:
                self._sessions[session_id] = SpeechSession(
                    session_id=session_id,
                    transcription_callback=transcription_callback
                )
            return self._sessions[session_id]
    
    def cleanup(self, session_id: str) -> bool:
        with self._lock:
            if session_id in self._sessions:
                self._sessions[session_id].cleanup()
                del self._sessions[session_id]
                return True
            return False
    
    @property
    def active_session_count(self) -> int:
        with self._lock:
            return len(self._sessions)


session_manager = SessionManager()