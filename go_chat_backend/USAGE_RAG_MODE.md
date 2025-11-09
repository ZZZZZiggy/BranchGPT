# RAG Mode 使用文档

## 概述

RAG Mode 控制文档处理时是否需要生成 embeddings。通过两层缓存策略（内存 + 数据库）实现高性能访问。

## 架构设计

```
请求流程：
1. 用户上传文档 → 设置 RagMode=true/false
2. Confirm 时检查 RagMode
3. 如果 RagMode=false，跳过 Python 微服务的 embedding 生成
4. 如果 RagMode=true，通过 gRPC 调用 Python 生成 embeddings

缓存策略：
查询：L1 内存缓存 → 数据库
写入：数据库 + L1 内存缓存同步更新
```

## 数据库 Migration

```sql
-- 添加 rag_mode 字段到 document_meta 表
ALTER TABLE document_meta 
ADD COLUMN rag_mode BOOLEAN DEFAULT false;

-- 为已存在的文档设置默认值
UPDATE document_meta SET rag_mode = false WHERE rag_mode IS NULL;
```

## 使用示例

### 1. 初始化服务（在 bootstrap 中）

```go
// bootstrap/services.go
func (a *App) initServices() {
    // ... 其他服务初始化
    
    // 初始化 RAG Mode 服务
    ragModeService := services.NewRagModeService(
        a.CacheService,
        a.DocumentRepository,
    )
    a.RagModeService = ragModeService
}
```

### 2. 在文档上传时设置 RAG Mode

```go
// handlers/document_handler.go

type UploadRequest struct {
    File    *multipart.FileHeader `form:"file" binding:"required"`
    RagMode bool                  `form:"rag_mode"` // 前端传入
}

func (h *DocumentHandler) Upload(c *gin.Context) {
    var req UploadRequest
    if err := c.ShouldBind(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    // 创建文档元数据
    doc := &models.DocumentMeta{
        FileID:   generateFileID(),
        UserID:   getUserID(c),
        Filename: req.File.Filename,
        RagMode:  req.RagMode, // 设置 RAG Mode
        // ... 其他字段
    }
    
    if err := h.docRepo.Create(c.Request.Context(), doc); err != nil {
        c.JSON(500, gin.H{"error": "Failed to create document"})
        return
    }
    
    c.JSON(200, gin.H{
        "file_id":  doc.FileID,
        "rag_mode": doc.RagMode,
    })
}
```

### 3. 在 Confirm 时检查 RAG Mode

```go
// handlers/document_handler.go

func (h *DocumentHandler) ConfirmDocument(c *gin.Context) {
    fileID := c.Param("file_id")
    ctx := c.Request.Context()
    
    // ✅ 高性能：从缓存/数据库获取 RAG Mode
    ragMode, err := h.ragModeService.GetRagMode(ctx, fileID)
    if err != nil {
        c.JSON(500, gin.H{"error": "Failed to get RAG mode"})
        return
    }
    
    // 根据 RAG Mode 决定处理流程
    if ragMode {
        // 需要生成 embeddings，调用 Python 微服务
        if err := h.processWithEmbeddings(ctx, fileID); err != nil {
            c.JSON(500, gin.H{"error": "Failed to process with embeddings"})
            return
        }
    } else {
        // 不需要 embeddings，直接存储文本
        if err := h.processWithoutEmbeddings(ctx, fileID); err != nil {
            c.JSON(500, gin.H{"error": "Failed to process without embeddings"})
            return
        }
    }
    
    c.JSON(200, gin.H{
        "message":  "Document confirmed",
        "file_id":  fileID,
        "rag_mode": ragMode,
    })
}
```

### 4. gRPC 调用时传递 RAG Mode

