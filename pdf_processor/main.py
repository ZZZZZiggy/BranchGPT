"""
Main entry point for the PDF Processor service.
Starts all components:
- gRPC Embedding server (port 50053)
- Redis worker (processes PDF tasks)
"""
from app.redis_worker import redis_main_loop
from service.grpc_embedding_service import start_embedding_grpc_server
import asyncio
import signal
import sys
from utils import get_logger

logger = get_logger(__name__)


async def main():
    """Main async entry point that runs all services together."""
    logger.info("=" * 60)
    logger.info("🚀 Starting PDF Processor Service")
    logger.info("=" * 60)

    # 预热 embedding 模型（避免首次请求时冷启动）
    logger.info("🔥 Warming up embedding model...")
    try:
        from infra.document_infra.embedding import get_local_embedding_model
        model = get_local_embedding_model()
        # 运行一次测试推理确保模型完全加载
        _ = model.encode("warmup test", show_progress_bar=False)
        logger.info("✓ Embedding model warmed up and ready")
    except Exception as e:
        logger.warning(f"⚠️  Model warmup failed (will load on first use): {e}")

    # Start gRPC servers (they run in background threads)
    logger.info("Starting gRPC servers...")
    embedding_server = start_embedding_grpc_server()  # Port 50053: Embedding service

    logger.info("✓ gRPC Embedding server started on port 50053")
    logger.info("=" * 60)

    # Graceful shutdown handler
    def shutdown_handler():
        logger.info("\n🛑 Shutting down services...")
        embedding_server.stop(grace=5)
        logger.info("✓ gRPC server stopped")
        sys.exit(0)

    # Register signal handlers
    loop = asyncio.get_running_loop()
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
