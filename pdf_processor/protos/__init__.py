"""
gRPC Protocol Buffer definitions for CogniCore service.

This package contains auto-generated code from cognicore.proto:
- cognicore_pb2.py: Message definitions
- cognicore_pb2_grpc.py: Service definitions
"""

# Make cognicore_pb2 importable from this package
from . import cognicore_pb2
from . import cognicore_pb2_grpc

__all__ = ['cognicore_pb2', 'cognicore_pb2_grpc']
