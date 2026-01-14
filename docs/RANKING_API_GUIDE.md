# 点赞排行榜 API 使用指南

## 服务启动

```bash
go run main.go
```

服务默认运行在 `http://localhost:3000`

## API 端点说明

### 1. 点赞接口（创建/更新点赞）

**请求**

```http
POST /api/articles/:id/like
X-User-Id: 1
```

| 参数 | 说明 |
|------|------|
| `:id` | 文章 ID（URL 路径参数） |
| `X-User-Id` (Header) | 用户 ID（可选，若提供则启用去重；若为空，则匿名点赞） |

**响应示例（首次点赞）**

```json
{
  "message": "Successfully liked the article",
  "likes": 5
}
```

**响应示例（重复点赞）**

```json
{
  "message": "already liked",
  "likes": "4"
}
```

**实现细节**

- 若提供 `X-User-Id`：使用 Redis Set `user:{uid}:liked:articles` 做去重，仅首次点赞才会增加计数。
- 若无 `X-User-Id`：匿名用户每次都增加计数（无去重）。
- 每次点赞都会：
  - Redis `INCR article:{id}:likes`（计数 +1）
  - Redis `ZINCRBY rank:article:likes 1 {id}`（更新排行 ZSET）
  - MySQL 写入 `likes` 表记录（历史审计）

### 2. 获取单篇文章点赞数

**请求**

```http
GET /api/articles/:id/like
```

**响应示例**

```json
{
  "likes": "10"
}
```

如果没有点赞记录，返回 `"0"`

### 3. 获取排行榜 TOP N（核心排行接口）

**请求**

```http
GET /api/articles/rank?top=20
```

| 参数 | 默认值 | 说明 |
|------|-------|------|
| `top` | 10 | 返回前 N 篇文章，范围 1-100（建议），可根据需要调整 |

**响应示例**

```json
{
  "list": [
    {
      "id": "1001",
      "rank": 1,
      "score": 150,
      "title": "Python 最佳实践"
    },
    {
      "id": "1002",
      "rank": 2,
      "score": 120,
      "title": "Go 并发编程"
    },
    {
      "id": "1003",
      "rank": 3,
      "score": 95,
      "title": "Redis 架构设计"
    }
  ]
}
```

如果排行榜为空，返回：

```json
{
  "list": []
}
```

**字段说明**

| 字段 | 说明 |
|------|------|
| `id` | 文章 ID |
| `rank` | 排名（从 1 开始） |
| `score` | 点赞数 |
| `title` | 文章标题（从 MySQL 查询，若查询失败则不返回） |

## 测试示例（PowerShell / curl）

### 示例 1：创建 3 篇文章并点赞

```powershell
# 创建文章 1
$article1 = curl.exe -X POST http://localhost:3000/api/articles `
  -H "Content-Type: application/json" `
  -H "Authorization: Bearer YOUR_TOKEN" `
  -d '{"title":"Python最佳实践","content":"内容...","preview":"预览..."}'

# 创建文章 2
$article2 = curl.exe -X POST http://localhost:3000/api/articles `
  -H "Content-Type: application/json" `
  -H "Authorization: Bearer YOUR_TOKEN" `
  -d '{"title":"Go并发编程","content":"内容...","preview":"预览..."}'

# 创建文章 3
$article3 = curl.exe -X POST http://localhost:3000/api/articles `
  -H "Content-Type: application/json" `
  -H "Authorization: Bearer YOUR_TOKEN" `
  -d '{"title":"Redis架构设计","content":"内容...","preview":"预览..."}'
```

（注：需要先登录获取有效的 Bearer Token，或在服务中配置绕过认证）

### 示例 2：用户 1 点赞文章 1、2、3（各多次）

```powershell
# 用户 1 点赞文章 1（5 次）
for ($i = 1; $i -le 5; $i++) {
  curl.exe -X POST http://localhost:3000/api/articles/1/like `
    -H "X-User-Id: 1"
}

# 用户 1 点赞文章 2（3 次，实际去重为 1 次）
for ($i = 1; $i -le 3; $i++) {
  curl.exe -X POST http://localhost:3000/api/articles/2/like `
    -H "X-User-Id: 1"
}

# 用户 1 点赞文章 3（2 次，实际去重为 1 次）
for ($i = 1; $i -le 2; $i++) {
  curl.exe -X POST http://localhost:3000/api/articles/3/like `
    -H "X-User-Id: 1"
}
```

### 示例 3：查看排行榜

```powershell
# 查看 Top 10
curl.exe http://localhost:3000/api/articles/rank?top=10

# 查看 Top 20
curl.exe http://localhost:3000/api/articles/rank?top=20
```

预期输出：文章 1 排名第一（5 次点赞），文章 2 第二（1 次点赞），文章 3 第三（1 次点赞）

### 示例 4：查询单篇文章点赞数

```powershell
curl.exe http://localhost:3000/api/articles/1/like
```

输出：

```json
{
  "likes": "5"
}
```

## Redis 数据结构（内部实现细节）

实现排行榜的 Redis Key 及其用途：

| Key 模式 | 类型 | 用途 |
|---------|------|------|
| `article:{id}:likes` | String | 单篇文章的即时点赞计数 |
| `rank:article:likes` | Sorted Set (ZSET) | 全量排行榜，member=articleId, score=likes 数 |
| `user:{uid}:liked:articles` | Set | 用户点赞过的文章集合（用于去重） |

### 查看 Redis 数据（使用 redis-cli）

```bash
# 查看文章 1 的点赞数
GET article:1:likes

