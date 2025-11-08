# Kubernetes 部署指南

## 📦 部署架构

```
┌─────────────────────────────────────────────────────────────────┐
│                      Kubernetes Cluster                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │  PDF Processor Worker Deployment (replicas: 3)           │  │
│  ├──────────────────────────────────────────────────────────┤  │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐                  │  │
│  │  │ Pod 1   │  │ Pod 2   │  │ Pod 3   │  ← 并发处理      │  │
│  │  │ Worker  │  │ Worker  │  │ Worker  │                  │  │
│  │  │ gRPC:   │  │ gRPC:   │  │ gRPC:   │                  │  │
│  │  │ 50052   │  │ 50052   │  │ 50052   │                  │  │
│  │  └────┬────┘  └────┬────┘  └────┬────┘                  │  │
│  │       │            │            │                        │  │
│  │       └────────────┴────────────┘                        │  │
│  │                    │                                      │  │
│  └────────────────────┼──────────────────────────────────────┘  │
│                       │                                          │
│  ┌────────────────────▼──────────────────────────────────────┐  │
│  │  Headless Service: pdf-processor-worker (port 50052)     │  │
│  └────────────────────┬──────────────────────────────────────┘  │
│                       │                                          │
│                       ▲                                          │
│                       │ Go 调用任意 Pod                          │
│  ┌────────────────────┴──────────────────────────────────────┐  │
│  │  Go Ingest Service (port 50051)                           │  │
│  └────────────────────┬──────────────────────────────────────┘  │
│                       │                                          │
│  ┌────────────────────▼──────────────────────────────────────┐  │
│  │  Redis (queue:upload_tasks)                               │  │
│  └────────────────────┬──────────────────────────────────────┘  │
│                       │                                          │
│  ┌────────────────────▼──────────────────────────────────────┐  │
│  │  MinIO / S3 (PDF Storage)                                 │  │
│  └───────────────────────────────────────────────────────────┘  │
│                                                                   │
└─────────────────────────────────────────────────────────────────┘
```

## 🚀 快速开始

### 1. 构建 Docker 镜像

```bash
# 在项目根目录
docker build -t your-registry/pdf-processor:v1.0.0 .
docker push your-registry/pdf-processor:v1.0.0
```

### 2. 创建命名空间（可选）

```bash
kubectl create namespace pdf-processing
```

### 3. 部署配置

```bash
# 按顺序部署
kubectl apply -f k8s/configmap.yaml
kubectl apply -f k8s/secret.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/hpa.yaml  # 可选：自动扩缩容
```

### 4. 验证部署

```bash
# 查看 Pod 状态
kubectl get pods -l app=pdf-processor

# 查看日志
kubectl logs -f deployment/pdf-processor-worker

# 查看服务
kubectl get svc pdf-processor-worker
```

## ⚙️ 并发控制策略

### 方案 1: 固定副本数（简单）

修改 `deployment.yaml`:

```yaml
spec:
  replicas: 5 # 固定 5 个 worker
```

**适用场景**:

- 负载稳定
- 预算固定

### 方案 2: 基于 CPU/内存自动扩缩容（推荐）

使用 `hpa.yaml` 的 HPA 配置：

```yaml
minReplicas: 2
maxReplicas: 10
metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        averageUtilization: 70
```

**适用场景**:

- 负载波动
- 成本优化

### 方案 3: 基于 Redis 队列长度扩缩容（最优）

需要安装 KEDA：

```bash
# 安装 KEDA
kubectl apply -f https://github.com/kedacore/keda/releases/download/v2.12.0/keda-2.12.0.yaml
```

使用 `hpa.yaml` 的 ScaledObject 配置：

```yaml
triggers:
  - type: redis
    metadata:
      listLength: "5" # 队列 > 5 时扩容
```

**适用场景**:

- 精确控制
- 高峰低谷差异大

## 📊 监控指标

### Prometheus Metrics (推荐添加)

在代码中添加 Prometheus 指标：

```python
# requirements.txt
prometheus-client>=0.19.0

# tasks/redis_worker.py
from prometheus_client import Counter, Gauge, Histogram, start_http_server

# 指标定义
tasks_processed = Counter('pdf_tasks_processed_total', 'Total tasks processed')
tasks_failed = Counter('pdf_tasks_failed_total', 'Total tasks failed')
processing_time = Histogram('pdf_processing_duration_seconds', 'Processing time')
queue_length = Gauge('pdf_queue_length', 'Current queue length')

# 在 main loop 启动 metrics server
start_http_server(8000)  # Prometheus scrape endpoint
```

