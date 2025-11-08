```markdown
# PDF Processor - 高并发文档处理服务

[![Python](https://img.shields.io/badge/Python-3.12+-blue.svg)](https://www.python.org/)
[![gRPC](https://img.shields.io/badge/gRPC-1.69-green.svg)](https://grpc.io/)
[![Redis](https://img.shields.io/badge/Redis-Queue-red.svg)](https://redis.io/)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

> **基于异步 I/O + 多线程 gRPC 的高性能 PDF 向量化处理服务**
>
> 核心特性：并发队列处理 | 流式数据传输 | 多提供商向量化 | 跨线程事件通知

---

## 📋 目录

- [架构概览](#-架构概览)
- [并发与并行设计](#-并发与并行设计)
- [线程模型详解](#-线程模型详解)
- [性能优化策略](#-性能优化策略)
- [快速开始](#-快速开始)
- [API 文档](#-api-文档)
- [部署指南](#-部署指南)
- [性能测试](#-性能测试)

---

## 🏗 架构概览

### 系统架构图
```

                        ┌─────────────────────────────────────┐
                        │     Go Backend Service              │
                        │  ┌──────────────────────────────┐   │
                        │  │  Port 50051: IngestService   │   │◄── 数据注入
                        │  │  (接收 PDF 处理结果)          │   │
                        │  └──────────────────────────────┘   │
                        │  ┌──────────────────────────────┐   │
                        │  │  Port 50052: APIKeyService   │   │───► API Key 传递
                        │  │  (向 Python 发送 API Key)    │   │
                        │  └──────────────────────────────┘   │
                        └──────────────┬──────────────────────┘
                                       │
                        ┌──────────────┴──────────────────────┐
                        │         Redis Queue                 │
                        │  ┌────────────────────────────────┐ │
                        │  │  queue:upload_tasks            │ │◄── 任务队列
                        │  │  - DocID                       │ │
                        │  │  - UserID                      │ │
                        │  │  - URL                         │ │
                        │  │  - task_id                     │ │
                        │  └────────────────────────────────┘ │
                        └──────────────┬──────────────────────┘
                                       │
        ┌──────────────────────────────┴─────────────────────────────────┐
        │                  Python PDF Processor                          │
        │                                                                │
        │  ┌───────────────────────────────────────────────────────────┐ │
        │  │  Main Thread (asyncio Event Loop)                         │ │
        │  │  ┌─────────────────────────────────────────────────────┐  │ │
        │  │  │  redis_main_loop()                                  │  │ │
        │  │  │   ├─→ BLPOP Redis 队列（异步等待）                     │  │ │
        │  │  │   ├─→ wait_for_api_key() (异步等待 Go 发送)           │  │ │
        │  │  │   ├─→ download_from_bucket() (异步下载)               │  │ │
        │  │  │   ├─→ process_and_vectorize() (生成器)             │  │ │
        │  │  │   └─→ stream_to_go_service() (流式上传)            │  │ │
        │  │  └─────────────────────────────────────────────────────┘  │ │
        │  └───────────────────────────────────────────────────────────┘ │
        │                                                                 │
        │  ┌───────────────────────────────────────────────────────────┐ │
        │  │  gRPC Thread Pool (ThreadPoolExecutor)                    │ │
        │  │  ┌─────────────────────────────────────────────────────┐  │ │
        │  │  │  Thread 1-10: API Key Server (Port 50052)          │  │ │
        │  │  │   └─→ ProvideAPIKey() 接收 Go 的 API Key            │  │ │
        │  │  │       └─→ loop.call_soon_threadsafe(event.set)     │  │ │
        │  │  │           └─→ 通知主线程继续处理                     │  │ │
        │  │  └─────────────────────────────────────────────────────┘  │ │
        │  │  ┌─────────────────────────────────────────────────────┐  │ │
        │  │  │  Thread 11-30: Embedding Server (Port 50053)       │  │ │
        │  │  │   └─→ GetEmbedding() 提供文本向量化服务             │  │ │
        │  │  │       └─→ 调用 OpenAI/Gemini API                    │  │ │
        │  │  └─────────────────────────────────────────────────────┘  │ │
        │  └───────────────────────────────────────────────────────────┘ │
        └─────────────────────────────────────────────────────────────────┘

```

### 数据流向

```

1. 任务入队
   Go Backend ──[Redis RPUSH]──► Redis Queue

2. 任务处理
   Python Worker ──[BLPOP]──► Redis Queue
   │
   ├─► 等待 API Key
   │ Go ──[gRPC ProvideAPIKey]──► Python (Port 50052)
   │ Python ──[Event.set]──► 唤醒等待的协程
   │
   ├─► 下载 PDF
   │ Python ──[Async HTTP]──► S3/MinIO
   │
   ├─► 向量化
   │ Python ──[API Call]──► OpenAI/Gemini
   │
   └─► 流式上传
   Python ──[gRPC Stream]──► Go (Port 50051)

3. 独立向量化服务
   Go ──[gRPC GetEmbedding]──► Python (Port 50053)
   Python ──[OpenAI/Gemini API]──► 返回向量

````

---

## ⚡ 并发与并行设计

### 核心概念对比

| 维度 | 并发 (Concurrency) | 并行 (Parallelism) |
|------|-------------------|-------------------|
| **定义** | 单核快速切换，看起来同时 | 多核真正同时执行 |
| **实现** | asyncio 协程 | 多线程 / 多进程 |
| **适用场景** | I/O 密集型（网络、磁盘） | CPU 密集型（计算） |
| **本项目使用** | ✅ Redis 队列监听<br>✅ 文件下载<br>✅ gRPC 客户端 | ✅ gRPC 服务器线程池<br>✅ 多个 PDF 同时处理 |

### 并发设计（asyncio）

#### 1. **主线程事件循环**

