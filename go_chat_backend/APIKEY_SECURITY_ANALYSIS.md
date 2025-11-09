# API Key 传输方案安全性分析

## 当前实现分析

### 现状

```
用户上传文档 → Go后端 → 保存到Redis缓存 + gRPC发送APIKey → Python Worker
                      ↓
                   Redis Queue (upload_tasks) - 不含APIKey
```

**流程：**

1. 用户调用 `/api/pdf/confirm` 传递 `apiKey`
2. Go 保存 `apiKey` 到 Redis (30 分钟 TTL)
3. Go 通过 gRPC 同步发送 `apiKey` 到 Python
4. Go 将任务信息（不含 apiKey）推送到 Redis Queue
5. Python Worker 从内存缓存获取 apiKey 处理任务

---

## 方案对比

### ❌ 方案 1：当前 gRPC 同步发送 (现有方案)

**优点：**

- ✅ 实时传输，Python Worker 立即获得 API Key
- ✅ API Key 不经过消息队列，减少暴露面
- ✅ 可以立即验证连接是否成功

**缺点：**

- ❌ **阻塞 HTTP 请求**：用户必须等待 gRPC 调用完成（10s 超时）
- ❌ **单点故障**：Python Worker 不可用会导致上传失败
- ❌ **网络明文传输**：gRPC 如果没有 TLS，API Key 在网络中明文传输
- ❌ **缺乏重试机制**：gRPC 失败直接导致流程中断
- ❌ **Python Worker 内存管理**：API Key 存在 Python 进程内存中，容易泄露

**安全风险：**

- 🔴 **内存泄露**：Python 进程可能被 dump，暴露 API Key
- 🔴 **日志泄露**：gRPC 日志可能记录完整 API Key
- 🔴 **中间人攻击**：无 TLS 时可被拦截

---

### ✅ 方案 2：Redis Queue + 加密传输 (推荐)

**架构：**

```
用户 → Go → Redis Queue (加密的 apiKey + task) → Python Worker
              ↓
          设置短 TTL (5分钟)
```

**优点：**

- ✅ **异步非阻塞**：用户请求立即返回
- ✅ **解耦服务**：Go 和 Python 不需要直接连接
- ✅ **可靠性高**：Redis 持久化保证消息不丢失
- ✅ **支持重试**：Worker 失败可重新消费
- ✅ **水平扩展**：多个 Worker 可并发消费
- ✅ **审计追踪**：Redis 可记录消息处理历史

**实现细节：**

```go
// 1. 加密 API Key
encryptedAPIKey := encrypt(apiKey, secretKey)

// 2. 推送到队列
task := TaskMessage{
    DocID: docID,
    EncryptedAPIKey: encryptedAPIKey,
    Timestamp: time.Now(),
}
redis.LPush("upload_tasks", task)
redis.Expire("upload_tasks:"+docID, 5*time.Minute) // 短 TTL

// 3. Python Worker 消费
task = redis.RPop("upload_tasks")
apiKey = decrypt(task.EncryptedAPIKey, secretKey)
```

**安全措施：**

- 🟢 **AES-256 加密**：API Key 在 Redis 中加密存储
- 🟢 **短 TTL**：消息 5 分钟后自动过期
- 🟢 **消费即删除**：Worker 获取后立即从队列删除
- 🟢 **密钥轮换**：定期更换加密密钥
- 🟢 **Redis ACL**：限制队列访问权限

**缺点：**

- ⚠️ 需要管理加密密钥
- ⚠️ Redis 需要配置持久化和备份

---

### 🟡 方案 3：Kafka + SASL/SSL (企业级)

**架构：**

```
Go → Kafka (TLS + SASL) → Python Worker
```

**优点：**

- ✅ **高吞吐量**：适合大规模并发
- ✅ **持久化保证**：消息持久化到磁盘
- ✅ **分区并行**：多个 Worker 并行消费
- ✅ **消息回溯**：可重新消费历史消息
- ✅ **企业级安全**：SASL/SCRAM + SSL 加密

**缺点：**

- ❌ **运维复杂**：需要维护 Kafka 集群
- ❌ **资源开销大**：内存和磁盘占用高
- ❌ **成本高**：对于小规模应用过度设计
- ❌ **延迟较高**：相比 Redis 有额外的网络延迟

**适用场景：**

- 日处理量 > 100 万文档
- 需要消息审计和回溯
- 已有 Kafka 基础设施

---

### 🟢 方案 4：Celery + Redis Backend (平衡方案)

**架构：**

```
Go → Celery (Redis Broker) → Python Worker
```

**优点：**

- ✅ **成熟稳定**：Celery 是 Python 生态标准
- ✅ **功能丰富**：重试、延迟、定时任务
- ✅ **监控完善**：Flower 提供 UI 监控
- ✅ **优先级队列**：支持任务优先级
- ✅ **结果追踪**：可查询任务状态

