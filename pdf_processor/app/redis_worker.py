from utils import get_logger
import config
import traceback

logger = get_logger(__name__)
import traceback
import redis.asyncio as redis
import json
import asyncio
from typing import Optional, Tuple
from pathlib import Path
from datetime import datetime
from infra.bucket_infra.file_downloader import download_from_bucket
from infra.document_infra.processing import process_and_vectorize
from app.doc_streamer import stream_to_go_service


async def process_data(task_data: dict, redis_client: redis.Redis, rag_mode: bool) -> dict:
    """Process the task data received from Redis.
    This function downloads the file, processes it, and streams the results to the Go service.
    Implements automatic retry on failure with exponential backoff.
    """
    doc_id = task_data.get("DocID")
    retry_count = task_data.get("retry_count", 0)
    max_retries = 3

    try:
        logger.info(f"Processing document {doc_id} with local embedding model...")

        file_name = task_data.get("FileName")
        user_id = task_data.get("UserID")
        download_url = task_data.get("URL")
        file_size = task_data.get("FileSize")
        created_at = task_data.get("CreatedAt")

        logger.info(f" Processing document {doc_id}...")
        logger.info(f"  - DocID: {doc_id}")
        logger.info(f"  - FileName: {file_name}")
        logger.info(f"  - UserID: {user_id}")
        logger.info(f"  - FileSize: {file_size} bytes" if file_size else "  - FileSize: N/A")
        logger.info(f"  - CreatedAt: {created_at}")
        logger.info(f"  - URL: {download_url[:80]}..." if download_url else "  - URL: None")

        # Validate required fields
        if not doc_id:
            logger.error(f" Missing DocID in task data. Skipping.")
            return {"success": False, "error": "Missing DocID"}

        if not download_url:
            logger.error(f" No download URL provided for document {doc_id}. Skipping.")
            return {"success": False, "error": "Missing URL"}

        if not file_name:
            logger.warning(f"Missing FileName for document {doc_id}, using DocID as filename")
            file_name = doc_id

        if not user_id:
            logger.warning(f"Missing UserID for document {doc_id}, using default '0'")
            user_id = "0"

        # Step 1: Download the file
        logger.info(f"Downloading file...")
        logger.debug(f"Full URL: {download_url}")

        temp_dir = Path(getattr(config, 'TEMP_DIR', "/tmp/pdf_processor"))
        temp_dir.mkdir(parents=True, exist_ok=True)

        # ensure file_name ends with .pdf
        if not file_name.endswith('.pdf'):
            file_name = f"{file_name}.pdf"

        local_file_path = temp_dir / f"{doc_id}_{file_name}"

        try:
            await download_from_bucket(download_url, local_file_path)  # type: ignore
            logger.info(f" File downloaded to {local_file_path}")
        except Exception as e:
            error_msg = f"Failed to download file from {download_url}"
            logger.error(f" Download failed: {e}")
            logger.error(f"Traceback:\n{traceback.format_exc()}")
            raise Exception(f"{error_msg}: {e}")

        # Step 2: Process and vectorize the document
        logger.info(f" - processing and vectorizing document {doc_id}...")
        data_generator, estimated_chunks = process_and_vectorize(
            str(local_file_path),
            file_id=doc_id,
            rag_mode=rag_mode
        ) # type: ignore

        logger.info(f" - document {doc_id} processed and vectorized.")

        # Step 3: Stream the processed data to the Go service
        logger.info(f" - streaming processed data to Go service for document {doc_id}...")
        res = await stream_to_go_service(
            doc_id=doc_id,
            user_id=user_id,
            pdf_path=str(local_file_path),
            data_generator=data_generator,
            estimated_chunks=estimated_chunks,
        ) # type: ignore
        logger.info(f" - streaming to Go service completed for document {doc_id}.")

        # Clean up temporary files
        if res.get("success"): # type: ignore
            try:
                local_file_path.unlink()
                logger.info(f"  - Temporary file deleted")
            except Exception as e:
                logger.warning(f"  - Failed to delete temporary file: {e}")
            return res

        # Failed - retry entire document
        logger.warning(
            f"✗ Document {doc_id} upload failed "
            f"(retry {retry_count}/{max_retries})"
        )
        logger.warning(f"  - Error: {res.get('message', 'Unknown error')}")

        # Check if we should retry
        if retry_count >= max_retries:
            logger.error(
                f"Document {doc_id} failed after {max_retries} retries, "
                f"moving to DLQ"
            )
            await move_to_dlq(task_data, res, redis_client)
            if local_file_path.exists():
                local_file_path.unlink()
            return res

        # Retry with exponential backoff
        delay = 2 ** retry_count
        logger.info(
            f"Will retry document {doc_id} in {delay}s "
            f"(attempt {retry_count + 1}/{max_retries})"
        )

        await asyncio.sleep(delay)

        # Re-queue for retry
        retry_task = {
            **task_data,
            "retry_count": retry_count + 1,
            "last_error": res.get("message", "Unknown error"),
            "last_attempt_time": datetime.now().isoformat(),
        }

        await redis_client.rpush(
            config.REDIS_QUEUE_NAME,
            json.dumps(retry_task)
        ) # type: ignore

        logger.info(f"Document {doc_id} re-queued for full retry")

        # Keep the file for potential debugging (will be re-downloaded on retry anyway)
        if local_file_path.exists():
            local_file_path.unlink()

        return res

    except Exception as e:
        logger.error(f"Exception processing document {doc_id}: {e}")
        logger.error(f"Traceback:\n{traceback.format_exc()}")

        # Exception - also consider retry
        if retry_count < max_retries:
            delay = 2 ** retry_count
            logger.info(f"Retrying after exception in {delay}s (attempt {retry_count + 1}/{max_retries})")

            await asyncio.sleep(delay)

            retry_task = {
                **task_data,
                "retry_count": retry_count + 1,
                "last_error": str(e),
                "last_attempt_time": datetime.now().isoformat(),
            }

            await redis_client.rpush(
                config.REDIS_QUEUE_NAME,
                json.dumps(retry_task)
            ) # type: ignore

            logger.info(f"Document {doc_id} re-queued after exception")
        else:
            logger.error(f"Document {doc_id} failed with exception after {max_retries} retries")
            await move_to_dlq(
                task_data,
                {"error": str(e), "success": False},
                redis_client
            )

        # Clean up on final failure
        if retry_count >= max_retries:
            local_file_path = Path(getattr(config, 'TEMP_DIR', "/tmp/pdf_processor")) / f"{doc_id}_{task_data.get('FileName', doc_id)}.pdf"
            if local_file_path.exists():
                local_file_path.unlink()

        raise