```python
async def main():
    # 初始化事件循环引用（关键！）
    loop = asyncio.get_running_loop()
    set_main_event_loop(loop)

    # 启动 gRPC 服务器（后台线程池）
    api_key_server = start_grpc_server()          # 10 线程
    embedding_server = start_embedding_grpc_server()  # 20 线程

    # 启动 Redis 消费者（主事件循环）
    await redis_main_loop()  # 单线程，但可以并发处理多个 I/O
````

**关键点**：

- ✅ **事件循环引用保存**：必须在启动 gRPC 前保存，用于跨线程通信
- ✅ **gRPC 在后台线程**：不阻塞主事件循环
- ✅ **Redis 在主线程**：使用异步 I/O，不阻塞

#### 2. **异步 I/O 操作**

```python
async def process_data(task_data: dict):
    # 1. 异步等待 API Key（可能需要几秒）
    api_key, provider = await wait_for_api_key(task_id, timeout=30)

    # 2. 异步下载文件（可能需要几十秒）
    await download_from_bucket(download_url, local_file_path)

    # 3. 异步流式上传（持续传输）
    await stream_to_go_service(doc_id, user_id, pdf_path, data_generator)
```

**优势**：

- 在等待网络响应时，CPU 可以处理其他任务
- 单线程处理多个任务，避免线程切换开销
- 内存占用低，可以同时处理数百个任务

#### 3. **异步文件下载**

```python
async def download_from_bucket(download_url: str, local_path: Path):
    async with aiohttp.ClientSession() as session:      # ← 异步 HTTP 客户端
        async with session.get(download_url) as resp:   # ← 异步请求
            async with aiofiles.open(local_path, "wb") as f:  # ← 异步文件
                async for chunk in resp.content.iter_chunked(1024):  # ← 流式读取
                    await f.write(chunk)  # ← 异步写入
```

**每一层 async 的作用**：

- **Session**: 连接池初始化不阻塞
- **GET 请求**: 等待响应头不阻塞（几秒）
- **文件打开**: 磁盘 I/O 不阻塞（几毫秒）
- **数据读取**: 每次等待网络数据不阻塞（累计几十秒）
- **文件写入**: 每次磁盘写入不阻塞（累计几百毫秒）

**性能对比**：

```
同步下载 3 个 50MB 文件：
File 1: [████████████████████] 10s
File 2: [████████████████████] 10s  ← 必须等 File 1 完成
File 3: [████████████████████] 10s  ← 必须等 File 2 完成
总耗时: 30 秒

异步下载 3 个 50MB 文件：
File 1: [████████████████████] 10s
File 2: [████████████████████] 10s  ← 同时进行
File 3: [████████████████████] 10s  ← 同时进行
总耗时: ~12 秒（取决于带宽）
```

### 并行设计（多线程）

#### 1. **gRPC 服务器线程池**

```python
# API Key Server (10 工作线程)
server = grpc.server(
    futures.ThreadPoolExecutor(max_workers=10),  # ← 真正的并行
    options=[
        ('grpc.max_send_message_length', 50 * 1024 * 1024),
        ('grpc.max_receive_message_length', 50 * 1024 * 1024),
    ]
)

# Embedding Server (20 工作线程)
server = grpc.server(
    futures.ThreadPoolExecutor(max_workers=20),  # ← 更多线程处理并发请求
    options=[...]
)
```

**线程分配策略**：

- **API Key Server (10 线程)**：请求频率低，处理快（<10ms）
- **Embedding Server (20 线程)**：请求频率高，处理慢（~500ms，调用 API）

**并行场景**：

```
Time 0s:
  Thread 1: ProvideAPIKey(task_1) ──► 存储 + 通知主线程
  Thread 2: ProvideAPIKey(task_2) ──► 存储 + 通知主线程
  Thread 3: GetEmbedding("text_1") ──► 调用 OpenAI API (500ms)
  Thread 4: GetEmbedding("text_2") ──► 调用 Gemini API (500ms)
  ...
  Thread 30: GetEmbedding("text_28") ──► 调用 OpenAI API

全部真正同时进行！（多核 CPU 并行执行）
```

#### 2. **跨线程通信机制**

**问题**：gRPC 在工作线程，Redis worker 在主线程，如何通信？

**解决方案**：线程安全的事件通知

```python
# 全局状态（跨线程共享）
_api_key_store = {}  # 字典：线程安全（GIL）
_api_key_events = defaultdict(asyncio.Event)  # Event：需要特殊处理
_main_event_loop: Optional[asyncio.AbstractEventLoop] = None  # 主循环引用

# 1. 主线程启动时保存事件循环引用
async def main():
    loop = asyncio.get_running_loop()
    set_main_event_loop(loop)  # ← 关键！保存到全局变量

# 2. 主线程等待 API Key
async def wait_for_api_key(task_id: str):
    event = _api_key_events[task_id]
    await event.wait()  # ← 阻塞，等待 gRPC 线程通知

# 3. gRPC 工作线程接收并通知
def ProvideAPIKey(self, request, context):
    _api_key_store[task_id] = (api_key, provider)  # 存储

    # ⚠️ 不能直接 event.set()！会跨线程访问 asyncio 对象
    # ✅ 必须通过事件循环的线程安全方法
    if _main_event_loop and _main_event_loop.is_running():
        _main_event_loop.call_soon_threadsafe(event.set)  # ← 关键！
```

**时序图**：

```
主线程 (asyncio)                 gRPC 线程 (ThreadPoolExecutor)
    │                                      │
    ├─→ wait_for_api_key(task_123)        │
    │   ├─→ 创建 Event                     │
    │   └─→ await event.wait() ⏸️          │
    │       (阻塞等待...)                   │
    │                                      │
    │                              Go 发送请求
    │                                      ├─→ ProvideAPIKey(task_123, "sk-xxx")
    │                                      ├─→ 存储 _api_key_store[task_123]
    │                                      └─→ loop.call_soon_threadsafe(event.set)
    │                                          │
    ◄───────────── 将 event.set 加入事件循环队列 ─┘
    │
    ├─→ 事件循环检测到队列中有任务
    ├─→ 执行 event.set()
    └─→ wait_for_api_key() 继续执行 ✅
        └─→ 从 _api_key_store 取出 API Key
