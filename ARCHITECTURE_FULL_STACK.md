# 全量技术栈与分布式系统蓝图（超详细新手版）

> 这是一份“我什么都不太懂也能看懂”的版本。每个技术名词我都会：先讲它是干什么的（像什么），为什么需要它（不用会怎样），再告诉你在本项目里什么时候上它 & 最小用法。你可以把它当成“路线图 + 白话手册”。

阅读建议：
1. 先读“核心 7 件事速览” -> 知道整体目标。
2. 然后按阶段实现，不要一次全做。
3. 碰到陌生名词，Ctrl/⌘+F 搜：术语索引。

---
## 核心 7 件事速览（一句话版本）
| 序号 | 主题 | 一句话 | 什么时候做 |
|------|------|--------|-----------|
| 1 | 异步解析 | 上传后后台慢慢解析 | 第一阶段立即 |
| 2 | Embedding 主题结构 | 让系统理解“语义 & 主题” | 解析稳定后 |
| 3 | gRPC & 流式输出 | 模型回答边生成边返回 | 聊天体验要提升时 |
| 4 | 语义检索 (RAG) | 用户可自由问整篇 | 用户开始模糊提问时 |
| 5 | 观测性 | 出问题能快速找到原因 | 多服务后必须上 |
| 6 | 安全与鉴权 | 确保内部通信安全 | 外部真实用户之前 |
| 7 | 灰度/回滚 | 新功能小流量验证 | 有线上流量后 |

---
## 为什么要从“单体”走向“分布式”？
好比最初“一家小餐馆一个厨师全做”，客人一多就排队。拆成多个“档口”后：炒菜、烧烤、甜品各自并行，不互相拖慢。你的系统里：解析、嵌入、聊天、检索分开后，慢任务不再卡住实时请求。

如果不拆：
- 大 PDF 解析时聊天会变慢或阻塞。
- 扩容只能整体复制一份，浪费机器。
- 问题出现时不知道卡在“解析”还是“LLM 调用”。

拆完后：
- 可单独扩容“瓶颈模块”。
- 每条用户请求形成一条“调用轨迹”，调试更快。
- 可以渐进式加新功能（主题聚类、推荐、语义检索）。

---
## 目录
1. 分布式演进路线总览
2. 服务划分与职责矩阵
3. 通信协议与模式 (HTTP / gRPC / Streaming / 异步事件)
4. 数据与存储层设计
5. 消息/任务队列选型对比
6. Protobuf 规范与目录结构建议
7. 流式响应与前端集成
8. 向量检索与语义搜索层
9. LLM Orchestrator 策略设计
10. 观测性：日志 / 指标 / Trace / Profiling
11. 安全与合规：认证、授权、密钥管理、审计
12. 配置与 Feature Flag 管理
13. CI/CD 与质量门控
14. 部署拓扑：本地 / 生产 / 弹性扩缩容
15. 服务网格与零信任网络 (可选阶段)
16. 灰度发布与 A/B / 回滚策略
17. 成本与性能优化策略
18. 常见故障场景与排查手册
19. 演进阶段里程碑表
20. 术语索引

---
## 1. 分布式演进路线总览（从“小步拆”到“成熟体系”）
| 阶段 | 目标 | 关键产出 | 风险控制 |
|------|------|----------|----------|
| 0 单体 | 稳定现状 | 基础 HTTP + Python 解析 | 日志简单 |
| 1 异步解析 | 解析不阻塞 | 任务表 + 队列 + Worker | 幂等保障 |
| 2 检索增强 | 语义搜索 | 向量化 + pgvector | 数据一致性监控 |
| 3 流式对话 | 更快响应 | gRPC Streaming + WebSocket | 流断线重试 |
| 4 分层 Orchestrator | 多模型策略 | LLM 选择模块 | 成本跟踪 |
| 5 Observability 完成 | 可定位问题 | OTel + Grafana + Alert | 告警抖动控制 |
| 6 安全与权限 | 零信任 | mTLS + JWT + RBAC | 证书轮换 |
| 7 网格与多副本 | 高可用 | Mesh + HPA | 配置复杂度 |
| 8 灰度策略 | 安全迭代 | Canary + Feature Flag | 流量分配错误 |

