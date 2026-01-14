# 排行榜设计文档（Redis + MySQL + MQ + WebSocket）

作者：自动生成
日期：2025-11-28

## 概述

本设计面向点赞排行榜场景，目标在高并发下实现：
- 实时性：前端能感知排行榜 TopN 的及时变化（秒级/近实时）
- 可扩展性：能够在中等/大规模下水平扩展（Redis Cluster / MQ / 多消费者）
- 可恢复性：事件可持久化到 MySQL，支持补偿与纠偏

核心组件：API(Gin) → Redis（计数 + ZSET）→ MQ（RabbitMQ/Kafka）→ Consumer（落库 + 聚合）→ WebSocket（实时推送）

## 核心流程

1. 前端发起点赞请求（POST /like/:id）
2. API 层：
   - 可选：检查用户是否已点赞（`user:{uid}:liked`）防止重复
   - Redis INCR 单篇点赞计数：`INCR article:{id}:likes`
   - 更新排行榜 ZSET：`ZINCRBY rank:article:likes 1 {id}`（或 ZADD score）
   - 发布事件到 MQ（LikeEvent：articleId、userId、action、ts）
   - 立即返回 200 给用户（快速响应）
3. MQ 将事件投递给 Consumer
4. Consumer：
   - 聚合事件（本地缓冲去抖，例如每 1s 汇总）
   - 批量落库（MySQL）：`UPDATE article SET likes = likes + ? WHERE id = ?`
   - 可选：校准 Redis ZSET（批量读取 DB 或使用 Redis 中的计数做幂等修正）
   - 按频率（1s/3s）计算 TopN 并通过 WebSocket Hub 广播差分更新

## Redis Key 设计（建议）

- `article:{id}:likes` — 单文章即时计数（String 或 Hash 字段）
- `rank:article:likes` — 全量排行榜（ZSET，member=articleId, score=likes）
- `rank:article:likes:day:YYYY-MM-DD` — 日榜（ZSET）
- `rank:article:likes:week:YYYY-WW` — 周榜（ZSET）
- `user:{uid}:liked:articles` — 用户点赞集合（Set / Bitmap / BloomFilter，用于去重）
- `likes:buffer:{shard}` — 批量同步缓冲 Key（可选，用于批量处理）

说明：ZSET 用于排行聚合，单条写操作可使用 `ZINCRBY`。对热点 Key 做监控（memory、TTL）。

## 原子性与一致性建议

- INCR + ZINCRBY 可通过 Lua 脚本在 Redis 端原子执行，避免并发下计数与 ZSET 不一致。
- 在极高并发时：使用本地缓冲/批量更新策略（1s 聚合），以减少对 Redis 的写压力。
- Consumer 批量做 DB 更新，定期做一次全量/差量校准以修正累积误差。

## 消息队列选型对比与建议

- RabbitMQ：适合事件确认、复杂路由、任务队列场景；易上手，适合大多数落库+广播场景。
- Kafka：适合高吞吐、保留历史、需要事件溯源或流处理的场景；运维复杂度更高。

推荐：若当前仅需异步落库与广播，优先 RabbitMQ；若需做大规模行为分析或流处理，选 Kafka。

## Consumer 设计（聚合与去抖）

- 消费策略：使用手动 ack（RabbitMQ）或按需提交 offset（Kafka）；保证至少一次或恰好一次语义（根据业务容忍度）。
- 聚合策略：把到达的事件按 articleId 累加到内存 map；每 T 秒（1s/3s）把 map 批量写入 DB 并清空。
- 广播策略：由 Consumer 计算 TopN（`ZREVRANGE rank:article:likes 0 N-1 WITHSCORES`），并仅将变化的条目（差分）推送到 WebSocket Hub。

示例伪逻辑：

```go
// 简化版消费者聚合伪代码
delta := map[int64]int64{}
for msg := range msgs {
  ev := decode(msg)
  delta[ev.ArticleID] += 1
  // ack 或延迟 ack
}

ticker := time.NewTicker(1 * time.Second)
for range ticker.C {
  // 批量更新 DB
  for id, v := range delta {
    db.Exec("UPDATE article SET likes = likes + ? WHERE id = ?", v, id)
  }
  delta = map[int64]int64{}
  // 计算 TopN 并广播
}
```