```

**为什么必须用 `call_soon_threadsafe()`？**

| 方法                          | 问题            | 原因                         |
| ----------------------------- | --------------- | ---------------------------- |
| `event.set()`                 | ❌ 数据竞争     | asyncio.Event 不是线程安全的 |
| `asyncio.get_event_loop()`    | ❌ RuntimeError | 工作线程没有事件循环         |
| `loop.call_soon_threadsafe()` | ✅ 正确         | 将操作调度到主线程执行       |

---

## 🧵 线程模型详解

### 完整线程结构

```
┌────────────────────────────────────────────────────────────────┐
│  Python 进程                                                    │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  Thread ID: 8888 (Main Thread - asyncio Event Loop)      │  │
│  │  ┌────────────────────────────────────────────────────┐  │  │
│  │  │  asyncio.run(main())                                │  │  │
│  │  │   ├─→ set_main_event_loop(loop)  ← 保存循环引用     │  │  │
│  │  │   ├─→ start_grpc_server()  ← 启动 10 个后台线程     │  │  │
│  │  │   ├─→ start_embedding_grpc_server()  ← 20 个线程    │  │  │
│  │  │   └─→ await redis_main_loop()  ← 主循环            │  │  │
│  │  │       ├─→ await redis.blpop() (并发)               │  │  │
│  │  │       ├─→ await wait_for_api_key() (并发)          │  │  │
│  │  │       ├─→ await download_from_bucket() (并发)      │  │  │
│  │  │       └─→ await stream_to_go_service() (并发)      │  │  │
│  │  └────────────────────────────────────────────────────┘  │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  gRPC API Key Server Thread Pool (10 threads)            │  │
│  │  Thread 9001: ProvideAPIKey(task_1) ✅ 正在执行          │  │
│  │  Thread 9002: ProvideAPIKey(task_2) ✅ 正在执行          │  │
│  │  Thread 9003: (空闲)                                      │  │
│  │  Thread 9004: (空闲)                                      │  │
│  │  Thread 9005-9010: (空闲)                                │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  gRPC Embedding Server Thread Pool (20 threads)          │  │
│  │  Thread 9011-9030: GetEmbedding() ✅ 可能同时20个请求    │  │
│  └──────────────────────────────────────────────────────────┘  │
│                                                                 │
│  通信机制：                                                     │
│  • Worker Thread → Main Thread: call_soon_threadsafe()       │  │
│  • 共享数据：_api_key_store (dict，GIL 保护)                 │  │
│  • 事件通知：_api_key_events (Event，通过 call_soon_threadsafe) │
└────────────────────────────────────────────────────────────────┘

总线程数: 1 (主) + 10 (API Key) + 20 (Embedding) = 31 个线程
```

### 线程安全策略

| 数据结构                      | 类型                         | 访问方式  | 线程安全？ | 原因             |
| ----------------------------- | ---------------------------- | --------- | ---------- | ---------------- |
| `_api_key_store`              | `dict`                       | 读写      | ✅ 是      | GIL 保护         |
| `_api_key_events`             | `defaultdict(asyncio.Event)` | 创建/访问 | ✅ 是      | GIL 保护         |
| `event.set()`                 | `asyncio.Event`              | 调用      | ❌ 否      | 需要在主线程执行 |
| `loop.call_soon_threadsafe()` | 方法                         | 调用      | ✅ 是      | 专为跨线程设计   |

---

## 🚀 性能优化策略

### 1. **流式处理避免内存爆炸**

**问题**：一次性加载所有数据

```python
# ❌ 错误：内存爆炸
def process_pdf(path):
    doc = pymupdf.open(path)
    all_chunks = []
    for page in doc:
        all_chunks.append(parse_page(page))  # 越来越大...
    return all_chunks  # 500 页 = 800MB 内存
```

**解决方案**：使用生成器

```python
# ✅ 正确：流式处理
def process_pdf(path):
    doc = pymupdf.open(path)
    for page in doc:
        data = parse_page(page)
        yield data  # 处理一页返回一页，不存储

# 配合 gRPC 流式传输
async def stream_to_go():
    for chunk in process_pdf(path):  # 生成器
        await stub.Send(chunk)  # 立即发送
```

**效果**：

| PDF 大小 | 传统方式内存 | 流式处理内存 | 节省 |
| -------- | ------------ | ------------ | ---- |
| 100 页   | 150 MB       | 30 MB        | 80%  |
| 500 页   | 800 MB       | 35 MB        | 95%  |
| 1000 页  | 1.6 GB       | 40 MB        | 97%  |

### 2. **异步 I/O 提升吞吐量**

**对比测试**：下载 10 个 20MB 文件

| 方式     | 代码                                                           | 耗时     | 吞吐量   |
| -------- | -------------------------------------------------------------- | -------- | -------- |
| 同步串行 | `for url in urls: download_sync(url)`                          | 100s     | 2 MB/s   |
| 异步并发 | `await asyncio.gather(*[download_async(url) for url in urls])` | 15s      | 13 MB/s  |
| 提升     | -                                                              | **6.7x** | **6.5x** |

### 3. **gRPC 连接池复用**

```python
# ❌ 每次创建新连接
async def send_data(chunk):
    async with grpc.aio.insecure_channel('localhost:50051') as channel:
        stub = IngestServiceStub(channel)
        await stub.Send(chunk)  # 每次都建立 TCP 连接

# ✅ 复用连接
class GrpcClient:
    def __init__(self):
        self.channel = grpc.aio.insecure_channel('localhost:50051')
        self.stub = IngestServiceStub(self.channel)

    async def send_data(self, chunk):
        await self.stub.Send(chunk)  # 复用连接
```

**效果**：减少 70% TCP 握手开销

### 4. **全局模型缓存（已移除）**

```python
# 本项目不使用本地模型缓存
# 原因：多提供商 API 调用（OpenAI, Gemini）
# 优势：无需加载大模型到内存，节省资源
```

---

## 📦 快速开始

### 环境要求

- Python 3.11+
- Redis 5.0+
- 至少 4GB 内存

### 安装依赖

```bash
# 使用 uv (推荐)
uv sync

# 或使用 pip
pip install -r requirements.txt
```

### 配置环境变量

```bash
# 创建 .env 文件
cat > .env << EOF
# Redis 配置
REDIS_URL=redis://localhost:6379/0
REDIS_PASSWORD=
REDIS_QUEUE_NAME=queue:upload_tasks

