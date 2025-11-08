import config
from pathlib import Path
from asyncio.log import logger

async def download_from_bucket(download_url: str, local_path: Path):
    """Download a file from the given URL to the specified local path."""
    try:
        if config.STORAGE_TYPE == "s3":
            import aioboto3
            session = aioboto3.Session()
            async with session.client(
                "s3",
                endpoint_url=config.BUCKET_ENDPOINT,
                aws_access_key_id=config.BUCKET_ACCESS_ID,
                aws_secret_access_key=config.BUCKET_ACCESS_KEY,
            ) as s3_client:  # type: ignore
                bucket_name = config.BUCKET_NAME
                object_key = download_url.replace(
                    f"{config.BUCKET_ENDPOINT}/{bucket_name}/", ""
                )
                await s3_client.download_file(bucket_name, object_key, str(local_path))


        elif config.STORAGE_TYPE == "minio":
            import aiohttp, aiofiles

            async with aiohttp.ClientSession() as session:
                async with session.get(download_url) as resp:
                    resp.raise_for_status()
                    async with aiofiles.open(local_path, "wb") as f:
                        async for chunk in resp.content.iter_chunked(1024):
                            await f.write(chunk)
    except Exception as e:
        logger.error(f"Error downloading file from {download_url}: {e}")
        raise RuntimeError(f"Failed to download file from {download_url}: {e}")