**实现：**

```python
# Python Worker
from celery import Celery
from cryptography.fernet import Fernet

app = Celery('tasks', broker='redis://localhost:6379')

@app.task(bind=True, max_retries=3)
def process_document(self, doc_id, encrypted_api_key):
    try:
        api_key = decrypt(encrypted_api_key)
        # 处理文档...
    except Exception as exc:
        raise self.retry(exc=exc, countdown=60)
```

```go
// Go 发送任务
task := map[string]interface{}{
    "task": "process_document",
    "args": []interface{}{docID, encryptedAPIKey},
    "kwargs": map[string]interface{}{},
}
redis.LPush("celery", json.Marshal(task))
```

**缺点：**

- ⚠️ Go 与 Celery 协议交互需要额外开发
- ⚠️ 依赖 Python 生态

---

## 🏆 推荐方案

### 短期方案 (1-2 周实现)：**方案 2 - Redis Queue + 加密**

**理由：**

1. **最小改动**：基于现有 Redis 基础设施
2. **安全性提升**：加密存储，短 TTL
3. **用户体验好**：异步处理，立即返回
4. **成本低**：无需额外组件

**实施步骤：**

```go
// Step 1: 添加加密工具
func encryptAPIKey(plaintext, key string) (string, error) {
    block, _ := aes.NewCipher([]byte(key))
    gcm, _ := cipher.NewGCM(block)
    nonce := make([]byte, gcm.NonceSize())
    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Step 2: 修改任务结构
type EtlTask struct {
    DocID            string
    FileName         string
    URL              string
    UserID           string
    EncryptedAPIKey  string // 新增
    Provider         string // 新增
    CreatedAt        time.Time
}

// Step 3: 推送加密任务
encryptedKey, _ := encryptAPIKey(req.ApiKey, config.EncryptionKey)
task := EtlTask{
    DocID:           info.FileID,
    EncryptedAPIKey: encryptedKey,
    Provider:        req.Provider,
    // ...
}
messageQueue.PushToQueue("upload_tasks", task)

// Step 4: Python Worker 解密
def decrypt_api_key(encrypted_key):
    cipher = Fernet(ENCRYPTION_KEY)
    return cipher.decrypt(encrypted_key.encode()).decode()
```

---

### 长期方案 (1-3 个月)：**方案 4 - Celery**

**理由：**

1. **可观测性**：Flower 监控面板
2. **可维护性**：Python 团队熟悉 Celery
3. **功能完善**：支持复杂的任务编排

---

## 安全加固建议

### 1. 加密层

```bash
# 使用环境变量管理密钥
export ENCRYPTION_KEY=$(openssl rand -base64 32)
export REDIS_PASSWORD=$(openssl rand -base64 24)
```

### 2. Redis 安全配置

```redis
# redis.conf
requirepass ${REDIS_PASSWORD}
rename-command FLUSHDB ""
rename-command FLUSHALL ""
rename-command CONFIG ""
bind 127.0.0.1  # 仅本地访问
```

### 3. 网络隔离

```yaml
# docker-compose.yml
services:
  redis:
    networks:
      - internal
  go-backend:
    networks:
      - internal
      - public
  python-worker:
    networks:
      - internal
```

### 4. API Key 轮换

```go
// 定期检查 API Key 有效性
func validateAPIKey(apiKey string) error {
    // 调用 LLM API 验证
    // 失败则通知用户更新
}
```

### 5. 审计日志

```go
// 记录 API Key 使用情况（脱敏）
logging.Logger.Info("API Key used",
    "userID", userID,
    "keyHash", sha256(apiKey)[:8],
    "timestamp", time.Now(),
)
```

---

## 性能对比

| 方案        | 延迟      | 吞吐量      | CPU | 内存 | 成本 |
| ----------- | --------- | ----------- | --- | ---- | ---- |
| gRPC 同步   | 100-500ms | 500 req/s   | 中  | 低   | 低   |
| Redis Queue | 10-50ms   | 5000 req/s  | 低  | 中   | 低   |
| Kafka       | 20-100ms  | 50000 req/s | 高  | 高   | 高   |
| Celery      | 20-80ms   | 10000 req/s | 中  | 中   | 中   |

---

## 结论

**当前系统规模：建议采用 方案 2 (Redis Queue + 加密)**

**升级路径：**

1. **Phase 1 (现在)**：移除 gRPC 发送 API Key，改用加密 Redis Queue
2. **Phase 2 (1 个月后)**：评估是否需要 Celery
3. **Phase 3 (规模扩大后)**：考虑 Kafka

**关键指标监控：**

- API Key 泄露事件：0
- 任务处理成功率：> 99%
- 平均处理延迟：< 2 秒
- 用户等待时间：< 200ms