---
## 2. 服务划分与职责矩阵（每个“部门”做什么 + 白话）
| 服务 | 职责 | 语言 | 可扩展性 | 状态类型 | 对外暴露 |
|------|------|------|----------|----------|-----------|
| API Gateway (Edge) | 入口、认证、限流、路由 | Go / Envoy | 水平 | 无状态 | Yes |
| Chat Service | 会话管理、上下文聚合 | Go | 水平 | 轻状态 (DB) | 通过 Gateway |
| Document Ingestion | 接收上传、创建任务 | Go | 水平 | 任务状态 | 通过 Gateway |
| PDF Processor Worker | PDF 解析 | Python | 水平 (多进程) | 无状态 (引用任务) | No |
| Embedding Service | 生成文本向量 | Go/Py | 水平 | 无状态 | No |
| Vector Index Service | 相似度搜索 | Go | 水平/读扩展 | 索引数据 | No |
| LLM Orchestrator | 模型路由、重试、流式输出 | Go | 中度 | 无状态 | 仅内部 |
| Auth Service | 用户/Token/OIDC | Go | 水平 | 用户/Token DB | 通过 Gateway |
| Notification / WS | 推送流式 token | Go/Node | 高连接数 | 会话连接 | 通过 Gateway |
| Admin / Control Plane | 配置、策略管理 | Go | 低 | 配置 | 限制访问 |

---
> 白话：服务之间“说话”要选合适的“语言”。快的小事（请求-响应）用 gRPC；耗时工作（解析/嵌入）写“任务单”放队列；需要持续输出（流式回答）开“直播通道”。

## 3. 通信协议与模式（为什么这么选）
| 场景 | 协议 | 说明 |
|------|------|------|
| 外部客户端 -> Gateway | HTTPS (REST/WebSocket) | 标准浏览器兼容 |
| Gateway -> 内部服务 | gRPC | 高性能结构化 |
| Chat -> Orchestrator | gRPC Streaming | token 逐步返回 |
| Ingestion -> Queue | Redis Streams / NATS | 解耦解析速度 |
| Worker -> DB | SQL (Postgres) | 状态更新 |
| Embedding -> VectorIndex | gRPC Batch | 降低往返开销 |
| Events (审计/指标) | NATS / Kafka (后期) | 可回放、订阅统计 |

重试策略（为什么要讲“幂等”）：
- 幂等 = 同一操作重复执行结果不变（例如：查询任务状态）。可以大胆自动重试。
- 非幂等（创建、扣费）需要“幂等键”防止重复创建：客户端先生成一个唯一 id。
- 队列消费者失败超过阈值 -> 投递到 Dead Letter Queue(DLQ) 等人工修复。

---
## 4. 数据与存储层设计（存哪里 + 为什么）
| 数据域 | 存储 | 归档策略 | 备份 |
|--------|------|----------|------|
| 用户/会话 | Postgres | 活跃保留 + 冷归档 | PITR + 日志回放 |
| PDF 原件 | S3/MinIO | 90天后冷存储/压缩 | 版本化开启 |
| 解析结果(JSON) | S3 + Postgres 引用 | 可裁剪冗余段落 | 与原件同策略 |
| 向量 | pgvector | 低价值段落淘汰 | 周期导出 |
| 缓存 (上下文) | Redis | TTL | 无需备份 |
| 事件日志 | NATS/Kafka -> S3 | 归档分区按日 | S3 版本化 |
| 配置/Feature Flag | Postgres / etcd (后期) | 历史版本保留 | 快照 |

一致性策略（口诀：先存后播，会失败补）：
1. 先写 DB 保证落盘，再发事件（防止消息发了但数据没写成功）。
2. 嵌入失败：不要阻塞主流程；标记“待补偿”，后台重试。
3. 大块 JSON 结果放对象存储（S3/MinIO），DB 只存引用路径，减轻 DB 压力。

