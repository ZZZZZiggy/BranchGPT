"""
gRPC Embedding Service
Go service sends text via gRPC, Python vectorizes it and returns embeddings.

Usage:
    python -m tasks.grpc_embedding
"""
import asyncio
import signal
import sys
from asyncio.log import logger
import config
import grpc
from concurrent import futures
from protos import cognicore_pb2, cognicore_pb2_grpc
from etl.processing import vectorize


class EmbeddingServicer(cognicore_pb2_grpc.EmbeddingServiceServicer):  # type: ignore
    """gRPC server to vectorize text for Go service."""

    def GetEmbedding(self, request, context):
        """Receive text from Go service and return embeddings.

        Args:
            request: EmbeddingRequest with task_id, text, api_key, provider
            context: gRPC context

        Returns:
            EmbeddingResponse with success status and embeddings
        """
        task_id = request.task_id
        text = request.text
        api_key = request.api_key
        provider = request.provider

        logger.info(f"[Task {task_id}] Received embedding request (provider: {provider}, text length: {len(text)})")

        # Validate inputs
        if not text or not text.strip():
            error_msg = "Text cannot be empty"
            logger.error(f"[Task {task_id}] {error_msg}")
            return cognicore_pb2.EmbeddingResponse(  # type: ignore
                success=False,
                message=error_msg,
                embeddings=[],
                dimension=0
            )

        if not api_key:
            error_msg = "API key is required"
            logger.error(f"[Task {task_id}] {error_msg}")
            return cognicore_pb2.EmbeddingResponse(  # type: ignore
                success=False,
                message=error_msg,
                embeddings=[],
                dimension=0
            )

        if provider not in ["openai", "gemini"]:
            error_msg = f"Unsupported provider: {provider}. Supported: 'openai', 'gemini'"
            logger.error(f"[Task {task_id}] {error_msg}")
            return cognicore_pb2.EmbeddingResponse(  # type: ignore
                success=False,
                message=error_msg,
                embeddings=[],
                dimension=0
            )

        # Generate embeddings
        try:
            embeddings = vectorize(text, api_key, provider)
            dimension = len(embeddings)

            logger.info(f"[Task {task_id}] ✓ Generated embeddings successfully (dimension: {dimension})")

            return cognicore_pb2.EmbeddingResponse(  # type: ignore
                success=True,
                message=f"Embeddings generated successfully using {provider}",
                embeddings=embeddings,
                dimension=dimension
            )

        except Exception as e:
            error_msg = f"Failed to generate embeddings: {str(e)}"
            logger.error(f"[Task {task_id}] {error_msg}")
            return cognicore_pb2.EmbeddingResponse(  # type: ignore
                success=False,
                message=error_msg,
                embeddings=[],
                dimension=0
            )


def start_embedding_grpc_server(port: int = 50053):
    """Start gRPC server to handle embedding requests from Go service.

    Args:
        port: Port to listen on (default: 50053, different from API key port 50052)

    Returns:
        grpc.Server: The started gRPC server instance
    """
    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=20),  # More workers for concurrent embedding requests
        options=[
            ('grpc.max_send_message_length', 50 * 1024 * 1024),  # 50MB
            ('grpc.max_receive_message_length', 50 * 1024 * 1024),  # 50MB
        ]
    )

    cognicore_pb2_grpc.add_EmbeddingServiceServicer_to_server(  # type: ignore
        EmbeddingServicer(), server
    )

    server.add_insecure_port(f'[::]:{port}')
    server.start()

    logger.info(f"🚀 gRPC Embedding server started on port {port}")
    logger.info(f"   Ready to receive embedding requests from Go service...")

    return server


def signal_handler(sig, frame):
    """Handle Ctrl+C gracefully"""
    logger.info("\n🛑 Shutting down gRPC Embedding server...")
    sys.exit(0)
