"""
Bucket infrastructure - Cloud storage operations.

Handles file downloads from S3-compatible storage (S3, MinIO, etc.)
"""

from infra.bucket_infra.file_downloader import download_from_bucket

__all__ = ['download_from_bucket']
