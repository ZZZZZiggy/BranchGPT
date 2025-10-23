# 分布式改造入门指南 (针对当前项目)

> 适合：还没有分布式/微服务实践经验，但想把现在的单体 + Python 辅助脚本，逐步升级成更清晰、可扩展的结构。
>
> 目标：用最少的概念，做最实用的拆分，不一下子“过度工程”。

---
## 1. 现在项目是怎样的？
你目前有两个主要部分：
- Go 后端：负责对话（Chat）、文档上传、调用数据库(Postgres)、缓存(Redis)、调用 LLM(OpenAI/Gemini)、再通过 HTTP 请求 Python。
- Python 程序：解析 PDF，把内容拆成结构化文本/JSON。

问题：
1. 所有逻辑都塞在一个 Go 服务里，职责越来越杂。
2. Python 解析耗时，如果阻塞 HTTP，会拖慢用户体验。
3. 以后想扩容解析，只能多启动整个 Go 后端，不经济。
4. 没有清晰“任务状态”，不利于排查（到底是解析慢，还是向量生成慢？）

---
## 2. 我们的第一阶段目标
不要一次做完所有“高级”东西。第一阶段只做 4 件：
1. 把“PDF 解析”从同步调用，改成“异步任务”。
2. 定义一个清晰的“任务状态”存储在数据库里。
3. 引入一个“消息队列”负责排队（选一个简单的：Redis Streams 或 NATS，初学者可以先用已经有的 Redis）。
4. 给未来的服务拆分留接口（使用 gRPC 定义服务边界，但一开始可以只生成代码、慢慢用）。

为什么这样排？因为“解析异步化”马上提升体验：用户上传后不用等解析完成就能继续浏览“处理中”状态。

---
## 3. 最简单的拆分示意图
```
[Browser]
   |
   v
[Go API 服务]
   |  (1) 上传 PDF -> 存储到 S3(或本地) + 记录 task(status=pending)
   |  (2) 推送任务ID 到 消息队列 (Redis Stream)
   |
   |--(查询任务状态)--> Postgres

[Python Worker]
   ^  消费消息(任务ID) <- Redis Stream
   |  下载 PDF -> 解析 -> 分段 JSON
   |  写结果到 S3 / Postgres (status=completed / failed)
```
未来再加入：向量嵌入、语义搜索、流式回复等。

---
## 4. 核心新概念的“通俗解释”
| 名称 | 通俗比喻 | 本项目例子 |
|------|----------|-----------|
| 服务拆分 | 把大公司部门拆出来 | 解析、聊天、嵌入 各做各的 |
| 异步任务 | 点了外卖不用站着等 | 上传后立即返回“处理中” |
| 消息队列 | 外卖待制作清单 | Redis Stream 里的待解析 PDF |
| gRPC | 结构化的“内部电话” | 未来 Chat 调 Orchestrator |
| Proto | 通话“脚本/规范” | 定义 Request/Response 字段 |
| 状态机 | 外卖进度：下单/制作/送达 | pending -> processing -> done |
| 观测性 | 后台监控大屏 | 日志 + 指标 + Trace |

---
## 5. 第一阶段你需要做的改动（分步骤）
### 步骤 1：新增任务表
在 Postgres 加一张表：`document_tasks`
```
id (UUID) | file_name | status(pending/processing/completed/failed) | created_at | updated_at | error_msg | result_location
```
上传 PDF 时：
1. 生成 task_id 写入表（status=pending）
2. 文件存储到磁盘或 S3（先本地：`/data/uploads/<task_id>.pdf`）
3. 往 Redis Stream 添加一条：`XADD pdf_tasks * task_id=<id>`
4. 返回 JSON：`{"task_id": "...", "status": "pending"}`

### 步骤 2：Python 改成 Worker
原来 `app.py` 提供 HTTP；现在加一个 worker 脚本：
伪代码：
```
while True:
  msg = XREAD BLOCK 5000 STREAMS pdf_tasks >
  if msg:
     task_id = msg['task_id']
     更新 task status=processing
     读取 uploads/<task_id>.pdf
     解析 -> 生成 sections.json
     保存结果 JSON 到 result/<task_id>.json
     更新 task status=completed, result_location=path
```
错误时：`status=failed`, `error_msg="解析失败: ..."`

### 步骤 3：新增查询接口
Go 服务新增：`GET /documents/tasks/{id}` 查询数据库返回状态 + 结果地址(若完成)。
前端轮询或后面用 WebSocket 推送。

