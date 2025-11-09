
from utils import get_logger
import traceback
from infra.grpc_infra.protos import cognicore_pb2, cognicore_pb2_grpc
from infra.document_infra.embedding import vectorize

logger = get_logger(__name__)


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

        logger.info(f"[Task {task_id}] Received embedding request (text length: {len(text)})")

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

        # Generate embeddings
        try:
            embeddings = vectorize(text)
            dimension = len(embeddings)

            logger.info(f"[Task {task_id}] ✓ Generated embeddings successfully (dimension: {dimension})")

            return cognicore_pb2.EmbeddingResponse(  # type: ignore
                success=True,
                message=f"Embeddings generated successfully",
                embeddings=embeddings,
                dimension=dimension
            )

        except Exception as e:
            error_msg = f"Failed to generate embeddings: {str(e)}"
            logger.error(f"[Task {task_id}] {error_msg}")
            logger.error(f"Traceback:\n{traceback.format_exc()}")
            return cognicore_pb2.EmbeddingResponse(  # type: ignore
                success=False,
                message=error_msg,
                embeddings=[],
                dimension=0
            )
