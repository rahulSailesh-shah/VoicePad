"""gRPC server implementation for Speech Service (STT only)."""

import logging
import signal
import sys
import threading
from concurrent import futures
from queue import Queue, Empty

import grpc

from .config import config
from .session_manager import session_manager

try:
    from . import speech_pb2
    from . import speech_pb2_grpc
except ImportError:
    speech_pb2 = None
    speech_pb2_grpc = None

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class SpeechServicer:
    
    def StreamTranscribe(self, request_iterator, context):
        session_id = None
        session = None
        transcription_queue = Queue()
        stream_active = threading.Event()
        stream_active.set()
        
        def transcription_callback(transcription: str):
            if stream_active.is_set() and transcription:
                try:
                    transcription_queue.put(transcription)
                except Exception as e:
                    logger.error(f"Error queuing transcription for {session_id}: {e}")
        
        try:
            def process_requests():
                nonlocal session_id, session
                try:
                    for request in request_iterator:
                        session_id = request.session_id
                        
                        if not session_id:
                            logger.error("Received request without session_id")
                            continue
                        
                        session = session_manager.get_or_create(
                            session_id,
                            transcription_callback=transcription_callback
                        )
                        
                        if request.audio_chunk:
                            session.feed_audio(request.audio_chunk)
                        if request.end_of_stream:
                            logger.info(f"End of stream received for {session_id}")
                            if session:
                                final_transcription = session.finalize_transcription()
                                if final_transcription:
                                    transcription_callback(final_transcription)
                            break
                    logger.debug(f"Request stream ended for {session_id}")
                    
                except Exception as e:
                    logger.error(f"Error processing requests for {session_id}: {e}", exc_info=True)
                finally:
                    stream_active.clear()
                    transcription_queue.put(None)
            
            request_thread = threading.Thread(
                target=process_requests,
                daemon=True,
                name=f"RequestProcessor-{session_id or 'unknown'}"
            )
            request_thread.start()
            
            while True:
                try:
                    try:
                        transcription = transcription_queue.get(timeout=1.0)
                    except Empty:
                        if not request_thread.is_alive():
                            if session:
                                final_transcription = session.finalize_transcription()
                                if final_transcription:
                                    yield speech_pb2.TranscribeResponse(
                                        transcription=final_transcription,
                                        success=True
                                    )
                            break
                        continue
                    
                    if transcription is None:
                        break
                    
                    yield speech_pb2.TranscribeResponse(
                        transcription=transcription,
                        success=True
                    )
                    
                except Exception as e:
                    logger.error(f"Error sending transcription for {session_id}: {e}")
                    yield speech_pb2.TranscribeResponse(
                        success=False,
                        error=str(e)
                    )
                    break
            
            request_thread.join(timeout=2.0)
            
        except Exception as e:
            logger.error(f"StreamTranscribe error for {session_id}: {e}", exc_info=True)
            yield speech_pb2.TranscribeResponse(
                success=False,
                error=str(e)
            )
        finally:
            stream_active.clear()
            logger.debug(f"StreamTranscribe completed for {session_id}")
    
    def CleanupSession(self, request, context):
        session_id = request.session_id
        
        if not session_id:
            return speech_pb2.CleanupResponse(success=False)
        
        success = session_manager.cleanup(session_id)
        logger.info(f"Session cleanup {'successful' if success else 'failed'}: {session_id}")
        
        return speech_pb2.CleanupResponse(success=success)


def serve():
    if speech_pb2_grpc is None:
        logger.error("Proto files not generated. Run: python -m grpc_tools.protoc ...")
        sys.exit(1)
    
    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=config.server.max_workers)
    )
    
    speech_pb2_grpc.add_SpeechServiceServicer_to_server(
        SpeechServicer(), server
    )
    
    address = f"{config.server.host}:{config.server.port}"
    server.add_insecure_port(address)
    
    def shutdown_handler(signum, frame):
        logger.info("Received shutdown signal, cleaning up...")
        session_manager.cleanup_all()
        server.stop(grace=5)
        sys.exit(0)
    
    signal.signal(signal.SIGINT, shutdown_handler)
    signal.signal(signal.SIGTERM, shutdown_handler)
    
    server.start()
    logger.info(f"Speech Service (STT) started on {address}")
    logger.info(f"STT Model: {config.stt.model}")
    logger.info(f"VAD Sensitivity: {config.stt.silero_sensitivity}")
    logger.info(f"Silence Duration: {config.stt.post_speech_silence_duration}s")
    
    server.wait_for_termination()


if __name__ == "__main__":
    serve()
