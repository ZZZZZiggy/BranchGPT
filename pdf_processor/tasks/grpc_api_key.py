import asyncio
from asyncio.log import logger
import cognicore_pb2
import cognicore_pb2_grpc
from collections import defaultdict
from typing import Optional

_api_key_store = {}
_api_key_events = defaultdict(asyncio.Event)
_main_event_loop: Optional[asyncio.AbstractEventLoop] = None

class APIKeyServer(cognicore_pb2_grpc.APIKeyServiceServicer): # type: ignore
    """gRPC server to receive API Keys from Go service."""

    def ProvideAPIKey(self, request, context):
        """Receive API Key from Go service (synchronous method for gRPC)."""
        task_id = request.task_id
        api_key = request.api_key
        provider = request.provider

        logger.info(f"Received API Key for task {task_id} (provider: {provider})")

        # Store as tuple (api_key, provider)
        _api_key_store[task_id] = (api_key, provider)

        # Set the event to notify waiting coroutine
        if task_id in _api_key_events:
            event = _api_key_events[task_id]
            # Schedule event.set() in the event loop
            # ⚠️ Cannot use asyncio.get_event_loop() in worker thread!
            # Use the saved main event loop reference instead
            if _main_event_loop and _main_event_loop.is_running():
                _main_event_loop.call_soon_threadsafe(event.set)

        return cognicore_pb2.APIKeyResponse( # type: ignore
            success=True,
            message=f"API Key received for provider {provider}"
        )

def set_main_event_loop(loop: asyncio.AbstractEventLoop):
    """Set the main event loop reference for cross-thread communication.

    This should be called once when the main asyncio loop starts,
    before any gRPC requests are received.

    Args:
        loop: The main asyncio event loop
    """
    global _main_event_loop
    _main_event_loop = loop
    logger.info("Main event loop reference saved for gRPC communication")


async def wait_for_api_key(task_id: str, timeout: int = 30) -> tuple[str, str]:
    """Wait for go service to provide API Key via gRPC.

    Args:
        task_id: Task ID to wait for
        timeout: Timeout in seconds

    Returns:
        Tuple of (api_key, provider)

    Raises:
        Exception: If timeout or API key not received
    """

    # Validate that main event loop has been initialized
    if _main_event_loop is None:
        error_msg = "Main event loop not initialized! Call set_main_event_loop() first."
        logger.error(error_msg)
        raise RuntimeError(error_msg)

    # If already received, return immediately
    if task_id in _api_key_store:
        api_key_data = _api_key_store.pop(task_id)
        logger.info(f"API Key already available for task {task_id}")
        return api_key_data

    # Create event for this task if not exists
    event = _api_key_events[task_id]

    logger.debug(f"Waiting for API Key for task {task_id} (timeout: {timeout}s)")

    try:
        await asyncio.wait_for(event.wait(), timeout=timeout)
        api_key_data = _api_key_store.pop(task_id, None)

        if api_key_data:
            logger.debug(f"API Key received for task {task_id}")
            return api_key_data
        else:
            error_msg = f"API Key not found in store for task {task_id}"
            logger.error(error_msg)
            raise Exception(error_msg)

    except asyncio.TimeoutError:
        error_msg = f"Timeout waiting for API Key for task {task_id} after {timeout}s"
        logger.error(error_msg)
        raise Exception(error_msg)

    finally:
        # Clean up event
        if task_id in _api_key_events:
            del _api_key_events[task_id]