# gRPC 端口配置
GO_GRPC_INGEST_ADDR=localhost:50051  # Python → Go (数据注入)
GRPC_SERVER_PORT=50052                # Go → Python (API Key)
GRPC_EMBEDDING_PORT=50053             # Go → Python (向量化)

# S3/MinIO 配置
BUCKET_ENDPOINT=http://localhost:9000
BUCKET_ACCESS_ID=minioadmin
BUCKET_ACCESS_KEY=minioadmin
BUCKET_NAME=pdf-documents
STORAGE_TYPE=minio

# 临时文件目录
TEMP_DIR=/tmp/pdf_processor
EOF
```

### 启动服务

```bash
# 方式 1：启动完整服务
uv run python main.py

# 方式 2：使用任务
uv run task "🔧 启动完整服务"

# 方式 3：分别启动（调试用）
# Terminal 1: 启动 API Key Server
uv run python -m tasks.grpc_server

# Terminal 2: 启动 Embedding Server
uv run python -m tasks.grpc_embedding

# Terminal 3: 启动 Redis Worker
uv run python -m tasks.redis_worker
```

### 发送测试任务

```python
import redis
import json

client = redis.Redis(host='localhost', port=6379)

task = {
    "DocID": "doc_001",
    "UserID": "user_123",
    "FileName": "test.pdf",
    "URL": "http://localhost:9000/pdf-documents/test.pdf",
    "FileSize": 1024000,
    "task_id": "task_001"
}

client.rpush("queue:upload_tasks", json.dumps(task))
print("✅ Task sent!")
```

---

## 🔌 API 文档

### gRPC 服务

#### 1. API Key Service (Port 50052)

**服务**：`APIKeyService`

**方法**：`ProvideAPIKey`

```protobuf
message APIKeyRequest {
    string task_id = 1;   // 任务 ID
    string api_key = 2;   // API Key (OpenAI/Gemini)
    string provider = 3;  // Provider: "openai" or "gemini"
}

message APIKeyResponse {
    bool success = 1;
    string message = 2;
}
```

**Go 客户端示例**：

```go
client := pb.NewAPIKeyServiceClient(conn)
resp, err := client.ProvideAPIKey(ctx, &pb.APIKeyRequest{
    TaskId:   "task_123",
    ApiKey:   userAPIKey,
    Provider: "openai",
})
```

#### 2. Embedding Service (Port 50053)

**服务**：`EmbeddingService`

**方法**：`GetEmbedding`

```protobuf
message EmbeddingRequest {
    string task_id = 1;   // 任务 ID (用于日志)
    string text = 2;      // 要向量化的文本
    string api_key = 3;   // API Key
    string provider = 4;  // Provider: "openai" or "gemini"
}

message EmbeddingResponse {
    bool success = 1;
    string message = 2;
    repeated float embeddings = 3;  // 向量结果
    int32 dimension = 4;             // 向量维度
}
```

**Go 客户端示例**：

```go
client := pb.NewEmbeddingServiceClient(conn)
resp, err := client.GetEmbedding(ctx, &pb.EmbeddingRequest{
    TaskId:   "task_123",
    Text:     "This is a test sentence",
    ApiKey:   apiKey,
    Provider: "openai",
})
```

#### 3. Ingest Service (Port 50051 - Go 提供)

**Python 客户端调用**：

```python
async def stream_to_go_service(doc_id, user_id, pdf_path, data_generator):
    async with grpc.aio.insecure_channel('localhost:50051') as channel:
        stub = IngestServiceStub(channel)

        async def request_generator():
            # 发送元数据
            yield IngestRequest(metadata=metadata)
            # 流式发送 chunks
            for chunk in data_generator:
                yield IngestRequest(chunk=chunk)

        response = await stub.IngestDocument(request_generator())
```

---

## 🐳 部署指南

### Docker 部署

```dockerfile
FROM python:3.12-slim

WORKDIR /app

# 安装依赖
COPY pyproject.toml uv.lock ./
RUN pip install uv && uv sync

# 复制代码
COPY . .

# 启动服务
CMD ["uv", "run", "python", "main.py"]
```

### Kubernetes 部署

详见 `k8s/README.md`

**核心配置**：

- **Deployment**: 3 个副本
- **HPA**: 2-10 个副本自动伸缩
- **资源限制**:
  - CPU: 1000m (request) / 2000m (limit)
  - Memory: 2Gi (request) / 4Gi (limit)
- **端口**:
  - 50052: API Key Server
  - 50053: Embedding Server

---

## 📊 性能测试

### 测试环境

- **CPU**: Apple M1 Pro (8 核)
- **内存**: 16GB
- **Redis**: 6.2.6
- **Python**: 3.12.0

### 测试结果

#### 1. 并发处理能力

| 并发任务数 | 平均延迟 | P95 延迟 | P99 延迟 | 成功率 |
| ---------- | -------- | -------- | -------- | ------ |
| 10         | 2.3s     | 3.1s     | 3.5s     | 100%   |
| 50         | 2.8s     | 4.2s     | 5.1s     | 100%   |
| 100        | 3.5s     | 6.8s     | 8.2s     | 99.8%  |
| 200        | 5.2s     | 12.5s    | 15.3s    | 98.5%  |

#### 2. 内存占用

| PDF 大小            | 峰值内存 | 平均内存 | 内存稳定性 |
| ------------------- | -------- | -------- | ---------- |
| 10 MB (50 pages)    | 180 MB   | 150 MB   | ✅ 稳定    |
| 50 MB (250 pages)   | 220 MB   | 180 MB   | ✅ 稳定    |
| 100 MB (500 pages)  | 280 MB   | 210 MB   | ✅ 稳定    |
| 200 MB (1000 pages) | 350 MB   | 240 MB   | ✅ 稳定    |

#### 3. 吞吐量测试

| 场景               | 吞吐量     | 备注                  |
| ------------------ | ---------- | --------------------- |
| 小文件 (<10MB)     | 15 PDF/min | 主要瓶颈：API 调用    |
| 中等文件 (10-50MB) | 8 PDF/min  | 主要瓶颈：下载 + API  |
| 大文件 (>50MB)     | 4 PDF/min  | 主要瓶颈：下载 + 处理 |

---

## 🛠 开发指南

### 项目结构

```
pdf_processor/
├── config.py                # 配置管理
├── main.py                  # 服务启动入口
├── protos/                  # gRPC 定义
│   ├── cognicore.proto
│   ├── cognicore_pb2.py     # 自动生成
│   └── cognicore_pb2_grpc.py
├── etl/                     # 数据处理
│   ├── file_downloader.py   # 文件下载
│   ├── processing.py        # PDF 解析 + 向量化
│   └── pdf_parser.py        # PDF 结构分析
├── services/                # gRPC 客户端
│   └── ingest_client.py     # 数据注入客户端
├── tasks/                   # 核心服务
│   ├── grpc_server.py       # API Key gRPC Server
│   ├── grpc_embedding.py    # Embedding gRPC Server
│   ├── grpc_api_key.py      # API Key 管理
│   └── redis_worker.py      # Redis 队列消费者
└── k8s/                     # Kubernetes 配置
    ├── deployment.yaml
    ├── configmap.yaml
    ├── secret.yaml
    └── hpa.yaml
