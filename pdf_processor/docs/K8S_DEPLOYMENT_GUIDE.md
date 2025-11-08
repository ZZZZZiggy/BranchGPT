# Kubernetes 部署详解

> 本文档深入讲解 PDF Processor 的 Kubernetes 部署脚本，包括 YAML 语法、设计理由、最佳实践和常见陷阱。

---

## 📚 目录

- [架构概览](#架构概览)
- [Deployment 详解](#deployment-详解)
- [ConfigMap 详解](#configmap-详解)
- [Secret 详解](#secret-详解)
- [HPA 详解](#hpa-详解)
- [Service 详解](#service-详解)
- [设计理由](#设计理由)
- [部署流程](#部署流程)
- [常见问题](#常见问题)

---

## 架构概览

### K8s 资源关系图

```
┌─────────────────────────────────────────────────────────┐
│  Kubernetes Cluster                                     │
│                                                         │
│  ┌────────────────────────────────────────────────┐    │
│  │  Namespace: default                            │    │
│  │                                                │    │
│  │  ┌──────────────────────────────────────────┐ │    │
│  │  │  HPA (自动扩缩容)                        │ │    │
│  │  │  - 监控 CPU/内存                         │ │    │
│  │  │  - 控制 Pod 数量 (2-10)                  │ │    │
│  │  └────────────┬─────────────────────────────┘ │    │
│  │               │ 控制                          │    │
│  │               ▼                               │    │
│  │  ┌──────────────────────────────────────────┐ │    │
│  │  │  Deployment: pdf-processor-worker        │ │    │
│  │  │  - replicas: 3 (初始)                    │ │    │
│  │  │  - 滚动更新策略                           │ │    │
│  │  └────────────┬─────────────────────────────┘ │    │
│  │               │ 创建/管理                      │    │
│  │               ▼                               │    │
│  │  ┌──────────────────────────────────────────┐ │    │
│  │  │  ReplicaSet                              │ │    │
│  │  │  - 维护 Pod 副本数                        │ │    │
│  │  └────────────┬─────────────────────────────┘ │    │
│  │               │ 创建                          │    │
│  │               ▼                               │    │
│  │  ┌──────────────────────────────────────────┐ │    │
│  │  │  Pods (3个实例)                          │ │    │
│  │  │  ┌─────────┐ ┌─────────┐ ┌─────────┐    │ │    │
│  │  │  │ Pod-1   │ │ Pod-2   │ │ Pod-3   │    │ │    │
│  │  │  │ Worker  │ │ Worker  │ │ Worker  │    │ │    │
│  │  │  │:50052   │ │:50052   │ │:50052   │    │ │    │
│  │  │  └─────────┘ └─────────┘ └─────────┘    │ │    │
│  │  └──────────────────────────────────────────┘ │    │
│  │               │ 共享配置                       │    │
│  │               ▼                               │    │
│  │  ┌──────────────────────────────────────────┐ │    │
│  │  │  ConfigMap: pdf-processor-config         │ │    │
│  │  │  - Redis URL                             │ │    │
│  │  │  - gRPC 地址                             │ │    │
│  │  │  - 存储配置                               │ │    │
│  │  └──────────────────────────────────────────┘ │    │
│  │               │                               │    │
│  │               ▼                               │    │
│  │  ┌──────────────────────────────────────────┐ │    │
│  │  │  Secret: pdf-processor-secrets           │ │    │
│  │  │  - Redis 密码                            │ │    │
│  │  │  - MinIO 凭证                            │ │    │
│  │  └──────────────────────────────────────────┘ │    │
│  │               │                               │    │
│  │               ▼                               │    │
│  │  ┌──────────────────────────────────────────┐ │    │
│  │  │  Service: pdf-processor-worker           │ │    │
│  │  │  - ClusterIP: None (Headless)            │ │    │
│  │  │  - Port 50052                            │ │    │
│  │  └──────────────────────────────────────────┘ │    │
│  │               │ DNS 解析                      │    │
│  │               ▼                               │    │
│  │  pdf-processor-worker-0.pdf-processor-worker │    │
│  │  pdf-processor-worker-1.pdf-processor-worker │    │
│  │  pdf-processor-worker-2.pdf-processor-worker │    │
│  └────────────────────────────────────────────────┘    │
│                                                         │
│  外部访问（Go 服务）                                      │
│  go-ingest-service → pdf-processor-worker:50052        │
└─────────────────────────────────────────────────────────┘
```

---

## Deployment 详解

### 完整配置解析

```yaml
apiVersion: apps/v1 # ① API 版本：Deployment 使用 apps/v1
kind: Deployment # ② 资源类型：Deployment（管理 Pod 的控制器）
metadata: # ③ 元数据：描述这个资源
  name: pdf-processor-worker # 资源名称
  namespace: default # 命名空间（逻辑隔离）
  labels: # 标签（用于筛选和组织）
    app: pdf-processor
    component: worker

spec: # ④ 规格：定义期望的状态
  replicas: 3 # ⑤ 副本数：运行 3 个 Pod

  strategy: # ⑥ 更新策略
    type: RollingUpdate # 滚动更新（逐步替换）
    rollingUpdate:
      maxSurge: 1 # 更新时最多比 replicas 多 1 个
      maxUnavailable: 1 # 更新时最多有 1 个不可用
```

#### ⑤ 副本数 (replicas)

**为什么是 3？**

```
设计理由：
1. 高可用性：
   - 1 个 Pod 故障 → 还有 2 个可用 ✅
   - 2 个 Pod 故障 → 还有 1 个可用 ✅
   - 3 个全故障 → 极小概率

2. 负载均衡：
   - 3 个 Pod 分摊并发任务
   - 避免单点过载

3. 滚动更新：
   - maxUnavailable: 1 → 更新时至少 2 个可用
   - maxSurge: 1 → 最多 4 个 Pod 同时运行

   更新过程：
   Time 0: [Pod-1] [Pod-2] [Pod-3] ← 旧版本
   Time 1: [Pod-1] [Pod-2] [Pod-3] [Pod-4-new] ← 启动新版本
   Time 2: [Pod-1] [Pod-2] [Pod-4-new] ← 删除旧版本 Pod-3
   Time 3: [Pod-1] [Pod-2] [Pod-4-new] [Pod-5-new]
   Time 4: [Pod-2] [Pod-4-new] [Pod-5-new]
   Time 5: [Pod-4-new] [Pod-5-new] [Pod-6-new] ← 完成

4. 资源效率：
   - 不会太多（浪费资源）
   - 不会太少（单点故障）
```

#### ⑥ 更新策略 (strategy)

**RollingUpdate 详解**

```yaml
strategy:
  type: RollingUpdate # 滚动更新（vs Recreate）
  rollingUpdate:
    maxSurge: 1 # 最多超出 1 个 Pod
    maxUnavailable: 1 # 最多 1 个不可用
```

**对比其他策略**：

| 策略              | maxSurge | maxUnavailable | 更新过程       | 适用场景              |
| ----------------- | -------- | -------------- | -------------- | --------------------- |
| **RollingUpdate** | 1        | 1              | 逐步替换       | ✅ 生产环境（零停机） |
| Recreate          | -        | -              | 全部删除再创建 | ❌ 会有停机时间       |

**工作流程图**：

```
replicas: 3, maxSurge: 1, maxUnavailable: 1

初始状态：
[Pod-1 ✅] [Pod-2 ✅] [Pod-3 ✅]  ← 3 个旧版本运行中

Step 1: 创建 1 个新 Pod（maxSurge: 1）
[Pod-1 ✅] [Pod-2 ✅] [Pod-3 ✅] [Pod-4 🟡]  ← 4 个 Pod（3+1）

Step 2: Pod-4 就绪后，删除 1 个旧 Pod（maxUnavailable: 1）
[Pod-1 ✅] [Pod-2 ✅] [Pod-4 ✅]  ← 3 个 Pod

Step 3: 创建下一个新 Pod
[Pod-1 ✅] [Pod-2 ✅] [Pod-4 ✅] [Pod-5 🟡]  ← 4 个 Pod

Step 4: Pod-5 就绪后，删除旧 Pod
[Pod-2 ✅] [Pod-4 ✅] [Pod-5 ✅]  ← 3 个 Pod

Step 5: 创建最后一个新 Pod
[Pod-2 ✅] [Pod-4 ✅] [Pod-5 ✅] [Pod-6 🟡]  ← 4 个 Pod

Step 6: 完成
[Pod-4 ✅] [Pod-5 ✅] [Pod-6 ✅]  ← 3 个新版本

优势：
✅ 始终有 2-3 个 Pod 可用（从不低于 2）
✅ 零停机部署
✅ 有问题可以快速回滚
```

### 容器配置详解

```yaml
containers:
  - name: worker
    image: your-registry/pdf-processor:latest
    imagePullPolicy: IfNotPresent # ⑦ 镜像拉取策略

    command: ["python", "-m", "tasks.redis_worker"] # ⑧ 启动命令

    resources: # ⑨ 资源限制
      requests: # 请求量（调度保证）
        cpu: "500m" # 0.5 核心
        memory: "1Gi" # 1 GB
      limits: # 限制量（不可超过）
        cpu: "2000m" # 2 核心
        memory: "4Gi" # 4 GB
```

#### ⑦ 镜像拉取策略 (imagePullPolicy)

| 策略             | 行为           | 适用场景                |
| ---------------- | -------------- | ----------------------- |
| **IfNotPresent** | 本地有则用本地 | ✅ 生产环境（节省带宽） |
| Always           | 每次都拉取     | CI/CD 测试              |
| Never            | 只用本地       | 离线环境                |

```yaml
# 为什么选择 IfNotPresent？
imagePullPolicy: IfNotPresent

原因：
1. 减少网络流量：
   - 3 个 Pod × 每次更新拉取 500MB = 1.5 GB
   - 如果本地有缓存，直接使用

2. 加快启动速度：
   - 拉取镜像：30-60 秒
   - 使用缓存：< 1 秒

3. 生产稳定性：
   - 不依赖外部镜像仓库
   - 网络故障不影响重启

注意：必须使用明确的 tag（不能用 :latest）
```

#### ⑧ 启动命令 (command)

```yaml
command: ["python", "-m", "tasks.redis_worker"]

# 等价于在容器内执行：
python -m tasks.redis_worker
```

**为什么不用 Dockerfile 的 CMD？**

```dockerfile
# Dockerfile
CMD ["python", "-m", "tasks.redis_worker"]  # ← 可以被 K8s 覆盖

# K8s deployment.yaml
command: ["python", "-m", "tasks.redis_worker"]  # ← 明确覆盖

优势：
✅ 同一个镜像可以运行不同的命令
   - 同一个镜像，不同的 Deployment：
     deployment-worker.yaml:   command: ["python", "-m", "tasks.redis_worker"]
     deployment-grpc.yaml:      command: ["python", "-m", "tasks.grpc_server"]
     deployment-embedding.yaml: command: ["python", "-m", "tasks.grpc_embedding"]

✅ 更灵活（不需要重新构建镜像）
```

#### ⑨ 资源限制 (resources)

```yaml
resources:
  requests: # ① 调度保证（K8s 调度器会找有足够资源的节点）
    cpu: "500m" # 500 millicores = 0.5 核心
    memory: "1Gi" # 1 GiB
  limits: # ② 硬限制（超过会被限流/杀死）
    cpu: "2000m" # 2 核心（超过会限流 throttle）
    memory: "4Gi" # 4 GiB（超过会 OOMKilled）
```

**CPU 单位**：

```
1 核心 = 1000m (millicores)

500m  = 0.5 核心  ← requests
1000m = 1 核心
2000m = 2 核心    ← limits
```

**为什么 requests < limits？**

```
设计理由：

1. 调度效率：
   Node 有 8 核：
   - 如果 requests = 2 核 → 只能调度 4 个 Pod
   - 如果 requests = 0.5 核 → 可以调度 16 个 Pod

   实际上：
   - 大部分时间 CPU 使用率 < 50%
   - 只有处理 PDF 时需要更多 CPU

   requests: 500m  → 保证最低性能
   limits: 2000m   → 突发需求可以用到 2 核

2. 资源利用率：
   8 核节点，运行 10 个 Pod：
   - requests: 10 × 0.5 = 5 核（调度依据）
   - 实际使用：可能只用了 3 核（闲时）
   - 突发时：某个 Pod 可以用到 2 核（忙时）

   ✅ 提高节点利用率

3. 内存限制：
   requests: 1Gi  → 保证有 1GB 可用
   limits: 4Gi    → 最多用 4GB

   ⚠️ 超过 4GB → OOMKilled（Pod 被杀死并重启）
```

**资源不足会怎样？**

```
场景 1：CPU 达到 limits
Pod CPU: 1.8 → 1.9 → 2.0 ← 限流！
结果：Pod 变慢，但不会被杀死

场景 2：内存达到 limits
Pod 内存: 3.5GB → 3.9GB → 4.1GB ← 超过！
结果：OOMKilled（Pod 被杀死，自动重启）

查看日志：
kubectl describe pod pdf-processor-worker-xxxxx

Events:
  Type     Reason     Message
  ----     ------     -------
  Warning  OOMKilled  Memory limit exceeded (4Gi)
```

### 健康检查详解

```yaml
livenessProbe: # ⑩ 存活探针（判断是否需要重启）
  exec:
    command:
      - python
      - -c
      - "import redis; redis.from_url('$(REDIS_URL)').ping()"
  initialDelaySeconds: 30 # 启动后等待 30 秒
  periodSeconds: 30 # 每 30 秒检查一次
  timeoutSeconds: 5 # 超时 5 秒算失败
  failureThreshold: 3 # 连续失败 3 次才重启

readinessProbe: # ⑪ 就绪探针（判断是否可以接收流量）
  exec:
    command:
      - python
      - -c
      - "import redis; redis.from_url('$(REDIS_URL)').ping()"
  initialDelaySeconds: 10 # 启动后等待 10 秒
  periodSeconds: 10 # 每 10 秒检查一次
  timeoutSeconds: 5
  failureThreshold: 3
```

#### ⑩ liveness vs ⑪ readiness

| 探针               | 目的                  | 失败后果        | 检查频率    |
| ------------------ | --------------------- | --------------- | ----------- |
| **livenessProbe**  | 判断是否**死锁/崩溃** | 重启 Pod        | 低频（30s） |
| **readinessProbe** | 判断是否**准备好**    | 从 Service 移除 | 高频（10s） |

**工作流程图**：

```
Pod 启动流程：

Time 0s:  Pod 创建
Time 5s:  容器启动
          ├─► Python 进程启动
          └─► 导入模块

Time 10s: readinessProbe 第一次检查
          ├─► 执行：redis.ping()
          └─► 成功 ✅ → Pod 标记为 Ready

          Service 开始路由流量到这个 Pod

Time 30s: livenessProbe 第一次检查
          ├─► 执行：redis.ping()
          └─► 成功 ✅ → Pod 正常

Time 40s: readinessProbe 定期检查（每 10s）
Time 60s: livenessProbe 定期检查（每 30s）
...

异常情况：

Time 120s: readinessProbe 失败（Redis 连接超时）
          └─► Service 停止路由流量到这个 Pod
          └─► 但 Pod 不重启（可能临时网络问题）

Time 130s: readinessProbe 重试
          └─► 仍然失败

Time 140s: readinessProbe 重试
          └─► 成功 ✅ → Pod 重新加入 Service

如果 livenessProbe 连续失败 3 次：
Time 150s: livenessProbe 失败 (1/3)
Time 180s: livenessProbe 失败 (2/3)
Time 210s: livenessProbe 失败 (3/3)
          └─► Kubelet 杀死容器并重启 🔄
```

**为什么用 Redis ping 作为健康检查？**

```python
# 健康检查命令
python -c "import redis; redis.from_url('$(REDIS_URL)').ping()"

设计理由：
1. 检查核心依赖：
   ✅ Redis 可达 → Worker 可以正常工作
   ❌ Redis 不可达 → Worker 无法获取任务

2. 快速响应：
   - Redis ping 通常 < 10ms
   - 不会阻塞太久

3. 全面性：
   - 检查网络连接
   - 检查 Redis 服务状态
   - 检查 Python 环境

替代方案：
# ❌ 不好：只检查进程存在
command: ["pgrep", "python"]
# 问题：进程存在但可能死锁

# ✅ 更好：检查核心依赖
command: ["python", "-c", "import redis; redis.ping()"]
```

### 卷挂载详解

```yaml
volumeMounts: # ⑫ 容器内的挂载点
  - name: temp-storage
    mountPath: /tmp/pdf_processor # 容器内路径

volumes: # ⑬ Pod 级别的卷定义
  - name: temp-storage
    emptyDir: # 临时卷（Pod 删除时清空）
      sizeLimit: 10Gi # 最多 10GB
```

#### ⑫⑬ 卷类型选择

**为什么用 emptyDir？**

```yaml
emptyDir:
  sizeLimit: 10Gi

特点：
1. 生命周期：
   - Pod 创建时创建
   - Pod 删除时清空
   - ✅ 适合临时文件

2. 位置：
   - 节点本地磁盘
   - ✅ 访问速度快

3. 共享：
   - 同一个 Pod 的多个容器可以共享
   - ✅ 适合临时数据交换

工作流程：
1. 从 S3 下载 PDF → /tmp/pdf_processor/task_123.pdf
2. 处理 PDF
3. 上传结果到 Go
4. 删除临时文件
5. Pod 重启/删除 → emptyDir 自动清空
```

**对比其他卷类型**：

| 卷类型                | 持久性          | 共享性 | 适用场景              |
| --------------------- | --------------- | ------ | --------------------- |
| **emptyDir**          | 临时（Pod 级）  | Pod 内 | ✅ 临时文件、缓存     |
| hostPath              | 永久（Node 级） | 同节点 | ❌ 不推荐（不可移植） |
| persistentVolumeClaim | 永久（集群级）  | 跨 Pod | 数据库、文件存储      |
| configMap             | 只读            | 跨 Pod | 配置文件              |

**为什么设置 sizeLimit: 10Gi？**

```
资源保护：

场景：处理 500 页 PDF
- 下载的 PDF：100 MB
- 解析后的文本：50 MB
- 临时处理文件：200 MB
- 总计：~350 MB per task

如果同时处理 10 个任务：
350 MB × 10 = 3.5 GB ✅ 远低于 10GB

如果没有限制：
- 恶意/错误的任务可能写入 100GB
- 填满节点磁盘
- 影响其他 Pod

sizeLimit: 10Gi → 保护节点磁盘不被耗尽
```

---

## ConfigMap 详解

### 为什么需要 ConfigMap？

```
传统方式：
┌────────────────────────────────────┐
│  Dockerfile                        │
│  ENV REDIS_URL="redis://..."      │  ← 硬编码
│  ENV BUCKET_ENDPOINT="minio..."   │  ← 修改需要重新构建
└────────────────────────────────────┘

ConfigMap 方式：
┌────────────────────────────────────┐
│  ConfigMap                         │
│  redis_url: "redis://..."          │  ← 外部配置
│  bucket_endpoint: "minio..."       │  ← 修改无需重建镜像
└────────────────────────────────────┘
        │ 注入
        ▼
┌────────────────────────────────────┐
│  Pod                               │
│  env:                              │
│    - name: REDIS_URL               │
│      valueFrom:                    │
│        configMapKeyRef: ...        │
└────────────────────────────────────┘

优势：
✅ 配置与代码分离
✅ 无需重建镜像
✅ 不同环境使用不同配置
```

### ConfigMap 配置详解

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: pdf-processor-config
  namespace: default
data:
  # Redis 配置
  redis_url: "redis://redis-service:6379" # ① K8s 内部 DNS
  redis_queue_name: "queue:upload_tasks"

  # gRPC 配置
  go_grpc_ingest_addr: "go-ingest-service:50051" # ② 服务发现
  grpc_server_port: "50052"

  # 存储配置
  bucket_endpoint: "minio-service:9000" # ③ MinIO 地址
  bucket_name: "pdf-uploads"
```

#### ① K8s 内部 DNS

```yaml
redis_url: "redis://redis-service:6379"
           ^^^^^^^^  ^^^^^^^^^^^^^
           协议       K8s Service 名称

DNS 解析：
redis-service → 10.96.123.45 (ClusterIP)

完整 DNS 名称：
- 同命名空间：redis-service
- 跨命名空间：redis-service.default.svc.cluster.local
  └─────┬──────┘ └──┬───┘ └┬┘ └────┬─────┘
      服务名      命名空间  类型  集群域
```

**为什么不用 IP 地址？**

```yaml
# ❌ 错误：硬编码 IP
redis_url: "redis://10.96.123.45:6379"
问题：
- IP 可能变化（Service 重建）
- 不同环境 IP 不同

# ✅ 正确：使用服务名
redis_url: "redis://redis-service:6379"
优势：
- K8s 自动解析
- 跨环境通用
- 支持负载均衡
```

#### ② 服务发现

```yaml
go_grpc_ingest_addr: "go-ingest-service:50051"

工作流程：
1. Python Worker 启动
2. 读取环境变量 GO_GRPC_INGEST_ADDR
3. 创建 gRPC 连接
4. K8s DNS 解析 go-ingest-service → IP
5. 建立连接
```

### 使用 ConfigMap

```yaml
# 在 Deployment 中引用
env:
  - name: REDIS_URL # ① 环境变量名
    valueFrom:
      configMapKeyRef: # ② 从 ConfigMap 读取
        name: pdf-processor-config # ConfigMap 名称
        key: redis_url # ConfigMap 中的 key
```

**更新 ConfigMap**：

```bash
# 方式 1：直接编辑
kubectl edit configmap pdf-processor-config

# 方式 2：重新应用
kubectl apply -f configmap.yaml

# ⚠️ 注意：需要重启 Pod 才能生效
kubectl rollout restart deployment pdf-processor-worker
```

---

## Secret 详解

### Secret vs ConfigMap

| 特性     | ConfigMap                | Secret                              |
| -------- | ------------------------ | ----------------------------------- |
| **用途** | 普通配置                 | 敏感信息                            |
| **存储** | 明文                     | Base64 编码                         |
| **适用** | URL、端口、参数          | 密码、Token、证书                   |
| **查看** | `kubectl get cm -o yaml` | `kubectl get secret -o yaml` (编码) |

### Secret 配置详解

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: pdf-processor-secrets
  namespace: default
type: Opaque # ① 通用 Secret 类型
stringData: # ② 使用 stringData（明文输入，K8s 自动编码）
  redis_password: "your-redis-password"
  bucket_access_id: "minioadmin"
  bucket_access_key: "minioadmin"
```

#### ① Secret 类型

| 类型                                | 用途                | 示例         |
| ----------------------------------- | ------------------- | ------------ |
| **Opaque**                          | 通用（默认）        | 密码、Token  |
| kubernetes.io/dockerconfigjson      | Docker 镜像拉取凭证 | 私有镜像仓库 |
| kubernetes.io/tls                   | TLS 证书            | HTTPS        |
| kubernetes.io/service-account-token | ServiceAccount      | K8s API 认证 |

#### ② stringData vs data

```yaml
# 方式 1：stringData（推荐）
stringData:
  password: "my-secret-password"  # ← 明文
  # K8s 自动 Base64 编码

# 方式 2：data（手动编码）
data:
  password: "bXktc2VjcmV0LXBhc3N3b3Jk"  # ← Base64 编码
  # echo -n "my-secret-password" | base64

推荐使用 stringData：
✅ 更直观
✅ 不易出错
✅ K8s 自动处理编码
```

### 使用 Secret

```yaml
env:
  - name: REDIS_PASSWORD
    valueFrom:
      secretKeyRef: # 从 Secret 读取
        name: pdf-processor-secrets
        key: redis_password
        optional: true # ③ 可选（不存在也不报错）
```

#### ③ optional 参数

```yaml
optional: true  # 可选的 Secret

场景：
- Redis 可能没有密码（开发环境）
- Redis 有密码（生产环境）

如果 optional: false（默认）：
- Secret 不存在 → Pod 启动失败 ❌

如果 optional: true：
- Secret 不存在 → 环境变量为空字符串
- Pod 正常启动 ✅
```

### Secret 安全最佳实践

```bash
# ❌ 不要提交到 Git
git add k8s/secret.yaml  # 危险！

# ✅ 使用 .gitignore
echo "k8s/secret.yaml" >> .gitignore

# ✅ 使用 Sealed Secrets（加密）
# 1. 安装 Sealed Secrets Controller
kubectl apply -f https://github.com/bitnami-labs/sealed-secrets/releases/download/v0.18.0/controller.yaml

# 2. 创建 Sealed Secret
kubeseal --format yaml < secret.yaml > sealed-secret.yaml

# 3. 提交加密后的文件
git add sealed-secret.yaml  # 安全 ✅

# ✅ 使用 External Secrets Operator
# 从外部密钥管理系统（AWS Secrets Manager、Vault）同步
```

---

## HPA 详解

### 自动扩缩容原理

```
HPA (Horizontal Pod Autoscaler) 工作流程：

Step 1: 监控指标
        ├─► Metrics Server 收集 Pod CPU/内存
        ├─► 自定义指标（Prometheus）
        └─► 计算平均值

Step 2: 计算期望副本数
        desiredReplicas = ceil(currentReplicas × (currentMetric / targetMetric))

        示例：
        - 当前副本数：3
        - 当前 CPU：80%
        - 目标 CPU：70%
        - 期望副本数 = ceil(3 × (80 / 70)) = ceil(3.43) = 4

Step 3: 调整副本数
        ├─► 扩容：创建新 Pod
        └─► 缩容：删除空闲 Pod
```

### HPA 配置详解

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: pdf-processor-worker-hpa
spec:
  scaleTargetRef: # ① 监控目标
    apiVersion: apps/v1
    kind: Deployment
    name: pdf-processor-worker

  minReplicas: 2 # ② 最小副本数
  maxReplicas: 10 # ③ 最大副本数

  metrics: # ④ 扩缩容指标
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70 # CPU 70% 触发

    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80 # 内存 80% 触发
```

#### ②③ 副本数范围

```yaml
minReplicas: 2
maxReplicas: 10

设计理由：

minReplicas: 2
- 为什么不是 1？
  └─► 高可用：1 个 Pod 故障，还有 1 个可用
- 为什么不是 3？
  └─► 成本：闲时不需要 3 个

maxReplicas: 10
- 为什么是 10？
  └─► 资源限制：每个 Pod 最多 4GB，10 个 = 40GB
  └─► 并发处理：10 个 Pod 足够处理高峰

副本数变化示例：
Night (00:00-06:00):  2 replicas  ← 低流量
Morning (06:00-09:00): 3-4 replicas  ← 逐渐增加
Peak (09:00-18:00):   6-8 replicas  ← 高峰
Evening (18:00-24:00): 3-5 replicas  ← 下降
```

#### ④ 扩缩容指标

```yaml
metrics:
  # 指标 1：CPU
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70  # 70%

# 计算方式：
所有 Pod 的 CPU 平均值 > 70% → 扩容
所有 Pod 的 CPU 平均值 < 70% → 缩容

示例：
Pod-1 CPU: 80%
Pod-2 CPU: 75%
Pod-3 CPU: 65%
平均：(80 + 75 + 65) / 3 = 73.3% > 70% → 扩容
```

**为什么 CPU 是 70%，内存是 80%？**

```
CPU: 70%
- 留 30% buffer 用于突发流量
- CPU 可以限流（throttle），不会导致崩溃
- ✅ 宁可提前扩容

内存: 80%
- 内存不能限流，超过会 OOMKilled
- 80% 已经比较高，但内存增长通常较慢
- ✅ 平衡扩容频率和安全性

调优建议：
- CPU 密集型：降低到 60%
- 内存密集型：降低到 70%
- 成本敏感：提高到 80-90%（风险增加）
```

### 扩缩容行为控制

```yaml
behavior: # ⑤ 扩缩容行为
  scaleDown: # 缩容行为
    stabilizationWindowSeconds: 300 # 5 分钟稳定期
    policies:
      - type: Percent
        value: 50 # 每次最多缩容 50%
        periodSeconds: 60 # 每分钟

  scaleUp: # 扩容行为
    stabilizationWindowSeconds: 60 # 1 分钟稳定期
    policies:
      - type: Percent
        value: 100 # 每次最多扩容 100%（翻倍）
        periodSeconds: 30 # 每 30 秒
      - type: Pods
        value: 2 # 或每次最多增加 2 个
        periodSeconds: 30
```

#### ⑤ 为什么扩容快、缩容慢？

```
设计哲学：
❗ 扩容要快（避免服务过载）
❗ 缩容要慢（避免频繁抖动）

扩容策略（激进）：
- stabilizationWindowSeconds: 60  # 1 分钟观察
- 每 30 秒可以翻倍
- 流量突增时快速响应

缩容策略（保守）：
- stabilizationWindowSeconds: 300  # 5 分钟观察
- 每分钟最多缩容 50%
- 避免过早缩容导致再次扩容

时间线示例：

扩容过程（快）：
Time 0:   3 replicas, CPU 80%
Time 30s: 观察到 CPU 持续高于 70%
Time 60s: 扩容到 5 replicas (3 + 2)  ← 快速响应
Time 90s: CPU 仍高
Time 120s: 扩容到 7 replicas (5 + 2)

缩容过程（慢）：
Time 0:   7 replicas, CPU 50%
Time 5m:  观察 5 分钟，CPU 持续低于 70%
Time 6m:  缩容到 6 replicas (7 × 50% = 3.5 → 向上取整到 6)
Time 11m: 观察 5 分钟
Time 12m: 缩容到 5 replicas

避免抖动：
❌ 错误：快速缩容
Time 0: 10 replicas
Time 1m: 5 replicas  ← 缩容过快
Time 2m: 流量回升，CPU 飙升
Time 3m: 10 replicas ← 又要扩容（浪费）

✅ 正确：缓慢缩容
Time 0: 10 replicas
Time 5m: 9 replicas  ← 缓慢观察
Time 10m: 8 replicas
Time 15m: 稳定在 7-8 replicas
```

### 基于自定义指标（KEDA）

```yaml
# 可选：基于 Redis 队列长度扩缩容
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: pdf-processor-worker-scaler
spec:
  scaleTargetRef:
    name: pdf-processor-worker

  minReplicaCount: 2
  maxReplicaCount: 20 # 更高的上限

  triggers:
    - type: redis # ⑥ Redis 触发器
      metadata:
        address: redis-service:6379
        listName: queue:upload_tasks
        listLength: "5" # 队列 > 5 个任务
        activationListLength: "1" # 至少 1 个任务才激活
```

#### ⑥ 为什么用队列长度而不是 CPU？

```
CPU/内存指标的局限：
- 反应滞后：CPU 高时可能已经积压很多任务
- 不准确：空闲等待也可能 CPU 低

队列长度指标的优势：
✅ 直接反映工作量
✅ 提前预测（队列增长 → 扩容）
✅ 精确控制（每 5 个任务增加 1 个 Pod）

计算示例：
desiredReplicas = ceil(queueLength / listLength)

队列 20 个任务：
desiredReplicas = ceil(20 / 5) = 4 个 Pod

队列 50 个任务：
desiredReplicas = ceil(50 / 5) = 10 个 Pod
```

---

## Service 详解

### Headless Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: pdf-processor-worker
spec:
  type: ClusterIP
  clusterIP: None # ⑦ Headless Service
  ports:
    - name: grpc-apikey
      port: 50052
      targetPort: 50052
  selector:
    app: pdf-processor
    component: worker
```

#### ⑦ 为什么用 Headless Service？

```yaml
clusterIP: None  # Headless Service

普通 Service：
┌────────────────────────────────────┐
│  Service: pdf-processor-worker     │
│  ClusterIP: 10.96.123.45          │
│  ├─► 负载均衡到所有 Pod            │
│  ├─► Pod-1: 10.244.1.5:50052      │
│  ├─► Pod-2: 10.244.2.8:50052      │
│  └─► Pod-3: 10.244.3.12:50052     │
└────────────────────────────────────┘

Go 连接：
grpc.Dial("pdf-processor-worker:50052")
└─► 连接到 10.96.123.45
    └─► 随机转发到某个 Pod

Headless Service：
┌────────────────────────────────────┐
│  Service: pdf-processor-worker     │
│  ClusterIP: None                   │
│  DNS 返回所有 Pod IP：              │
│  ├─► Pod-1: 10.244.1.5:50052      │
│  ├─► Pod-2: 10.244.2.8:50052      │
│  └─► Pod-3: 10.244.3.12:50052     │
└────────────────────────────────────┘

Go 连接：
grpc.Dial("pdf-processor-worker:50052")
└─► DNS 返回 [10.244.1.5, 10.244.2.8, 10.244.3.12]
    └─► gRPC 客户端自己做负载均衡
```

**为什么 gRPC 更适合 Headless Service？**

```
gRPC 特点：
1. 长连接：
   - HTTP/2 持久连接
   - 一次连接，多次请求

2. 客户端负载均衡：
   - gRPC 内置负载均衡
   - 支持多种策略（round_robin, pick_first）

普通 Service 的问题：
┌─────────────────────────────────────────┐
│  Go 建立 1 个连接到 Service             │
│  └─► Service 转发到 Pod-1               │
│      └─► 所有请求都发往 Pod-1 ❌        │
│          (HTTP/2 连接复用)              │
└─────────────────────────────────────────┘

结果：
- Pod-1: 100 requests/s  ← 过载
- Pod-2: 0 requests/s    ← 空闲
- Pod-3: 0 requests/s    ← 空闲

Headless Service 的优势：
┌─────────────────────────────────────────┐
│  Go 获取所有 Pod IP                     │
│  ├─► 建立连接到 Pod-1                   │
│  ├─► 建立连接到 Pod-2                   │
│  └─► 建立连接到 Pod-3                   │
│      └─► gRPC 客户端轮询分发 ✅         │
└─────────────────────────────────────────┘

结果：
- Pod-1: 33 requests/s  ← 均衡
- Pod-2: 34 requests/s  ← 均衡
- Pod-3: 33 requests/s  ← 均衡
```

---

## 设计理由总结

### 1. 为什么用 Deployment 而不是 StatefulSet？

| 特性         | Deployment | StatefulSet |
| ------------ | ---------- | ----------- |
| **Pod 名称** | 随机后缀   | 固定序号    |
| **网络标识** | 不稳定     | 稳定        |
| **存储**     | 共享或临时 | 独立 PVC    |
| **适用场景** | 无状态服务 | 有状态服务  |

```yaml
# Deployment Pod 名称（随机）
pdf-processor-worker-7d9f8b6c5-xk9jm
pdf-processor-worker-7d9f8b6c5-p4d2w

# StatefulSet Pod 名称（固定序号）
pdf-processor-worker-0
pdf-processor-worker-1

我们选择 Deployment 因为：
✅ Worker 是无状态的
✅ 任何 Pod 都可以处理任何任务
✅ 不需要固定的网络标识
✅ 不需要独立的持久化存储
```

### 2. 为什么用 ConfigMap + Secret 而不是硬编码？

```
12-Factor App 原则：
III. Config: 严格分离配置和代码

优势：
✅ 同一个镜像，多个环境
   - dev: redis-dev:6379
   - staging: redis-staging:6379
   - prod: redis-prod:6379

✅ 修改配置无需重建镜像
   kubectl edit configmap pdf-processor-config
   kubectl rollout restart deployment pdf-processor-worker

✅ 安全：密码不在代码中
```

### 3. 为什么用 HPA 而不是固定副本数？

```
固定副本数的问题：
- 高峰期：3 个 Pod 不够，服务慢 ❌
- 低峰期：3 个 Pod 太多，浪费资源 ❌

HPA 的优势：
✅ 自动适应流量
✅ 节省成本（低峰期缩容）
✅ 保证性能（高峰期扩容）

成本对比（按 AWS 价格）：
固定 5 个 Pod × 24 小时 × 30 天 = 3600 Pod·小时
HPA：
- 2-10 个 Pod 动态调整
- 平均 4 个 Pod × 24 × 30 = 2880 Pod·小时
- 节省：(3600 - 2880) / 3600 = 20%
```

---

## 部署流程

### 完整部署步骤

```bash
# Step 1: 创建命名空间（可选）
kubectl create namespace pdf-processor

# Step 2: 部署 ConfigMap
kubectl apply -f k8s/configmap.yaml

# Step 3: 部署 Secret
kubectl apply -f k8s/secret.yaml

# Step 4: 部署 Deployment
kubectl apply -f k8s/deployment.yaml

# Step 5: 部署 HPA
kubectl apply -f k8s/hpa.yaml

# Step 6: 验证部署
kubectl get pods -l app=pdf-processor
kubectl get hpa pdf-processor-worker-hpa
kubectl get svc pdf-processor-worker

# Step 7: 查看日志
kubectl logs -f deployment/pdf-processor-worker

# Step 8: 测试健康检查
kubectl exec -it pdf-processor-worker-xxxxx -- \
  python -c "import redis; print(redis.from_url('redis://redis-service:6379').ping())"
```

### 滚动更新

```bash
# 更新镜像
kubectl set image deployment/pdf-processor-worker \
  worker=your-registry/pdf-processor:v2.0

# 查看更新状态
kubectl rollout status deployment/pdf-processor-worker

# 暂停更新
kubectl rollout pause deployment/pdf-processor-worker

# 继续更新
kubectl rollout resume deployment/pdf-processor-worker

# 回滚到上一个版本
kubectl rollout undo deployment/pdf-processor-worker

# 回滚到指定版本
kubectl rollout history deployment/pdf-processor-worker
kubectl rollout undo deployment/pdf-processor-worker --to-revision=2
```

### 扩缩容

```bash
# 手动扩容
kubectl scale deployment pdf-processor-worker --replicas=5

# 查看 HPA 状态
kubectl get hpa
# NAME                         REFERENCE                         TARGETS   MINPODS   MAXPODS   REPLICAS
# pdf-processor-worker-hpa     Deployment/pdf-processor-worker   45%/70%   2         10        3

# 禁用 HPA（手动控制）
kubectl delete hpa pdf-processor-worker-hpa

# 重新启用 HPA
kubectl apply -f k8s/hpa.yaml
```

---

## 常见问题

### Q1: Pod 启动失败

```bash
# 查看 Pod 状态
kubectl get pods
# NAME                                    READY   STATUS             RESTARTS
# pdf-processor-worker-7d9f8b6c5-xk9jm   0/1     CrashLoopBackOff   5

# 查看详细信息
kubectl describe pod pdf-processor-worker-7d9f8b6c5-xk9jm

# 常见原因：
# 1. 镜像拉取失败
Events:
  Failed to pull image: ImagePullBackOff

解决：检查镜像名称和凭证

# 2. ConfigMap/Secret 不存在
Events:
  Error: configmap "pdf-processor-config" not found

解决：先部署 ConfigMap/Secret

# 3. 健康检查失败
Events:
  Liveness probe failed

解决：检查 Redis 连接
```

### Q2: HPA 不扩容

```bash
# 检查 Metrics Server
kubectl get apiservice v1beta1.metrics.k8s.io
# NAME                     SERVICE                      AVAILABLE
# v1beta1.metrics.k8s.io   kube-system/metrics-server   True

# 如果不可用，安装 Metrics Server
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# 查看 Pod 指标
kubectl top pods
# NAME                                    CPU(cores)   MEMORY(bytes)
# pdf-processor-worker-7d9f8b6c5-xk9jm   450m         1200Mi

# 查看 HPA 详细信息
kubectl describe hpa pdf-processor-worker-hpa
```

### Q3: 配置修改不生效

```bash
# 原因：Pod 启动时读取环境变量，运行中不会更新

# 解决：重启 Pod
kubectl rollout restart deployment/pdf-processor-worker

# 或删除 Pod（会自动重建）
kubectl delete pod pdf-processor-worker-xxxxx
```

### Q4: 内存溢出 (OOMKilled)

```bash
# 查看事件
kubectl describe pod pdf-processor-worker-xxxxx
Events:
  Type     Reason     Message
  ----     ------     -------
  Warning  OOMKilled  Container killed due to OOM (Out Of Memory)

# 解决方案：
# 1. 增加内存限制
resources:
  limits:
    memory: "8Gi"  # 从 4Gi 增加到 8Gi

# 2. 优化代码（使用流式处理）
# 3. 降低并发数（减少同时处理的 PDF 数量）
```

---

## 最佳实践

### 1. 资源配置

```yaml
# ✅ 推荐：requests < limits
resources:
  requests:
    cpu: "500m"
    memory: "1Gi"
  limits:
    cpu: "2000m"
    memory: "4Gi"

# ❌ 不推荐：requests = limits
resources:
  requests:
    cpu: "2000m"    # 浪费资源
    memory: "4Gi"
  limits:
    cpu: "2000m"
    memory: "4Gi"
```

### 2. 健康检查

```yaml
# ✅ 推荐：liveness 和 readiness 都配置
livenessProbe:
  initialDelaySeconds: 30 # 给足够的启动时间
  periodSeconds: 30 # 低频检查

readinessProbe:
  initialDelaySeconds: 10
  periodSeconds: 10 # 高频检查

# ❌ 不推荐：只配置一个
```

### 3. 更新策略

```yaml
# ✅ 推荐：滚动更新
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxSurge: 1
    maxUnavailable: 1

# ❌ 不推荐：全部重建（有停机时间）
strategy:
  type: Recreate
```

### 4. 日志管理

```bash
# ✅ 推荐：结构化日志
logger.info("Task started", extra={"task_id": task_id, "action": "start"})

# ❌ 不推荐：纯文本日志
print(f"Task {task_id} started")

# 使用日志聚合
kubectl logs -f deployment/pdf-processor-worker | jq .
```

---

## 总结

K8s 部署的核心理念：

1. **声明式配置**：描述期望状态，K8s 自动维护
2. **无状态设计**：任何 Pod 可替换
3. **自动化运维**：自动扩缩容、自愈、滚动更新
4. **关注分离**：配置、密钥、代码分离

关键设计决策：

- ✅ Deployment：无状态服务
- ✅ emptyDir：临时文件存储
- ✅ ConfigMap/Secret：配置管理
- ✅ HPA：自动扩缩容
- ✅ Headless Service：gRPC 负载均衡
- ✅ RollingUpdate：零停机部署

---

<div align="center">

**[← 返回主文档](../readme.md)** | **[并发详解](./CONCURRENCY_DEEP_DIVE.md)**

</div>
