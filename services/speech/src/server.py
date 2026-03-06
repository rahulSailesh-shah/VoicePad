"""gRPC server implementation for Speech Service (STT, utterance-level)."""

import logging
import signal
import sys
import threading
from concurrent import futures
from queue import Queue

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
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
)
logger = logging.getLogger(__name__)


class SpeechServicer(speech_pb2_grpc.SpeechServiceServicer):

    def StreamTranscribe(self, request_iterator, context):
        """
        Utterance-level STT stream.
        Emits exactly one transcription per detected utterance.
        Safe for STT → LLM → TTS pipelines.
        """

        session = None
        session_id = None

        # Queue carries ONLY completed utterances (strings)
        utterance_queue: Queue[str | None] = Queue()

        def transcription_callback(text: str):
            """
            Called by SpeechSession ONLY when an utterance is complete
            (post-VAD silence).
            """
            if context.is_active():
                utterance_queue.put(text)

        # ----------------------------------------
        # Audio ingestion thread (producer)
        # ----------------------------------------
        def audio_reader():
            nonlocal session, session_id
            try:
                for request in request_iterator:
                    if not context.is_active():
                        break

                    if not request.session_id:
                        logger.warning("Request missing session_id")
                        continue

                    if session is None:
                        session_id = request.session_id
                        session = session_manager.get_or_create(
                            session_id=session_id,
                            transcription_callback=transcription_callback,
                        )
                        logger.info(f"Session started: {session_id}")

                    if request.audio_chunk:
                        session.feed_audio(request.audio_chunk)

                    if request.end_of_stream:
                        logger.info(f"End of stream received: {session_id}")
                        break

            except Exception as e:
                logger.error(
                    f"Audio reader error for {session_id}: {e}",
                    exc_info=True,
                )
            finally:
                # Signal completion to response loop
                utterance_queue.put(None)

        reader_thread = threading.Thread(
            target=audio_reader,
            daemon=True,
            name=f"AudioReader-{id(self)}",
        )
        reader_thread.start()

        # ----------------------------------------
        # Response loop (consumer)
        # ----------------------------------------
        try:
            while context.is_active():
                item = utterance_queue.get()

                if item is None:
                    break

                # Emit exactly ONE utterance per response
                yield speech_pb2.TranscribeResponse(
                    transcription=item,
                    success=True,
                )

            # ----------------------------------------
            # Final flush (single, safe)
            # ----------------------------------------
            if session:
                final_text = session.finalize_transcription()
                if final_text:
                    yield speech_pb2.TranscribeResponse(
                        transcription=final_text,
                        success=True,
                    )

        except Exception as e:
            logger.error(
                f"StreamTranscribe error for {session_id}: {e}",
                exc_info=True,
            )
            yield speech_pb2.TranscribeResponse(
                success=False,
                error=str(e),
            )

        finally:
            if session_id:
                session_manager.cleanup(session_id)
                logger.info(f"Session cleaned up: {session_id}")

            logger.debug(f"StreamTranscribe completed: {session_id}")

    def CleanupSession(self, request, context):
        if not request.session_id:
            return speech_pb2.CleanupResponse(success=False)

        success = session_manager.cleanup(request.session_id)
        logger.info(
            f"Manual cleanup {'successful' if success else 'failed'}: "
            f"{request.session_id}"
        )

        return speech_pb2.CleanupResponse(success=success)


def serve():
    if speech_pb2_grpc is None:
        logger.error("Proto files not generated")
        sys.exit(1)

    server = grpc.server(
        futures.ThreadPoolExecutor(
            max_workers=config.server.max_workers
        )
    )

    speech_pb2_grpc.add_SpeechServiceServicer_to_server(
        SpeechServicer(),
        server,
    )

    address = f"{config.server.host}:{config.server.port}"
    server.add_insecure_port(address)

    def shutdown_handler(signum, frame):
        logger.info("Shutdown signal received")
        server.stop(grace=5)
        sys.exit(0)

    signal.signal(signal.SIGINT, shutdown_handler)
    signal.signal(signal.SIGTERM, shutdown_handler)

    server.start()
    logger.info(f"Speech Service (STT) started on {address}")
    logger.info("Mode: Utterance-level (LLM-safe)")

    server.wait_for_termination()


if __name__ == "__main__":
    serve()