# 查看排行榜 Top 3（降序）
ZREVRANGE rank:article:likes 0 2 WITHSCORES

# 查看用户 1 点赞过的文章
SMEMBERS user:1:liked:articles
```

## MySQL 数据结构

### `likes` 表（点赞历史记录）

```sql
CREATE TABLE likes (
  id bigint AUTO_INCREMENT PRIMARY KEY,
  user_id int,
  article_id int,
  created_at timestamp DEFAULT CURRENT_TIMESTAMP,
  updated_at timestamp DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted_at timestamp NULL,
  INDEX idx_user_id (user_id),
  INDEX idx_article_id (article_id)
);
```

每条点赞记录对应一行，用于：
- 审计：查看用户的点赞历史
- 分析：统计用户点赞趋势
- 校验：与 Redis 定期对账

## 架构说明

```
用户点赞请求
    ↓
POST /api/articles/:id/like (with X-User-Id header)
    ↓
API 层（controllers/like_controller.go）
    ├── 检查用户是否已点赞（Redis Set 去重）
    ├── INCR article:{id}:likes（计数 +1）
    ├── ZINCRBY rank:article:likes（排行 ZSET 更新）
    └── INSERT into likes table（历史记录）
    ↓
Redis（热数据缓存 + 排行榜）
    └── ZREVRANGE rank:article:likes → 返回排行榜
    ↓
MySQL（冷数据持久化 + 审计）
    └── likes 表（点赞历史）
```

## 生产优化建议

### 1. 异步持久化（消息队列）

当前实现为同步写 MySQL。若并发较高，建议改为：

```
API 层：
  1. Redis INCR/ZINCRBY + SADD（快速响应）
  2. 发布 LikeEvent 到 Kafka/RabbitMQ

消费者服务：
  1. 聚合事件（每 1s 或 3s）
  2. 批量更新 MySQL
  3. 可选：更新/校准 Redis ZSET
  4. 推送排行榜变更到 WebSocket 客户端
```

我可以帮你实现这部分，详见 `docs/ranking_design.md`。

### 2. 定期校验与修复

建议每天凌晨或定时执行：

```sql
-- 对账脚本：比较 MySQL 与 Redis 的点赞数
SELECT article_id, COUNT(*) as count FROM likes WHERE deleted_at IS NULL GROUP BY article_id;
-- 与 Redis ZSET 中的分数对比，发现差异则修正
```

### 3. 缓存更新策略

- 排行榜 ZSET 仅在内存中维护，无 TTL（永久存储）。
- 若需定期重置排行（例如日榜/周榜），可用额外 Key：
  - `rank:article:likes:day:2025-11-30`
  - `rank:article:likes:week:2025-W48`

### 4. 监控告警

- 监控 `rank:article:likes` ZSET 的大小（包含文章数）。
- 监控点赞 API 的 QPS、延迟、错误率。
- 定期检查 MySQL `likes` 表的增长速度和 DB 大小。

## 常见问题（FAQ）

**Q: 为什么同一用户点赞多次只算 1 次？**

A: 这是去重机制，使用 Redis Set `user:{uid}:liked:articles` 实现。用户首次点赞时 `SADD` 返回 1（新增），之后返回 0（已存在），API 检测到 0 即拒绝重复点赞。这防止了刷点赞的作弊行为。

**Q: 匿名用户（无 X-User-Id）能点赞吗？**

A: 可以。每次请求都会增加计数，但无法去重（因为没有用户身份来追踪）。生产环境建议仅允许登录用户点赞。

**Q: 排行榜何时更新？**

A: 实时更新。点赞后立即 `ZINCRBY rank:article:likes`，`GET /api/articles/rank` 会获取最新的 ZSET 内容。

**Q: 能否按时间范围查看排行（例如今日排行）？**

A: 当前实现为全量排行。若要支持日榜/周榜，可添加额外的 ZSET Key：
- `rank:article:likes:day:YYYY-MM-DD`
- `rank:article:likes:week:YYYY-WW`
消费者或定时任务定期更新这些 Key。

**Q: 点赞记录能删除吗（取消点赞）？**

A: 当前实现无取消功能。可添加 `DELETE /api/articles/:id/unlike` 接口：
- `DECR article:{id}:likes`
- `ZINCRBY rank:article:likes -1 {id}`（减 1）
- `SREM user:{uid}:liked:articles {id}`（从集合移除）
- 标记 MySQL 中的点赞记录为已删除（soft delete）

## 下一步

1. **测试**：用上面的示例在本地测试各个接口。
2. **集成消息队列**：若需高并发支持，接入 Kafka/RabbitMQ（见 `docs/ranking_design.md`）。
3. **添加 WebSocket 推送**：让前端实时感知排行榜变更（支持 `ws://localhost:3000/api/rank/subscribe`）。
4. **部署与监控**：上线前做好 Redis/MySQL 的备份、监控和告警。

---

**文档维护**：如有问题或需要补充，请更新此文档或联系开发团队。

最后更新：2025-11-30