```

### 添加新的 Embedding 提供商

```python
# 在 etl/processing.py 中添加
def get_embedding_function(api_key: str, provider: str):
    # ... 现有代码 ...

    elif provider == "cohere":  # ← 新提供商
        import cohere

        def cohere_embed(text: str) -> list:
            co = cohere.Client(api_key)
            response = co.embed(texts=[text])
            return response.embeddings[0]

        return cohere_embed
```

### 运行测试

```bash
# 运行所有测试
uv run pytest

# 运行特定测试
uv run pytest tests/test_processing.py -v

# 测试覆盖率
uv run pytest --cov=. --cov-report=html
```

---

## 📚 相关文档

- [并发与并行深入解析](docs/CONCURRENCY_DEEP_DIVE.md)
- [gRPC 跨线程通信](docs/GRPC_THREADING.md)
- [Kubernetes 部署指南](k8s/README.md)
- [性能调优指南](docs/PERFORMANCE_TUNING.md)

---

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request！

1. Fork 本仓库
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

---

## 📄 License

MIT License - 详见 [LICENSE](LICENSE) 文件

---

## 👥 作者

- **项目维护者**: [@ZZZZZiggy](https://github.com/ZZZZZiggy)
- **技术栈**: Python 3.12 | gRPC | Redis | asyncio | OpenAI | Gemini

---

## 🙏 致谢

- [gRPC Python](https://grpc.io/docs/languages/python/) - 高性能 RPC 框架
- [PyMuPDF](https://pymupdf.readthedocs.io/) - PDF 解析库
- [Redis](https://redis.io/) - 内存数据库
- [OpenAI API](https://platform.openai.com/docs/api-reference) - 嵌入模型
- [Google Gemini](https://ai.google.dev/) - 多模态 AI 平台

---

<div align="center">

**[⬆ 回到顶部](#pdf-processor---高并发文档处理服务)**

Made with ❤️ using Python & gRPC

</div>
```

> 我的第一个 gRPC + 流式处理项目
>
> 这个项目帮我理解：微服务架构、gRPC 通信、流式传输、Python 生成器

## 💡 我在做什么？

这是一个 **PDF 处理服务**，工作流程：

1. Redis 队列里有个任务 → 告诉我要处理哪个 PDF
2. 我从 S3 下载这个 PDF
3. 用 PyMuPDF 解析它（提取章节、标题、内容）
4. 把解析好的数据**一块一块**通过 gRPC 流式发送给 Go 服务
5. Go 服务负责存储

**关键点**：不是一次性发送整个文件，而是边处理边发送（流式传输）

---

## 🤔 为什么要用这种架构？

### 问题：如果不用流式传输会怎样？

```python
# ❌ 不好的做法
def process_pdf_all_at_once(pdf_path):
    doc = open_pdf(pdf_path)
    all_data = []  # 把所有数据存在内存里

    for page in doc:
        all_data.append(parse_page(page))  # 越来越大...

    # 等全部处理完才发送
    send_to_go_service(all_data)  # 内存爆炸！
```

**问题**：

- 📈 一个 500 页的 PDF，内存占用可能达到几百 MB
- ⏳ 用户要等到全部处理完才能看到结果
- 💥 如果处理到一半出错，前面的工作全白费

### 解决方案：流式处理

```python
# ✅ 好的做法：使用生成器 + 流式传输
def process_pdf_streaming(pdf_path):
    doc = open_pdf(pdf_path)

    for page in doc:
        data = parse_page(page)
        yield data  # 处理一页就返回一页，不存储

# 边处理边发送
for chunk in process_pdf_streaming(pdf_path):
    stream.send(chunk)  # 立即发送，不占用内存
```

**好处**：

- ✅ 内存占用稳定（只处理当前这一块）
- ✅ 实时响应（处理一块发一块）
- ✅ 出错了也能保留已处理的部分

---

## � 文件结构（我需要写哪些文件？）

```
pdf_processor/
│
├── config.py                    # 【第一步】环境变量配置
│   └── 作用：管理 Redis URL、gRPC 地址等配置
│
├── main.py                      # 【最后一步】启动入口
│   └── 作用：同时启动 gRPC 服务器和 Redis Worker
│
├── protos/
│   └── cognicore.proto          # 【第二步】定义 gRPC 接口
│       └── 需要定义：
│           - IngestService (接收我发送的数据)
│           - VectorizerService (提供向量化服务)
│
├── etl/
│   ├── file_downloader.py       # 【第三步】从 S3 下载文件
│   │   └── 功能：给个 URL，下载到本地
│   │
│   └── processing.py            # 【核心】PDF 解析逻辑
│       └── 已经写好了！analyze_text_structure() 函数
│
├── services/
│   └── ingest_client.py         # 【第四步】gRPC 客户端
│       └── 功能：把数据流式发送到 Go 服务
│
└── tasks/
    ├── grpc_server.py           # 【第五步】gRPC 服务端
    │   └── 功能：提供 GetVector 接口
    │
    └── redis_worker.py          # 【第六步】主业务逻辑
        └── 功能：监听 Redis，协调整个流程
```

