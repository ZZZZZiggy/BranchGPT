import os
from dotenv import load_dotenv

load_dotenv()

REDIS_URL = os.getenv("REDIS_URL")
REDIS_PASSWORD = os.getenv("REDIS_PASSWORD")
REDIS_QUEUE_NAME = os.getenv("REDIS_QUEUE_NAME", "queue:upload_tasks")
REDIS_RETRY_QUEUE_NAME = os.getenv("REDIS_RETRY_QUEUE_NAME", "queue:upload_tasks_retry")

BUCKET_ENDPOINT = os.getenv("BUCKET_ENDPOINT")
BUCKET_ACCESS_ID = os.getenv("BUCKET_ACCESS_ID")
BUCKET_ACCESS_KEY = os.getenv("BUCKET_ACCESS_KEY")
BUCKET_NAME = os.getenv("BUCKET_NAME")
STORAGE_TYPE = os.getenv("STORAGE_TYPE")

EMBEDDING_MODEL_NAME = os.getenv("EMBEDDING_MODEL_NAME", "paraphrase-multilingual-MiniLM-L12-v2")
EMBEDDING_DIMENSION = int(os.getenv("EMBEDDING_DIMENSION", "384"))

GO_GRPC_INGEST_ADDR = os.getenv("GO_GRPC_INGEST_ADDR", "localhost:50051")  # Python → Go (数据注入)
GRPC_SERVER_PORT = int(os.getenv("GRPC_SERVER_PORT", "50052"))  # Go → Python (API Key 接收)
GRPC_EMBEDDING_PORT = int(os.getenv("GRPC_EMBEDDING_PORT", "50053"))  # Go → Python (文本向量化)

LOG_LEVEL = os.getenv("LOG_LEVEL", "INFO")
LOG_FILE = os.getenv("LOG_FILE", "logs/app.log")