## WebSocket Hub 设计

- 使用 `github.com/gorilla/websocket` 实现 Hub（Register/Unregister/Broadcast）。
- 支持按榜单类型订阅房间（global/day/week），并实现心跳（ping/pong）与写队列以防慢客户端。
- 广播内容采用差分（只发送发生改变的 articleId、score、rank），减少流量。

示例消息格式（JSON）：

```json
{
  "type": "rank_update",
  "list": [
    {"id": 1001, "rank": 1, "score": 1234},
    {"id": 1002, "rank": 2, "score": 1200}
  ]
}
```

## 去重与防刷策略

- 在 Redis 中维护 `user:{uid}:liked` set（或 bitmap / bloom filter）用于快速判断重复点赞。
- 对敏感场景使用频率限制（限流）和 CAPTCHA 或风控链路。

## 分布式扩展方案

- 单 Redis：简单直接，适用于中小规模服务。
- Redis Cluster + 聚合器：各 shard 保存本地 TopK，Central Aggregator 定期合并各 shard TopK 得到全局 TopN（适合中等规模）。
- Kafka + 流处理：将点赞事件写入 Kafka，使用 Flink/KStream 做 TopN 计算并写回 Redis，适合大流量与复杂流式分析场景。

合并策略示例（每 shard 保存 Top100）：
1. 每个 shard 的 Aggregator 抽取本地 TopK 推送到 Central Aggregator（或 MQ）。
2. Central Aggregator 合并所有 shard 的 TopK（内存合并）得到全局 TopN。
3. 写回统一的 `rank:article:likes` 或直接通知 WS。

## 推送与频率控制建议

- 去抖（Debounce）聚合：每 T 秒推一次 TopN（通常 1s/3s）以减少推送频率。
- 差分广播：只向客户端发送发生变化的项目。
- 分房间订阅：用户仅订阅感兴趣的榜单/分类，减少无效推送。

## 生产部署注意事项

- Redis：监控 Memory、AOF/RDB 配置；对热点 key 做监控和报警；考虑使用 Redis Cluster。
- MQ：监控队列长度、消费延迟与死信队列（DLQ）；配置合理的 prefetch/consumer 并发。
- MySQL：批量写入、索引优化；对于高吞吐场景使用分库分表或读写分离策略。
- 监控：建立 Grafana + Prometheus 面板，监控 MQ lag、Redis key 热点、API QPS、消费延迟、TopN 计算耗时。

## 在 `Exchangeapp_backend` 项目中的落地建议（文件/位置）

- 新增 `controllers/like_controller.go`：实现点赞接口，调用 Redis、ZINCRBY、发布 MQ。
- 新增 `ws/hub.go`：实现 WebSocket Hub。
- 新增 `consumer/like_consumer.go`：独立消费者进程或 goroutine，聚合、落库、广播 TopN。
- 新增 `jobs/sync_likes.go`：定时批量同步脚本（用于修正/补偿）。
- 在 `config/config.yml` 或 `config/config.go` 中添加 MQ、Redis 配置。

## 示例快速参考（简化）

- LikeHandler（核心步骤）

```go
// 1. 校验 user
// 2. conn.Do("INCR", likeKey)
// 3. conn.Do("ZINCRBY", rankKey, 1, articleId)
// 4. publish to MQ
```

- Consumer（聚合）

```go
// aggregate map
// ticker flush -> batch DB update -> compute TopN -> hub.Broadcast
```

## 校验与补偿

- 推荐定期（每天/每小时）做一次校验任务：把 MySQL 的 likes 值与 Redis 中的计数做比对，发现差异时修正（以 DB 为准或人工介入）。

## 参考与后续工作

- 我可以根据本设计帮你在仓库中创建：
  - `controllers/like_controller.go`
  - `ws/hub.go`
  - `consumer/like_consumer.go`
  - 在 `config/` 中添加 MQ 配置示例

如需我继续实现代码，请回复“执行 A：创建代码文件并实现示例”。

---

文件位置：`docs/ranking_design.md`