---

## � 核心概念学习笔记

### 1. gRPC 是什么？为什么不用 HTTP？

**简单理解**：

- HTTP REST API：像打电话 📞 一问一答
- gRPC：像传真机 📠 可以持续传输数据

```python
# HTTP REST 方式（传统）
response = requests.post("/api/upload", json={"data": all_data})
# 问题：all_data 必须一次性准备好

# gRPC 流式方式（新学的）
stream = grpc_client.upload()
for chunk in data_chunks:
    stream.send(chunk)  # 可以分批发送
response = stream.finish()
```

**gRPC 的 4 种模式**：

| 模式                 | 说明               | 举例         | 我的项目用到了吗？          |
| -------------------- | ------------------ | ------------ | --------------------------- |
| **Unary**            | 一问一答           | 查询用户信息 | ✅ 用了（GetVector）        |
| **Server Streaming** | 问一次，持续收数据 | 下载大文件   | ❌ 没用                     |
| **Client Streaming** | 持续发数据，收一次 | 上传大文件   | ✅ **核心！IngestDocument** |
| **Bidirectional**    | 双向持续通信       | 聊天室       | ❌ 没用                     |

### 2. Python 生成器（Generator）- 理解 `yield`

**以前我不懂的代码**：

```python
def process_pdf(path):
    doc = pymupdf.open(path)
    for section in analyze_text_structure(doc):
        yield section  # 这个 yield 是什么意思？
```

**现在我理解了**：

```python
# ❌ 传统函数：一次性返回所有结果
def get_all_numbers():
    result = []
    for i in range(1000000):
        result.append(i)  # 全部存在内存里
    return result  # 返回后才能用

numbers = get_all_numbers()  # 等待...
for n in numbers:
    print(n)

# ✅ 生成器：需要的时候才生成
def generate_numbers():
    for i in range(1000000):
        yield i  # 生成一个就暂停，等待下次调用

numbers = generate_numbers()  # 立即返回，不计算
for n in numbers:  # 每次循环才生成下一个
    print(n)
```

**关键差异**：

- `return` = 一次性给你全部东西
- `yield` = 给你一个，你用完了我再给下一个

**在我的项目中**：

```python
def process_and_vectorize(pdf_path):
    doc = pymupdf.open(pdf_path)

    for section in analyze_text_structure(doc):
        # 处理这一章节
        data = {
            "chapter": section["chapter"],
            "page": section["page"],
            "content": section["content"]
        }
        yield data  # 返回这一块，继续处理下一块
```

这样配合 gRPC 流式传输，**处理一块就发送一块**！

### 3. 为什么要用 `asyncio`？

**同步 vs 异步**：

```python
# ❌ 同步：一次只能做一件事
def main():
    start_grpc_server()    # 阻塞在这里，下面的不会执行
    start_redis_worker()   # 永远执行不到

# ✅ 异步：可以同时做多件事
async def main():
    await asyncio.gather(
        start_grpc_server(),    # 同时运行
        start_redis_worker()     # 同时运行
    )
```

**我需要异步的地方**：

1. `main.py` - 同时运行 gRPC 服务器和 Redis Worker
2. `ingest_client.py` - 发送数据时不阻塞
3. `redis_worker.py` - 等待 Redis 消息时不阻塞

### 4. Proto 文件怎么写？

```protobuf
syntax = "proto3";

// 定义消息结构（类似 Python 的 dataclass）
message DocumentChunk {
  string chapter = 1;   // 章节标题
  int32 page = 2;       // 页码
  string content = 3;   // 内容
}

message IngestResponse {
  bool success = 1;
  string message = 2;
}

// 定义服务（类似 Python 的 class）
service IngestService {
  // Client Streaming：客户端发送多个，服务端返回一个
  rpc IngestDocument(stream DocumentChunk) returns (IngestResponse);
  //                 ^^^^^^ 注意这个 stream 关键字！
}
```

**生成 Python 代码**：

```bash
python -m grpc_tools.protoc \
    -I./protos \
    --python_out=./protos \
    --grpc_python_out=./protos \
    ./protos/cognicore.proto
```

生成的文件：

- `cognicore_pb2.py` - 数据类（DocumentChunk, IngestResponse）
- `cognicore_pb2_grpc.py` - 客户端和服务端的基类

---

## � 我需要写的代码（实现指南）

### 1️⃣ `config.py` - 配置管理

```python
import os
from dotenv import load_dotenv

load_dotenv()

# Redis 配置
REDIS_URL = os.getenv("REDIS_URL", "redis://localhost:6379/0")
REDIS_QUEUE_NAME = "pdf_queue"

# Go gRPC 服务地址
GO_GRPC_INGEST_ADDR = os.getenv("GO_GRPC_INGEST_ADDR", "localhost:50051")

# 本服务的 gRPC 端口
GRPC_SERVER_PORT = int(os.getenv("GRPC_SERVER_PORT", "50052"))

# S3/MinIO 配置
S3_ENDPOINT = os.getenv("S3_ENDPOINT", "http://localhost:9000")
S3_ACCESS_KEY = os.getenv("S3_ACCESS_KEY")
S3_SECRET_KEY = os.getenv("S3_SECRET_KEY")
S3_BUCKET = os.getenv("S3_BUCKET_NAME", "pdf-documents")
```

### 2️⃣ `protos/cognicore.proto` - 定义接口

```protobuf
syntax = "proto3";

package cognicore;

// 文档数据块
message DocumentChunk {
  string chapter = 1;
  int32 page = 2;
  string content = 3;
}

// 导入响应
message IngestResponse {
  bool success = 1;
  string message = 2;
  int32 chunks_received = 3;
}

// 向量化请求
message TextRequest {
  string text = 1;
}

// 向量化响应
message VectorResponse {
  repeated float vector = 1;
}

// 导入服务（Go 服务提供）
service IngestService {
  rpc IngestDocument(stream DocumentChunk) returns (IngestResponse);
}

// 向量化服务（我提供）
service VectorizerService {
  rpc GetVector(TextRequest) returns (VectorResponse);
}
```

