import grpc
import sys
from concurrent import futures
from utils import get_logger
from infra.grpc_infra.protos import cognicore_pb2_grpc
from infra.grpc_infra.grpc_server import EmbeddingServicer

logger = get_logger(__name__)

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
