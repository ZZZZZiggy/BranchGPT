import grpc
import config
from infra.grpc_infra.protos import cognicore_pb2, cognicore_pb2_grpc
from utils import get_logger

logger = get_logger(__name__)

class CRPCClient:
    def __init__(self, target: str = ""):
        self.target = target or config.GO_GRPC_INGEST_ADDR
        self.channel = None
        self.stub = None

    async def __aenter__(self):
        logger.info(f" Connecting to Go gRPC service at {self.target}...")
        self.channel = grpc.aio.insecure_channel(self.target)
        self.stub = cognicore_pb2_grpc.IngestServiceStub(self.channel)
        return self.stub
    async def __aexit__(self, exc_type, exc, tb):
        if self.channel:
            await self.channel.close()
            logger.info(" gRPC channel closed.")
