"""gRPC server implementation for Speech Service (STT only)."""

import logging
import signal
import sys
from concurrent import futures

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
    """Implementation of SpeechService gRPC service (STT only)."""
    
    def StreamTranscribe(self, request_iterator, context):
        """
        Handle streaming transcription requests.
        
        Client sends audio chunks with session_id.
        When end_of_stream=True, finalize and return transcription.
        """
        session_id = None
        
        try:
            for request in request_iterator:
                session_id = request.session_id
                
                if not session_id:
                    return speech_pb2.TranscribeResponse(
                        success=False,
                        error="session_id is required"
                    )
                
                session = session_manager.get_or_create(session_id)
                
                # Feed audio chunk to session
                if request.audio_chunk:
                    session.feed_audio(request.audio_chunk)
                
                # Check for end of stream
                if request.end_of_stream:
                    transcription = session.finalize_transcription()
                    logger.info(f"Transcription complete for {session_id}: {transcription[:100]}...")
                    
                    return speech_pb2.TranscribeResponse(
                        transcription=transcription,
                        success=True
                    )
            
            # If we reach here, stream ended without end_of_stream flag
            if session_id:
                session = session_manager.get(session_id)
                if session:
                    transcription = session.finalize_transcription()
                    return speech_pb2.TranscribeResponse(
                        transcription=transcription,
                        success=True
                    )
            
            return speech_pb2.TranscribeResponse(
                success=False,
                error="Stream ended without finalization"
            )
            
        except Exception as e:
            logger.error(f"StreamTranscribe error: {e}")
            return speech_pb2.TranscribeResponse(
                success=False,
                error=str(e)
            )
    
    def CleanupSession(self, request, context):
        """Clean up resources for a session."""
        session_id = request.session_id
        
        if not session_id:
            return speech_pb2.CleanupResponse(success=False)
        
        success = session_manager.cleanup(session_id)
        logger.info(f"Session cleanup {'successful' if success else 'failed'}: {session_id}")
        
        return speech_pb2.CleanupResponse(success=success)


def serve():
    """Start the gRPC server."""
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
    
    server.wait_for_termination()


if __name__ == "__main__":
    serve()