### 步骤 4：准备 gRPC Proto（为了后续扩展）
新建目录：`proto/pdfchat.proto`
只定义最核心的两个服务：
- DocumentTaskService（查询/创建任务）
- PdfProcessorService（以后如果要同步模式）
一开始可以不实现 gRPC，只是放在那里 + CI 保证不破坏。

---
## 6. 为什么第一阶段不直接做“全部微服务化”？
- 直接上 Kubernetes / Mesh / Kafka 会让你陷入大量运维学习，价值回报不高。
- 先把“异步 + 状态跟踪”做好，已经是 70% 用户体验提升。
- 后面再按“最慢/最难扩容”的模块拆：解析 -> 嵌入 -> 检索 -> LLM Orchestrator。

---
## 7. 第二阶段展望（等第一阶段稳定后）
| 阶段 | 新内容 | 指标关注 |
|------|--------|----------|
| 2 | 向量检索 (pgvector) | 搜索延迟 < 150ms |
| 3 | Chat 流式输出 | 首 Token 时间 |
| 4 | Embedding 异步队列化 | 入队等待时长 |
| 5 | 服务网格 & 追踪 | Trace 覆盖率 |

---
## 8. 代码结构建议（完成第一阶段后）
```
go_chat_backend/
  proto/                 # protobuf 放这里
  internal/
    tasks/               # 任务CRUD逻辑
    queue/               # Redis Stream 抽象
    storage/             # 文件存取封装
    handlers/            # HTTP Handlers (调用 internal)
  cmd/api/main.go        # 启动 HTTP API
python_worker/
  worker.py              # 消费任务
  parser/                # pdf_parser.py 原逻辑
  requirements.txt or pyproject.toml
shared/
  docs/ARCHITECTURE_BEGINNER.md
```

---
## 9. 监控最小集合 (MVP)
| 指标 | 采集方式 |
|------|----------|
| 解析任务总数 | 在 Go 创建任务时 +1 | 
| 解析失败数 | Python 捕获异常 +1 |
| 平均解析耗时 | Python 记录开始/结束，打印日志 | 
| 队列堆积长度 | 定期 `XLEN pdf_tasks` | 

不用一开始就上 Prometheus，可以先日志打点：`metric=parse_duration_ms value=123 task_id=...`

---
## 10. 常见坑和避免方式
| 坑 | 说明 | 预防 |
|----|------|------|
| 任务重复执行 | Worker 崩溃/重试导致 | Ack 模式 + 幂等：如果 status=completed 就跳过 |
| 大文件阻塞 | 同步解析耗时长 | 一定要异步队列 |
| 状态不同步 | 只存在内存 | 所有状态写 Postgres |
| JSON 膨胀 | 全部塞数据库 | 大块 JSON 放文件/S3，DB 只存指针路径 |
| 无法扩容 | 程序硬编码路径 | 用环境变量/配置抽象 queue, storage 路径 |

---
## 11. 下一步实际动作清单（我可帮你做）
1. 创建 `proto/pdfchat.proto`（初稿）
2. 添加 `ARCHITECTURE_BEGINNER.md`（本文件）
3. Go 新增：任务表迁移 SQL + 数据访问代码骨架
4. Go 新增：`POST /documents/upload` 改为异步写队列
5. Python 新增：`worker.py` 消费 Redis Stream

你可以先阅读这个文档，确认是否需要我直接继续创建 proto 和骨架代码。

---
## 12. 术语快速索引
| 术语 | 英文 | 含义 |
|------|------|------|
| 异步任务 | async job | 不阻塞用户请求的后台处理 |
| 消息队列 | message queue | 排队调度工作单元的系统 |
| Worker | worker | 持续消费并执行任务的进程 |
| Proto | protobuf schema | 定义服务与消息结构的文件 |
| gRPC | gRPC | 基于 HTTP/2 的高性能内部通信协议 |
| 状态机 | state machine | 任务从一个状态转换到另一个的规则 |

---
## 13. 如果后面继续升级
按顺序：
1. 增向量检索 (pgvector)
2. Chat 流式输出（节省用户等待）
3. Embedding 异步化（解析后再单独排队生成嵌入）
4. 统一请求追踪 (trace id 注入日志)
5. 引入 Kubernetes / 自动扩容
6. Service Mesh + mTLS + 细粒度限流

---

> 看完后如果都 OK，告诉我：继续执行。或者你想调整步骤，也可以说。