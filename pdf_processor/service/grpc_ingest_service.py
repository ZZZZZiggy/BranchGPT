from pathlib import Path
import hashlib
import os
import pymupdf
from utils import get_logger
from infra.grpc_infra.protos import cognicore_pb2

logger = get_logger(__name__)
from typing import Generator

class IngestService:
    "base class for gRPC Ingest Service"
    @staticmethod
    def build_metadata(doc_id: str, user_id: str, pdf_path: str, estimated_chunks: int):
        file_path = Path(pdf_path)
        file_size = file_path.stat().st_size if file_path.exists() else 0
        file_hash = ""

        if file_path.exists():
            try:
                with open(file_path, "rb") as f:
                    file_hash = hashlib.sha256(f.read()).hexdigest()
            except Exception as e:
                logger.warning(f"Failed to compute hash: {e}")

        return cognicore_pb2.DocumentMetadata( # type: ignore
            file_id=doc_id,
            user_id=user_id,
            filename=file_path.name,
            total_pages=0,
            estimated_chunks=estimated_chunks,
            file_hash=file_hash,
            file_size=file_size,
            created_at="",
        )

    @staticmethod
    async def request_stream(
        doc_id : str,
        metadata: cognicore_pb2.DocumentMetadata, # type: ignore
        data_generator: Generator[dict, None, None],
    ):
        yield cognicore_pb2.IngestRequest(metadata=metadata)  # type: ignore
        logger.info(f"Metadata prepared and sent for document {doc_id}.")

        for idx, chunk_data in enumerate(data_generator):
            try:
                embeddings = chunk_data.get("embeddings", [])
                if not embeddings:
                    logger.warning(f"Empty embeddings for chunk {idx+1}, skipping...")

                text_chunk = cognicore_pb2.TextChunk( # type: ignore
                    chunk_id=chunk_data.get("chunk_id", ""),
                    file_id=doc_id,
                    chapter=chunk_data.get("chapter", ""),
                    chapter_num=chunk_data.get("chapter_num", ""),
                    chunk_text=chunk_data.get("content", ""),
                    embedding_vector=embeddings,
                    chunk_index=idx,
                )
                yield cognicore_pb2.IngestRequest(chunk=text_chunk) # type: ignore
                if idx % 10 == 0:
                    logger.info(f"Streamed {idx} chunks...")
            except Exception as e:
                logger.error(f"Error streaming chunk {idx+1}: {e}")
                continue