---
## 5. 消息/任务队列（为什么不用 HTTP 强行等）
解析 PDF、生成 Embedding、批量摘要都不适合阻塞用户；放到队列 = “取号排队”，Worker 慢慢干。
| 维度 | Redis Streams | NATS JetStream | Kafka |
|------|---------------|---------------|-------|
| 入门难度 | 低 | 中 | 高 |
| 持久化 | 有 (AOF/RDB) | 有 | 强 |
| 顺序保证 | 流内局部 | 主题内 | 分区内 |
| 消费模式 | 消费组 | 消费组 | 消费组 |
| 可回放 | 基于ID | 可配置 | 强 |
| 吞吐 | 中 | 高 | 很高 |
| 适用初期 | ✅ | ✅ | 过度 |

结论：已有 Redis → 先用 Redis Streams（入门最省心）。当需要更好的可观测 / 多主题扩展 → 换 NATS。真正大规模海量吞吐 → Kafka。

---
## 6. Protobuf / gRPC（为什么不直接 HTTP JSON）
问题：HTTP+JSON 字段随意、类型模糊；团队大了容易“不小心改坏接口”。
解决：Protobuf 像“接口合同”，写在 `.proto` 文件里，代码自动生成。Go / Python 公用一份协议。

最小例子：
```
syntax = "proto3";
package pdfchat.v1;

message CreateTaskRequest { string document_id = 1; string file_name = 2; }
message CreateTaskResponse { string task_id = 1; string status = 2; }

service DocumentTaskService {
  rpc CreateTask(CreateTaskRequest) returns (CreateTaskResponse);
}
```
未来还会加：流式返回、错误细节、分页等。
```
proto/
  pdfchat/
    common.proto      # 通用消息（Error, Pagination, Timestamp）
    document.proto    # 文档任务、解析接口
    chat.proto        # 聊天消息、流式 token
    embedding.proto   # 嵌入批处理
    search.proto      # 向量检索请求/响应

# 代码生成（示例）
make proto # 调用 buf generate
```
命名建议：
1. 包名带版本：`pdfchat.v1`，升级兼容用 `v2`。
2. 字段编号一旦占用不要复用，删除字段标记 `deprecated = true`。
3. 公共结构（时间戳、错误）放 `common.proto`，避免重复。

---
## 7. 流式响应（为什么“边生成边发”）
LLM 长回答如果等全部完成，用户可能 5~10 秒没反馈；流式让首 token 1 秒内出现，主观体验显著升级。
流程：
1. 前端调用 `POST /chat` 获取 `stream_session_id`。
2. 前端建立 WebSocket：`/chat/stream/{id}`。
3. Gateway 内部发起 gRPC Streaming 到 Orchestrator。
4. 每个 token 推送：`{"type":"token","content":"He"}`。
5. 结束：`{"type":"end","usage":{...}}`。
断线恢复：客户端带上 `last_token_seq` 重连；后端在 Redis 缓存最近 N 个 token 重放缺口。

---
## 8. 向量 / Embedding / 语义搜索（为什么文本要“变成数字”）
问题：用户问“核心贡献是什么”，关键词匹配找不到“意思相近但词不同”的段落。Embedding 把段落 -> 一个高维向量；相似语义 → 向量距离近。
流程：
1. 解析完成 -> 触发“待嵌入”任务 (队列)
2. Embedding Service 批量取若干段落 -> 调用外部 embedding API -> 写入 pgvector：
   表结构：
```
segments(
  id UUID,
  doc_id UUID,
  content TEXT,
  embedding VECTOR(1536),
  meta JSONB,
  created_at TIMESTAMPTZ
)
```
3. 查询：`ORDER BY embedding <=> query_vec LIMIT 8` (`<=>` 是 pgvector 相似度运算符) 拿最接近的段落。
4. 可再 Re-Rank：用小模型对 TopK 重新打分提升精准度。
5. 聚类：把相似段落归类成“主题包”，生成层级摘要 → 更像“导读手册”。