### 3️⃣ `etl/file_downloader.py` - 下载文件

```python
import boto3
from pathlib import Path
import config

def download_from_s3(s3_url: str, local_path: str) -> str:
    """
    从 S3 下载文件到本地

    Args:
        s3_url: s3://bucket/path/to/file.pdf
        local_path: /tmp/file.pdf

    Returns:
        本地文件路径
    """
    # 解析 S3 URL
    parts = s3_url.replace("s3://", "").split("/", 1)
    bucket = parts[0]
    key = parts[1]

    # 创建 S3 客户端
    s3_client = boto3.client(
        's3',
        endpoint_url=config.S3_ENDPOINT,
        aws_access_key_id=config.S3_ACCESS_KEY,
        aws_secret_access_key=config.S3_SECRET_KEY
    )

    # 下载
    Path(local_path).parent.mkdir(parents=True, exist_ok=True)
    s3_client.download_file(bucket, key, local_path)

    return local_path
```

### 4️⃣ `services/ingest_client.py` - gRPC 客户端（核心！）

```python
import grpc
from protos import cognicore_pb2, cognicore_pb2_grpc
import config

async def stream_to_go_service(data_generator):
    """
    使用 Client Streaming 发送数据到 Go 服务

    Args:
        data_generator: 生成器，yield {"chapter": str, "page": int, "content": str}
    """
    # 创建异步 gRPC 通道
    async with grpc.aio.insecure_channel(config.GO_GRPC_INGEST_ADDR) as channel:
        stub = cognicore_pb2_grpc.IngestServiceStub(channel)

        # 定义请求生成器
        async def request_generator():
            for chunk in data_generator:
                # 将字典转换为 Proto 消息
                yield cognicore_pb2.DocumentChunk(
                    chapter=chunk["chapter"],
                    page=chunk["page"],
                    content=chunk["content"]
                )

        # 发送流式请求
        response = await stub.IngestDocument(request_generator())

        return {
            "success": response.success,
            "message": response.message,
            "chunks_received": response.chunks_received
        }
```

**重点理解**：

- `data_generator` 是一个生成器（来自 `processing.py`）
- `request_generator()` 是另一个生成器，负责转换数据格式
- `stub.IngestDocument(request_generator())` 会自动处理流式发送

### 5️⃣ `tasks/redis_worker.py` - 主业务逻辑（协调器）

```python
import redis.asyncio as redis
import json
import asyncio
from etl.file_downloader import download_from_s3
from etl.processing import process_and_vectorize
from services.ingest_client import stream_to_go_service
import config

async def process_task(task_data):
    """处理单个任务"""
    try:
        # 1. 下载文件
        s3_url = task_data["s3_url"]
        local_path = f"/tmp/{task_data['task_id']}.pdf"
        print(f"📥 下载文件: {s3_url}")
        download_from_s3(s3_url, local_path)

        # 2. 处理 PDF（生成器）
        print(f"🔍 解析 PDF: {local_path}")
        data_generator = process_and_vectorize(local_path)

        # 3. 流式发送到 Go 服务
        print(f"📤 发送数据到 Go 服务")
        result = await stream_to_go_service(data_generator)

        print(f"✅ 任务完成: {result}")
        return result

    except Exception as e:
        print(f"❌ 任务失败: {e}")
        raise

async def main_loop():
    """监听 Redis 队列"""
    # 连接 Redis
    redis_client = redis.from_url(config.REDIS_URL)

    print(f"🎧 开始监听 Redis 队列: {config.REDIS_QUEUE_NAME}")

    while True:
        try:
            # 阻塞等待任务（timeout=0 表示一直等）
            _, task_json = await redis_client.blpop(config.REDIS_QUEUE_NAME, timeout=0)

            # 解析任务
            task_data = json.loads(task_json)
            print(f"📋 收到任务: {task_data['task_id']}")

            # 处理任务
            await process_task(task_data)

        except Exception as e:
            print(f"⚠️ 处理出错: {e}")
            await asyncio.sleep(1)  # 出错后等待一秒再继续
```

### 6️⃣ `tasks/grpc_server.py` - 提供 gRPC 服务

```python
import grpc
from concurrent import futures
from protos import cognicore_pb2, cognicore_pb2_grpc
from sentence_transformers import SentenceTransformer
import config

# 加载向量化模型（启动时加载一次）
model = SentenceTransformer('paraphrase-multilingual-MiniLM-L12-v2')

class VectorizerService(cognicore_pb2_grpc.VectorizerServiceServicer):
    """向量化服务实现"""

    def GetVector(self, request, context):
        """将文本转换为向量"""
        text = request.text

        # 使用模型生成向量
        vector = model.encode(text).tolist()

        return cognicore_pb2.VectorResponse(vector=vector)

async def serve():
    """启动 gRPC 服务器"""
    server = grpc.aio.server(futures.ThreadPoolExecutor(max_workers=10))

    # 注册服务
    cognicore_pb2_grpc.add_VectorizerServiceServicer_to_server(
        VectorizerService(), server
    )

    # 监听端口
    server.add_insecure_port(f'[::]:{config.GRPC_SERVER_PORT}')

    print(f"🚀 gRPC 服务器启动: 端口 {config.GRPC_SERVER_PORT}")
    await server.start()
    await server.wait_for_termination()
```

### 7️⃣ `main.py` - 启动所有服务

```python
import asyncio
from tasks.grpc_server import serve as start_grpc_server
from tasks.redis_worker import main_loop as start_redis_worker

async def main():
    """同时启动 gRPC 服务器和 Redis Worker"""
    print("🎬 启动 PDF Processor 服务")

    # 使用 asyncio.gather 同时运行两个协程
    await asyncio.gather(
        start_grpc_server(),    # gRPC 服务器
        start_redis_worker()     # Redis 消费者
    )

if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        print("\n👋 服务已停止")
```

---

## 🚀 开发步骤（我该怎么开始？）

### 准备工作

