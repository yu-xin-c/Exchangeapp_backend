# RAG 问答系统集成指南

## 概述

项目已完成基础 RAG 问答接口实现。为了启用完整的向量检索与 AI 回答功能，需要与 `eino_assistant` 文件夹中的 Eino 框架集成。

## 当前实现

### 1. 问答接口
- **路由**: `POST /api/qa`
- **认证**: 需要 JWT token
- **请求体**:
  ```json
  {
    "question": "您的问题",
    "topk": 3
  }
  ```
- **响应**:
  ```json
  {
    "answer": "AI生成的回答",
    "sources": [
      {
        "id": 1,
        "title": "文章标题",
        "preview": "文章预览"
      }
    ]
  }
  ```

### 2. 服务层
- **`services/qa_service.go`**: 核心问答逻辑
  - `AnswerQuestion()`: 处理问题，优先使用RAG服务，回退到简单检索
  - `InitRAGService()`: 初始化RAG服务

- **`services/ai_client.go`**: OpenAI API 集成
  - 支持环境变量 `OPENAI_API_KEY` 配置

- **`services/rag_service.go`**: RAG 服务框架（待完整实现）

### 3. 控制器层
- **`controllers/qa_controller.go`**: HTTP 请求处理
  - `AnswerQuestion()`: 接收问题请求并返回回答

## 集成 Eino RAG

### 第一步：配置环境变量

创建 `.env` 文件，配置火山云方舟 API：

```bash
# 火山云方舟配置
export ARK_API_KEY="your-api-key"
export ARK_CHAT_MODEL="doubao-pro-32k-241215"
export ARK_EMBEDDING_MODEL="doubao-embedding-large-text-240915"
export ARK_API_BASE_URL="https://ark.cn-beijing.volces.com/api/v3"

# Redis 配置（用于向量检索）
export REDIS_ADDR="localhost:6379"

# OpenAI 配置（可选）
export OPENAI_API_KEY="your-openai-key"
```

### 第二步：初始化知识库

使用 `eino_assistant` 中的工具将文章索引到 Redis：

```bash
cd eino_assistant

# 启动 Redis（如果未启动）
docker-compose up -d

# 运行知识库索引
go run cmd/knowledgeindexing/main.go
```

这会将 `eino_assistant/cmd/knowledgeindexing/eino-docs` 目录下的 Markdown 文件转换为向量并存储到 Redis。

### 第三步：测试问答接口

```bash
# 启动主服务
go run main.go

# 使用 curl 测试（需要有效的 JWT token）
curl -X POST http://localhost:8080/api/qa \
  -H "Authorization: Bearer <your-jwt-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "question": "如何使用 Redis 存储向量?",
    "topk": 3
  }'
```

## 工作流程

### 简单检索流程（当前默认）
1. 用户提交问题
2. 系统按关键词在 MySQL 数据库中搜索相关文章
3. 收集前 topK 篇文章作为上下文
4. 构建 prompt 提交给 OpenAI（如果配置了 API key）
5. 返回 AI 回答和源文章

### 向量检索流程（启用 RAG 后）
1. 用户提交问题
2. 使用 Eino 嵌入模型将问题转换为向量
3. 向量在 Redis 中进行 KNN 相似性搜索
4. 返回最相似的 topK 篇文章
5. 将这些文章内容作为上下文
6. 调用火山云方舟 AI 模型生成回答
7. 返回 AI 回答和源文章

## 完整实现清单

- [x] 基础问答接口 (`POST /api/qa`)
- [x] 简单关键词检索
- [x] OpenAI API 集成
- [x] 路由与中间件配置
- [x] 编译通过
- [ ] Eino 完整集成（需要配置 ARK_API_KEY）
- [ ] Redis 向量索引初始化
- [ ] 火山云方舟 API 集成
- [ ] 多租户知识库支持
- [ ] WebSocket 实时问答

## 调试说明

### 检查 RAG 服务状态
```go
// 在 main.go 中
if ragService != nil {
    log.Println("RAG service initialized successfully")
} else {
    log.Println("Using fallback simple retrieval")
}
```

### 查看检索结果
修改 `services/qa_service.go` 中的 `AnswerQuestion()` 函数，打印检索到的文章：

```go
for _, art := range articles {
    log.Printf("Retrieved article: %s (ID: %d)", art.Title, art.ID)
}
```

## 性能优化建议

1. **缓存热点问题**: 在 Redis 中缓存常见问题的回答
2. **批量处理**: 使用消息队列处理批量问题
3. **异步生成**: 对长文本回答使用后台任务
4. **向量缓存**: 缓存常用查询的向量表示

## 参考资源

- [Eino 官方文档](https://github.com/cloudwego/eino)
- [eino_assistant 示例](./eino_assistant/readme.md)
- [排行榜设计文档](./docs/ranking_design.md)

## 下一步计划

1. 配置火山云方舟 API
2. 初始化 Redis 向量索引
3. 集成完整 Eino RAG 流程
4. 添加批量文章索引功能
5. 实现知识库版本管理
6. 支持多个知识库的切换

---

**注意**: 当前实现支持无 RAG 回退模式，确保系统在缺少 Eino 依赖时仍能正常运行。
