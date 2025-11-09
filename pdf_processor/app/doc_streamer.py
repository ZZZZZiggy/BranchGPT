# broadcaster/document_streamer.py
from utils import get_logger
import traceback
from infra.grpc_infra.grpc_client import CRPCClient
from service.grpc_ingest_service import IngestService

logger = get_logger(__name__)


async def stream_to_go_service(
    doc_id: str,
    user_id: str,
    pdf_path: str,
    data_generator,
    estimated_chunks: int = 0,
):
    """High-level broadcaster that streams document to Go service."""
    try:
        ingest_service = IngestService()
        metadata = ingest_service.build_metadata(doc_id, user_id, pdf_path, estimated_chunks)
        async with CRPCClient() as stub:
            request_gen = ingest_service.request_stream(doc_id, metadata, data_generator)
            logger.info(f"🚀 Streaming document {doc_id}...")
            response = await stub.IngestDocument(request_gen)

            logger.info(f"✅ Ingested document {doc_id}, success={response.success}")
            return {
                "success": response.success,
                "message": response.message,
                "chunks_received": response.chunks_received,
                "chunks_stored": response.chunks_stored,
                "chunks_failed": response.chunks_failed,
                "processing_time_ms": response.processing_time_ms,
                "file_id": response.file_id,
            }
    except Exception as e:
        logger.error(f"Streaming error for {doc_id}: {e}")
        logger.error(f"Traceback:\n{traceback.format_exc()}")
        return {"success": False, "message": str(e)}