---
## 9. LLM Orchestrator（集中“调用大模型”的大脑）
为什么单独抽出：
| 痛点 | 如果分散各服务 | 抽出后好处 |
|------|----------------|------------|
| 重试/限流 | 每处重复写 | 统一控制 |
| 模型选择 | 各自随意用贵模型 | 动态策略降成本 |
| 统计成本 | 难聚合 | 中央记账表 |
| Prompt 模板 | 各写各的 | 统一模板可迭代 |
| 流式封装 | 重复实现 | 一处实现，共享 |
职责：
- 模型选择：根据上下文长度 / 成本预算 / SLA 选择 GPT-4 / GPT-4o-mini / local。
- Prompt 管线：system + history + user + retrieved context。
- 流控：外部 API 429 时指数退避。
- 成本度量：记录 tokens_in / tokens_out -> 存 Postgres。
- 回滚：模型异常时 fallback 到“次优”模型。

---
## 10. 观测性（怎么知道“慢在哪里”）
层次：日志(点) + 指标(面) + Trace(链)。Trace 像“物流单”，告诉你包裹（请求）一路走了哪些站点。
| 层 | 工具 | 示例指标 |
|----|------|----------|
| 日志 | zap(log)，结构化 JSON | request_id, latency_ms |
| 指标 | Prometheus + OpenTelemetry | parse_duration_seconds |
| Trace | OTel SDK + Jaeger | span: Ingestion->Worker->Embedding |
| Profiling | pprof (Go)，Py-spy (Python) | CPU热点 |
| 报警 | Alertmanager | parse_queue_lag > 阈值 |

采样策略：
- 正常：随机 5%
- 错误 / 超时：100%
- 大批量压测：可降低到 1% 只保留异常

---
## 11. 安全（最少要做的五件事）
1. 所有外部调用走 HTTPS
2. 用户鉴权（JWT）+ 服务间不信任（后期 mTLS）
3. Secrets 不写死：用环境变量 + K8s Secret/SOPS
4. 最小权限：数据库用户不授多余写权限
5. 关键操作日志：删除、导出、权限变更写审计事件
| 领域 | 措施 |
|------|------|
| 认证 | Gateway 验证 JWT (来自 Auth/OIDC) |
| 授权 | 简单 RBAC：角色 -> 资源操作映射 |
| 服务间信任 | mTLS（Mesh 注入） |
| 审计 | 关键操作事件 -> 事件总线 -> S3 归档 |
| 密钥管理 | K8s Secret + SOPS（Git 加密） |
| 输入校验 | Proto / JSON schema 双层 |
| 速率限制 | Envoy/Gateway token bucket |
| 漏洞扫描 | CI 中 Trivy / Grype |

---
## 12. 配置与 Feature Flag（功能开关 ≈ “遥控开关”）
- 运行时不可变核心配置：监听端口、数据库地址（环境变量）。
- 可动态更新：模型权重优先级、限流阈值（存 Postgres + 缓存）。
- Feature Flag 表：`features(service, flag, enabled, rollout_percent)`。
- 灰度：random(seed=user_id) < rollout_percent -> 启用新路径。

---
## 13. CI/CD（自动化流水线 = 质量守门员）
流水线阶段：
1. Lint：Go (golangci-lint), Python (ruff)
2. Test：`go test ./...` + `pytest`
3. Proto：`buf lint` + `buf breaking`
4. Build：多服务 Docker 镜像 (tag=gitsha)
5. Scan：Trivy 镜像漏洞
6. Deploy：Helm (staging) -> Canary (10%) -> Promote
7. Smoke：/healthz, gRPC reflection list services
8. 通知：Slack / 邮件

Gate 条件：测试全绿、严重漏洞=0、proto 无破坏变更。

