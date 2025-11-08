# Multi-stage build for PDF Processor Worker
FROM python:3.11-slim

WORKDIR /app

# 系统库 (PyMuPDF 需要)
RUN apt-get update && apt-get install -y \
    libglib2.0-0 libsm6 libxrender1 libxext6 curl ca-certificates \
    libmupdf-dev \
    && rm -rf /var/lib/apt/lists/*

# 安装 uv (单文件二进制)
RUN curl -LsSf https://astral.sh/uv/install.sh | sh
ENV PATH="/root/.local/bin:${PATH}"

# 只复制依赖定义，提前缓存
COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev

# 拷贝项目代码
COPY . .

# 创建临时目录
RUN mkdir -p /tmp/pdf_processor

# 环境变量
ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1

# Healthcheck
HEALTHCHECK --interval=30s --timeout=5s --start-period=30s --retries=3 \
    CMD python -c "import redis; redis.from_url('${REDIS_URL}').ping()" || exit 1

# 默认启动 Redis Worker (包含 gRPC server)
# K8s 部署时可以通过 command 覆盖
CMD ["uv", "run", "python", "-m", "tasks.redis_worker"]
