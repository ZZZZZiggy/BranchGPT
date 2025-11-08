import asyncio
from asyncio.log import logger
import cognicore_pb2
import cognicore_pb2_grpc
from collections import defaultdict

_api_key_store = {}
_api_key_events = defaultdict(asyncio.Event)

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
            loop = asyncio.get_event_loop()
            if loop.is_running():
                loop.call_soon_threadsafe(event.set)

        return cognicore_pb2.APIKeyResponse( # type: ignore
            success=True,
            message=f"API Key received for provider {provider}"
        )

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