```go
// services/grpc_chunk_services.go

func (s *ChunkService) ProcessChunk(ctx context.Context, fileID string, chunkText string) error {
    // 检查是否需要生成 embedding
    ragMode, err := s.ragModeService.GetRagMode(ctx, fileID)
    if err != nil {
        return fmt.Errorf("failed to get rag mode: %w", err)
    }
    
    // 准备 chunk 数据
    chunk := &models.Chunk{
        ChunkID:    generateChunkID(),
        FileID:     fileID,
        ChunkText:  chunkText,
        // ... 其他字段
    }
    
    if ragMode {
        // ✅ 调用 Python gRPC 服务生成 embedding
        embeddingReq := &pb.EmbeddingRequest{
            TaskId: generateTaskID(),
            Text:   chunkText,
        }
        
        embeddingResp, err := s.grpcClient.GetEmbedding(ctx, embeddingReq)
        if err != nil {
            return fmt.Errorf("failed to get embedding: %w", err)
        }
        
        if !embeddingResp.Success {
            return fmt.Errorf("embedding failed: %s", embeddingResp.Message)
        }
        
        // 将 embedding 转换为 pgvector.Vector
        chunk.EmbeddingVector = convertToVector(embeddingResp.Embeddings)
    } else {
        // ❌ 跳过 embedding 生成
        // chunk.EmbeddingVector 保持空值或零向量
        chunk.EmbeddingVector = pgvector.NewVector(make([]float32, 384))
    }
    
    // 保存到数据库
    return s.chunkRepo.Create(ctx, chunk)
}
```

### 5. 动态修改 RAG Mode（可选）

```go
// handlers/document_handler.go

func (h *DocumentHandler) UpdateRagMode(c *gin.Context) {
    fileID := c.Param("file_id")
    
    var req struct {
        RagMode bool `json:"rag_mode" binding:"required"`
    }
    
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    // 更新 RAG Mode（同时更新数据库和缓存）
    if err := h.ragModeService.SetRagMode(c.Request.Context(), fileID, req.RagMode); err != nil {
        c.JSON(500, gin.H{"error": "Failed to update RAG mode"})
        return
    }
    
    c.JSON(200, gin.H{
        "message":  "RAG mode updated",
        "file_id":  fileID,
        "rag_mode": req.RagMode,
    })
}
```

## 性能优化

### 缓存命中率

- **第一次查询**：查数据库（~5-10ms）
- **后续查询**：读内存缓存（~0.1ms）
- **缓存过期**：24 小时（可调整）

### 适用场景

- ✅ **高频读取**：同一文档的 RAG mode 被频繁查询
- ✅ **低频修改**：RAG mode 通常在创建时设置，很少改变
- ✅ **批量处理**：处理大量 chunks 时只需查询一次

### 内存占用

- 每个 fileID 的缓存：~100 bytes
- 10,000 个文档：~1MB 内存
- 可接受的开销

## 监控建议

```go
// 添加 metrics
var (
    ragModeCacheHits   = prometheus.NewCounter(...)
    ragModeCacheMisses = prometheus.NewCounter(...)
    ragModeDBQueries   = prometheus.NewCounter(...)
)

// 在 GetRagMode 中记录
func (s *RagModeService) GetRagMode(ctx context.Context, fileID string) (bool, error) {
    cacheKey := s.getCacheKey(fileID)
    
    if ragMode, found, err := s.l1Cache.Get(cacheKey); err == nil && found {
        ragModeCacheHits.Inc() // 缓存命中
        return ragMode, nil
    }
    
    ragModeCacheMisses.Inc() // 缓存未命中
    ragModeDBQueries.Inc()   // 数据库查询
    
    // ... 从数据库查询
}
```

## 总结

| 方面 | 实现方式 |
|------|---------|
| **持久化** | PostgreSQL `document_meta.rag_mode` 字段 |
| **缓存** | L1 内存缓存（24h TTL） |
| **性能** | 首次 ~5ms，后续 ~0.1ms |
| **一致性** | 写入同时更新 DB + 缓存 |
| **内存开销** | ~100 bytes/文档 |
| **适用场景** | 高频读、低频写 |

这个方案在性能和复杂度之间取得了很好的平衡！