修改 `deployment.yaml` 添加 metrics 端口：

```yaml
ports:
  - name: metrics
    containerPort: 8000
    protocol: TCP
```

### Grafana Dashboard

关键指标：

- 任务处理速率
- 任务失败率
- 队列长度
- Worker CPU/内存使用
- gRPC 调用延迟

## 🔧 配置调优

### 资源配置建议

#### 小负载（<100 任务/天）

```yaml
replicas: 2
resources:
  requests:
    cpu: "250m"
    memory: "512Mi"
  limits:
    cpu: "1000m"
    memory: "2Gi"
```

#### 中等负载（100-1000 任务/天）

```yaml
replicas: 3-5
resources:
  requests:
    cpu: "500m"
    memory: "1Gi"
  limits:
    cpu: "2000m"
    memory: "4Gi"
```

#### 高负载（>1000 任务/天）

```yaml
minReplicas: 5
maxReplicas: 20
resources:
  requests:
    cpu: "1000m"
    memory: "2Gi"
  limits:
    cpu: "4000m"
    memory: "8Gi"
```

### Redis 连接池配置

修改 `redis_worker.py`:

```python
redis_client = redis.from_url(
    config.REDIS_URL,
    password=config.REDIS_PASSWORD,
    max_connections=50,  # 连接池大小
    socket_keepalive=True,
    socket_connect_timeout=5,
)
```

## 🛡️ 生产环境最佳实践

### 1. 资源隔离

使用 ResourceQuota 和 LimitRange：

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: pdf-processor-quota
  namespace: pdf-processing
spec:
  hard:
    requests.cpu: "20"
    requests.memory: "40Gi"
    limits.cpu: "40"
    limits.memory: "80Gi"
    pods: "50"
```

### 2. 网络策略

限制 Pod 间通信：

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: pdf-processor-network-policy
spec:
  podSelector:
    matchLabels:
      app: pdf-processor
  policyTypes:
    - Ingress
    - Egress
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app: go-ingest-service
      ports:
        - protocol: TCP
          port: 50052
  egress:
    - to:
        - podSelector:
            matchLabels:
              app: redis
    - to:
        - podSelector:
            matchLabels:
              app: go-ingest-service
```

### 3. 优雅关闭

确保代码支持 SIGTERM：

```python
import signal
import sys

def signal_handler(sig, frame):
    logger.info("Received shutdown signal, finishing current task...")
    # 完成当前任务
    sys.exit(0)

signal.signal(signal.SIGTERM, signal_handler)
```

### 4. PodDisruptionBudget

确保滚动更新时至少有 worker 在线：

```yaml
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: pdf-processor-pdb
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: pdf-processor
```

## 🐞 故障排查

### 查看 Pod 日志

```bash
kubectl logs -f pod/pdf-processor-worker-xxx
```

### 查看事件

```bash
kubectl get events --sort-by='.lastTimestamp'
```

### 进入 Pod 调试

```bash
kubectl exec -it pod/pdf-processor-worker-xxx -- bash
```

### 查看资源使用

```bash
kubectl top pods -l app=pdf-processor
```

## 🔄 更新部署

### 滚动更新

```bash
# 更新镜像
kubectl set image deployment/pdf-processor-worker \
  worker=your-registry/pdf-processor:v1.1.0

# 查看更新状态
kubectl rollout status deployment/pdf-processor-worker
```

### 回滚

```bash
kubectl rollout undo deployment/pdf-processor-worker
```

## 📝 环境变量说明

| 变量名                | 说明             | 默认值               | 必需 |
| --------------------- | ---------------- | -------------------- | ---- |
| `REDIS_URL`           | Redis 连接地址   | -                    | ✅   |
| `REDIS_PASSWORD`      | Redis 密码       | -                    | ❌   |
| `REDIS_QUEUE_NAME`    | 任务队列名       | `queue:upload_tasks` | ✅   |
| `GO_GRPC_INGEST_ADDR` | Go 服务地址      | -                    | ✅   |
| `GRPC_SERVER_PORT`    | Python gRPC 端口 | `50052`              | ✅   |
| `BUCKET_ENDPOINT`     | 存储端点         | -                    | ✅   |
| `BUCKET_ACCESS_ID`    | 存储 Access ID   | -                    | ✅   |
| `BUCKET_ACCESS_KEY`   | 存储 Access Key  | -                    | ✅   |

## 📚 相关文档

- [Kubernetes HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)
- [KEDA Scalers](https://keda.sh/docs/scalers/)
- [Prometheus Operator](https://github.com/prometheus-operator/prometheus-operator)