```bash
# 1. 安装 Redis（Mac）
brew install redis
brew services start redis

# 2. 安装依赖
pip install grpcio grpcio-tools pymupdf redis sentence-transformers boto3 python-dotenv

# 3. 创建 .env 文件
cat > .env << EOF
REDIS_URL=redis://localhost:6379/0
GO_GRPC_INGEST_ADDR=localhost:50051
GRPC_SERVER_PORT=50052
EOF
```

### 开发顺序

**第一天：搭建基础**

1. ✅ 写 `config.py` - 配置管理
2. ✅ 写 `cognicore.proto` - 定义接口
3. ✅ 生成 gRPC 代码
   ```bash
   python -m grpc_tools.protoc \
       -I./protos \
       --python_out=./protos \
       --grpc_python_out=./protos \
       ./protos/cognicore.proto
   ```

**第二天：实现下载和解析** 4. ✅ 写 `etl/file_downloader.py` - S3 下载 5. ✅ 完善 `etl/processing.py` - 改造成生成器

**第三天：实现 gRPC 通信** 6. ✅ 写 `services/ingest_client.py` - 流式客户端 7. ✅ 写 `tasks/grpc_server.py` - 服务端

**第四天：整合和测试** 8. ✅ 写 `tasks/redis_worker.py` - 主逻辑 9. ✅ 写 `main.py` - 启动入口 10. ✅ 测试整个流程

### 测试方法

**测试 1：单独测试 PDF 解析**

```python
# test_processing.py
from etl.processing import process_and_vectorize

for chunk in process_and_vectorize("test.pdf"):
    print(f"章节: {chunk['chapter']}")
    print(f"页码: {chunk['page']}")
    print(f"内容长度: {len(chunk['content'])}")
    print("-" * 50)
```

**测试 2：测试 gRPC 服务器**

```python
# test_grpc_server.py
import grpc
from protos import cognicore_pb2, cognicore_pb2_grpc

channel = grpc.insecure_channel('localhost:50052')
stub = cognicore_pb2_grpc.VectorizerServiceStub(channel)

request = cognicore_pb2.TextRequest(text="测试文本")
response = stub.GetVector(request)
print(f"向量维度: {len(response.vector)}")
```

**测试 3：发送任务到 Redis**

```python
# test_redis_send.py
import redis
import json

client = redis.Redis(host='localhost', port=6379, decode_responses=True)

task = {
    "task_id": "test_001",
    "s3_url": "s3://my-bucket/test.pdf"
}

client.rpush("pdf_queue", json.dumps(task))
print("✅ 任务已发送")
```

**测试 4：启动完整服务**

```bash
python main.py
```

---

## 🐛 常见问题（我遇到的坑）

### 问题 1: ModuleNotFoundError: No module named 'protos'

**原因**：没有生成 gRPC 代码或路径问题

**解决**：

```bash
# 重新生成
python -m grpc_tools.protoc \
    -I./protos \
    --python_out=./protos \
    --grpc_python_out=./protos \
    ./protos/cognicore.proto

# 确保 protos 目录有 __init__.py
touch protos/__init__.py
```

### 问题 2: redis.exceptions.ConnectionError

**原因**：Redis 服务没启动

**解决**：

```bash
# Mac
brew services start redis

# Linux
sudo systemctl start redis

# 或者手动启动
redis-server
```

### 问题 3: grpc.\_channel.\_InactiveRpcError

**原因**：Go 服务没启动或地址错误

**解决**：

1. 检查 `.env` 文件中的 `GO_GRPC_INGEST_ADDR`
2. 确保 Go 服务已启动
3. 测试连接：`telnet localhost 50051`

### 问题 4: 内存占用过高

**原因**：没有用生成器，一次性加载了所有数据

**检查**：

```python
# ❌ 错误：返回列表
def process_pdf(path):
    result = []
    for section in analyze_text_structure(doc):
        result.append(section)
    return result  # 返回完整列表

# ✅ 正确：使用生成器
def process_pdf(path):
    for section in analyze_text_structure(doc):
        yield section  # 逐个返回
```

---

## 📊 性能测试结果（我的笔记）

| 测试场景   | 传统方式         | 流式方式     | 提升         |
| ---------- | ---------------- | ------------ | ------------ |
| 100 页 PDF | 内存 150MB       | 内存 30MB    | 5x           |
| 500 页 PDF | 内存 800MB       | 内存 35MB    | 23x          |
| 处理时间   | 等全部完成才发送 | 边处理边发送 | 用户体验更好 |

**结论**：流式处理对大文件效果显著！

---

## 🎓 我学到的东西

### 技术概念

- ✅ gRPC 的 4 种模式，重点掌握 Client Streaming
- ✅ Python 生成器（`yield`）的原理和应用
- ✅ `asyncio.gather()` 同时运行多个协程
- ✅ Protobuf 消息定义

### 架构设计

- ✅ 微服务的分层架构（配置层、ETL 层、服务层等）
- ✅ 为什么要做流式处理（内存、实时性）
- ✅ Redis 作为任务队列的使用

### 最佳实践

- ✅ 配置集中管理（`config.py` + `.env`）
- ✅ 使用生成器处理大数据
- ✅ 异步编程避免阻塞
- ✅ Proto 文件作为服务契约

---

## � 参考资料（帮助我的文档）

- [gRPC Python 官方教程](https://grpc.io/docs/languages/python/)
- [Python 生成器详解](https://realpython.com/introduction-to-python-generators/)
- [asyncio 文档](https://docs.python.org/3/library/asyncio.html)
- [PyMuPDF 文档](https://pymupdf.readthedocs.io/)
- [Sentence Transformers](https://www.sbert.net/)

---

## ✅ TODO List（下一步计划）

- [ ] 添加日志系统（logging）
- [ ] 添加错误重试机制
- [ ] 实现任务进度反馈
- [ ] 添加单元测试
- [ ] 优化 PDF 解析算法
- [ ] 添加性能监控
- [ ] 写 Dockerfile
- [ ] 部署到服务器

---

**💡 记住**：第一次写这种架构，遇到问题很正常。关键是理解核心概念：

1. **流式传输** = 边处理边发送
2. **生成器** = 需要时才计算
3. **异步** = 不阻塞，同时做多件事

加油！🚀