---
## 14. 部署拓扑（本地和线上差异）
本地：`docker-compose`：Postgres + Redis + (可选) MinIO + API + Worker。
生产 (Kubernetes)：
- Namespace: `edge`, `core`, `data`
- HPA：CPU>70% 或 自定义QPS 指标 scale
- 节点亲和：数据层(stateful) 独立节点池
- 备份：Velero/S3

---
## 15. 服务网格（别太早上）
何时考虑：服务 >= 5，需统一 mTLS、细粒度流控、零侵入指标。
选型：Linkerd (简单) / Istio (复杂特性)。
功能：
- mTLS 自动化
- 请求级重试和熔断
- 金丝雀流量分配（Istio VirtualService）
- 可观测增强（边车 metrics）

---
## 16. 灰度与 A/B（上线别“一刀切”）
策略：
- Canary: 5% -> 25% -> 50% -> 100%
- 按用户哈希：`hash(user_id) % 100 < percent`
- 回滚：检测错误率 / p95 延迟阈值超出立即切回旧版本。
- 存储实验结果：`experiments_results` 表。

---
## 17. 成本与性能（先盯 LLM + Embedding）
| 场景 | 手段 |
|------|------|
| LLM 成本 | Prompt 压缩 / 向量召回截断 / 缓存输出 |
| 向量存储 | 稀疏字段压缩 / 分片表 | 
| 并发解析 | 控制并行度，避免 I/O 抢占 |
| 热点短文本 | 内存缓存最近对话摘要 |
| 外部 API 延迟 | 并发 + 超时 + 退避策略 |

---
## 18. 故障排查（定位顺序：前端 → 网关 → 内部调用 → 任务/队列 → 外部依赖）
| 症状 | 排查顺序 |
|------|----------|
| 解析队列积压 | XLEN -> Worker 日志 -> 单任务耗时 -> 瓶颈线程/CPU |
| 聊天延迟高 | Trace: Chat -> Orchestrator -> LLM | 
| 向量召回少 | 检查 embedding 数量 / 维度 / 向量范数 |
| 内存飙升 | Profiling -> 确认是否缓存泄漏 |
| LLM 429 频繁 | 限速策略 / Token 分配表 / 并发阈值 |

---
## 19. 演进里程碑（完成一个阶段就“存档”一次）
| 里程碑 | 验收标准 | 指标 |
|--------|----------|------|
| M1 异步解析 | 状态机稳定 | 失败率 <2% |
| M2 检索上线 | 语义问答可用 | Top-k 命中率 ≥ 70% |
| M3 流式回复 | 首 token < 1.5s | 用户满意度提升 |
| M4 观测完善 | Trace 覆盖 > 90% | 平均定位时间下降 |
| M5 安全基线 | mTLS + JWT | 未授权访问 0 |
| M6 Canary 能力 | 可 10% 灰度 | 回滚 < 5min |

---
## 20. 术语索引（按需翻阅）
| 术语 | 说明 |
|------|------|
| HPA | Horizontal Pod Autoscaler 水平自动扩容 |
| DLQ | Dead Letter Queue 死信队列 |
| Canary | 金丝雀发布，小流量试运行 |
| Re-Rank | 二次排序模型提升结果相关度 |
| Fallback | 主方案失败后的后备策略 |
| Circuit Breaker | 熔断，防止级联故障 |
| Backoff | 退避策略 (指数延迟重试) |
| OTel | OpenTelemetry 统一观测标准 |
| RBAC | Role-Based Access Control 角色访问控制 |
| PITR | Point-In-Time Recovery 恢复到任意时间点 |

---
## 附录：最小原型命令 (示例)
(仅说明结构，不强制现在就实现)
```bash
# 生成 proto (未来)
buf lint && buf generate

# 启动本地服务
docker compose up -d postgres redis api worker

# 查看解析任务
db=> SELECT id,status FROM document_tasks ORDER BY created_at DESC LIMIT 10;
```

---
---
> 下一步：如果你希望我继续：可以让我 “生成 proto 和骨架” 或 “加 Embedding 聚类示例”。
