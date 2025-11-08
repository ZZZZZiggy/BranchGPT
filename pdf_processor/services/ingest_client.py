from asyncio.log import logger
import hashlib
from pathlib import Path
import grpc
from typing import Generator, Dict, Any, List
from protos import cognicore_pb2, cognicore_pb2_grpc
import config

async def stream_to_go_service(
    doc_id: str,
    user_id: str,
    pdf_path: str,
    data_generator: Generator[Dict[str, Any], None, None],
    estimated_chunks: int = 0,
) -> Dict[str, Any]:
    """Stream processed data to the Go service."""
    try:
        logger.info(f"Streaming document {doc_id} to Go service at {config.GO_GRPC_INGEST_ADDR}...")

        # 1 create gRPC channel
        async with grpc.aio.insecure_channel(config.GO_GRPC_INGEST_ADDR) as channel:
            stub = cognicore_pb2_grpc.IngestServiceStub(channel)
            logger.info(f"gRPC channel established.")

            # 2 create request generator
            async def request_generator():
                """Generate requests for streaming document chunks."""
                logger.info(f"Preparing metadata for document {doc_id}...")

                file_path = Path(pdf_path)
                file_size = file_path.stat().st_size if file_path.exists() else 0
                file_hash = ""
                if file_path.exists():
                    try:
                        with open(file_path, "rb") as f:
                            file_hash = hashlib.sha256(f.read()).hexdigest()
                    except Exception as e:
                        logger.warning(f"Failed to compute hash for {pdf_path}: {e}")

                metadata = cognicore_pb2.DocumentMetadata(  # type: ignore
                    file_id=doc_id,
                    user_id=user_id,
                    filename=file_path.name,
                    total_pages=0,
                    estimated_chunks=estimated_chunks,
                    file_hash=file_hash,
                    file_size=file_size,
                    created_at="",
                )
                yield cognicore_pb2.IngestRequest(metadata=metadata) # type: ignore
                logger.info(f"Metadata prepared and sent for document {doc_id}.")

                # 3 stream chunks
                chunk_idx = 0
                logger.info(f"Streaming chunks for document {doc_id}...")
                for chunk_data in data_generator:
                    try:
                        embeddings = chunk_data.get("embeddings", [])
                        # 🔍 DEBUG: Check embeddings before sending
                        if not embeddings:
                            logger.error(f"❌ Chunk {chunk_idx}: embeddings is EMPTY! chunk_data keys: {chunk_data.keys()}")
                        else:
                            logger.debug(f"✓ Chunk {chunk_idx}: embeddings length = {len(embeddings)}")

                        text_chunk = cognicore_pb2.TextChunk(  # type: ignore
                            chunk_id=chunk_data.get("chunk_id", ""),
                            file_id=doc_id,
                            chapter=chunk_data.get("chapter", ""),
                            chapter_num=chunk_data.get("chapter_num", ""),
                            chunk_text=chunk_data.get("content", ""),
                            embedding_vector=embeddings,
                            chunk_index=chunk_idx,
                        )
                        yield cognicore_pb2.IngestRequest(chunk=text_chunk) # type: ignore

                        chunk_idx += 1

                        if chunk_idx % 10 == 0:
                            logger.info(f"Processed {chunk_idx} chunks for document {doc_id}.")


                    except Exception as e:
                        logger.error(f"Error yielding chunk {chunk_idx} for document {doc_id}: {e}")
                logger.info(f"All chunks streamed for document {doc_id}. Total chunks: {chunk_idx}.")

            # 4 call IngestDocument
            logger.info(f"Starting to stream document {doc_id} to Go service...")
            response = await stub.IngestDocument(request_generator())

            logger.info(f"Document {doc_id} ingested successfully. Response: {response}")

            res = {
                "success": response.success,
                "message": response.message,
                "chunks_received": response.chunks_received,
                "chunks_stored": response.chunks_stored,
                "chunks_failed": response.chunks_failed,
                "processing_time_ms": response.processing_time_ms,
                "file_id": response.file_id,
                "failed_rate": (response.chunks_failed / response.chunks_received) if response.chunks_received > 0 else 0.0
            }
            # wrong file_id check (response is protobuf object, not dict)
            if response.file_id != doc_id:
                logger.warning(
                    f"File ID mismatch! Sent: {doc_id}, "
                    f"Received: {response.file_id}"
                )
                return {
                    **res,
                    "success": False,
                    "file_id_mismatch": True,
                    "sent_file_id": doc_id,
                    "received_file_id": response.file_id,
                    "message": f"File ID mismatch: sent {doc_id}, received {response.file_id}",
                }
            if response.success:
                logger.info(f" upload succeeded for document {doc_id}.")
                logger.info(f"  - Received: {response.chunks_received} chunks")
                logger.info(f"  - Stored: {response.chunks_stored} chunks")
                logger.info(f"  - Failed: {response.chunks_failed} chunks")
                logger.info(f"  - Processing time: {response.processing_time_ms}ms")
            else:
                logger.error(f"✗ Document {doc_id} upload failed")
                logger.error(f"  - Received: {response.chunks_received}")
                logger.error(f"  - Stored: {response.chunks_stored}")
                logger.error(f"  - Failed: {response.chunks_failed} chunks")
                logger.error(f"  - Message: {response.message}")

            return res

    except Exception as e:
        logger.error(f"Error streaming document {doc_id}: {e}")
        return {
            "success": False,
            "chunks_failed": -1,
            "failed_rate": 1.0,
            "message": str(e),
        }