async def move_to_dlq(task_data: dict, result: dict, redis_client: redis.Redis):
    """Move failed task to Dead Letter Queue for manual inspection."""
    dlq_data = {
        **task_data,
        "final_result": result,
        "failed_at": datetime.now().isoformat(),
        "retry_count": task_data.get("retry_count", 0),
    }

    dlq_queue = getattr(config, 'REDIS_DLQ_NAME', 'queue:dlq')
    await redis_client.rpush(dlq_queue, json.dumps(dlq_data)) # type: ignore

    logger.error(
        f"Task moved to DLQ: {task_data.get('DocID')} "
        f"(retries: {task_data.get('retry_count', 0)})"
    )

async def redis_main_loop():
    """Main loop for processing Redis messages.
    Continuously listens for new tasks from the Redis queue and processes them.
    Implements automatic retry with exponential backoff and DLQ for failed tasks.
    """
    redis_client = redis.from_url(config.REDIS_URL, password=config.REDIS_PASSWORD)
    logger.info(f" start listening to Redis queue")

    while True:
        try:
            res: Optional[Tuple[bytes, bytes]] = await redis_client.blpop(config.REDIS_QUEUE_NAME, timeout=0) # type: ignore

            if res:
                queue_name, task_json = res
                logger.info(f"Received task from {queue_name.decode()}: {task_json.decode()}")
                # queue_name = "pdf_queue"
                # task_json = '{"task_id": "doc_123", "s3_url": "..."}'
                task_data = json.loads(task_json.decode())
                doc_id = task_data.get('DocID', 'unknown')
                retry_count = task_data.get('retry_count', 0)

                logger.info(
                    f" Processing document {doc_id} "
                    f"(retry: {retry_count})"
                )
                rag_mode = task_data.get("RagMode", False)
                # Process with retry support
                await process_data(task_data, redis_client, rag_mode)

        except json.JSONDecodeError as e:
            logger.error(f" Error decoding JSON: {e}")
            logger.error(f"Traceback:\n{traceback.format_exc()}")
        except Exception as e:
            logger.error(f" Unexpected error in main loop: {e}")
            logger.error(f"Traceback:\n{traceback.format_exc()}")

if __name__ == '__main__':
    asyncio.run(redis_main_loop())
