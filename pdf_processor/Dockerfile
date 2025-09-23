FROM python:3.11-slim

WORKDIR /app

# 系统库 (PyMuPDF 需要)
RUN apt-get update && apt-get install -y \
    libglib2.0-0 libsm6 libxrender1 libxext6 curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 安装 uv (单文件二进制)
RUN curl -LsSf https://astral.sh/uv/install.sh | sh
ENV PATH="/root/.local/bin:${PATH}"

# 只复制依赖定义，提前缓存
COPY pyproject.toml ./
# 若已有锁文件，也复制： COPY pyproject.toml uv.lock ./
RUN uv lock && uv sync --frozen --no-dev

# 拷贝项目代码
COPY . .

# 运行 FastAPI 服务（使用 uvicorn）
CMD ["uv", "run", "uvicorn", "app:app", "--host","0.0.0.0","--port","8000"]
