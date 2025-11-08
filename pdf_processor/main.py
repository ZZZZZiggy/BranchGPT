"""
Main entry point for the PDF Processor service.
Starts all components:
- gRPC API Key server (port 50052)
- gRPC Embedding server (port 50053)
- Redis worker (processes PDF tasks)
"""
from tasks.redis_worker import redis_main_loop
from tasks.grpc_server import start_grpc_server
from tasks.grpc_embedding import start_embedding_grpc_server
from tasks.grpc_api_key import set_main_event_loop
import asyncio
import signal
import sys
from asyncio.log import logger


async def main():
    """Main async entry point that runs all services together."""
    logger.info("=" * 60)
    logger.info("🚀 Starting PDF Processor Service")
    logger.info("=" * 60)

    # IMPORTANT: Initialize event loop reference FIRST
    # This must be done before any gRPC requests can arrive
    loop = asyncio.get_running_loop()
    set_main_event_loop(loop)
    logger.info("✓ Main event loop initialized")

    # Start gRPC servers (they run in background threads)
    logger.info("Starting gRPC servers...")
    api_key_server = start_grpc_server()           # Port 50052: API Key reception
    embedding_server = start_embedding_grpc_server()  # Port 50053: Embedding service

    logger.info("✓ All gRPC servers started")
    logger.info("  - API Key server: port 50052")
    logger.info("  - Embedding server: port 50053")
    logger.info("=" * 60)

    # Graceful shutdown handler
    def shutdown_handler():
        logger.info("\n🛑 Shutting down services...")
        api_key_server.stop(grace=5)
        embedding_server.stop(grace=5)
        logger.info("✓ gRPC servers stopped")
        sys.exit(0)

    # Register signal handlers (use the same loop we saved earlier)
    for sig in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(sig, shutdown_handler)

    try:
        # Start Redis worker (main event loop)
        logger.info("Starting Redis worker...")
        await redis_main_loop()
    except KeyboardInterrupt:
        logger.info("\n🛑 Received shutdown signal")
    finally:
        shutdown_handler()


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        logger.info("✓ Shutdown complete")
        sys.exit(0)
