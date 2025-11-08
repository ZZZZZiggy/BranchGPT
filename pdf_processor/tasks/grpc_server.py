"""
Standalone gRPC API Key Server
This script can be run independently to start only the gRPC server
without the Redis worker.

Usage:
    python -m tasks.grpc_server
"""
import asyncio
import signal
import sys
from asyncio.log import logger
import config
import grpc
from concurrent import futures
from protos import cognicore_pb2_grpc
from tasks.grpc_api_key import APIKeyServer

def signal_handler(sig, frame):
    """Handle Ctrl+C gracefully"""
    logger.info("\n🛑 Shutting down gRPC server...")
    sys.exit(0)

def start_grpc_server():
    """Start gRPC server to receive API Keys from Go service.

    Returns:
        grpc.Server: The started gRPC server instance
    """
    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=10),
        options=[
            ('grpc.max_send_message_length', 50 * 1024 * 1024),  # 50MB
            ('grpc.max_receive_message_length', 50 * 1024 * 1024),  # 50MB
        ]
    )

    cognicore_pb2_grpc.add_APIKeyServiceServicer_to_server( # type: ignore
        APIKeyServer(), server
    )

    port = getattr(config, 'GRPC_SERVER_PORT', 50052)
    server.add_insecure_port(f'[::]:{port}')
    server.start()

    logger.info(f"🚀 gRPC API Key server started on port {port}")
    logger.info(f"   Listening for API Key requests from Go service...")

    return server
