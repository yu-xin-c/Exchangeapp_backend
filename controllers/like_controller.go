package controllers

import (
	"exchangeapp/global"
	"exchangeapp/models"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis"
)

// LikeArticle: 增量更新 Redis 计数并记录到 MySQL（当前为同步写入，生产可改为 MQ 异步）
func LikeArticle(ctx *gin.Context) {
	articleIDStr := ctx.Param("id")
	articleID, _ := strconv.ParseUint(articleIDStr, 10, 64)

	// 尽量从 header 获取用户 id（可选）
	var userID uint64 = 0
	if u := ctx.GetHeader("X-User-Id"); u != "" {
		if v, err := strconv.ParseUint(u, 10, 64); err == nil {
			userID = v
		}
	}

	likeKey := "article:" + articleIDStr + ":likes"
	rankKey := "rank:article:likes"

	// 如果有 userID，则使用 Set 做去重：user:{uid}:liked:articles
	if userID != 0 {
		userKey := "user:" + strconv.FormatUint(userID, 10) + ":liked:articles"
		added, err := global.RedisDB.SAdd(userKey, articleIDStr).Result()
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if added == 0 {
			// 已点赞，返回当前点赞数
			likesStr, _ := global.RedisDB.Get(likeKey).Result()
			if likesStr == "" {
				likesStr = "0"
			}
			ctx.JSON(http.StatusOK, gin.H{"message": "already liked", "likes": likesStr})
			return
		}
		// 如果新增成功（首次点赞），继续更新计数和写库
	}

	// 使用 pipeline 同步执行 INCR + ZINCRBY
	pipe := global.RedisDB.TxPipeline()
	incrCmd := pipe.Incr(likeKey)
	zincrCmd := pipe.ZIncrBy(rankKey, 1, articleIDStr)
	if _, err := pipe.Exec(); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	likes := incrCmd.Val()
	_ = zincrCmd.Val() // score，暂不使用单独返回

	// 同步记录到 MySQL（简单记录一条点赞历史），可以改为发送到 MQ
	like := models.Like{
		UserID:    uint(userID),
		ArticleID: uint(articleID),
	}
	if err := global.Db.Create(&like).Error; err != nil {
		// 记录失败不影响主流程，但返回信息中可提示
		ctx.JSON(http.StatusOK, gin.H{"message": "liked (redis ok), db record failed", "likes": likes, "db_error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "Successfully liked the article", "likes": likes})
}

// GetArticleLikes: 从 Redis 获取单篇文章点赞数
func GetArticleLikes(ctx *gin.Context) {
	articleID := ctx.Param("id")

	likeKey := "article:" + articleID + ":likes"

	likes, err := global.RedisDB.Get(likeKey).Result()

	if err == redis.Nil {
		likes = "0"
	} else if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"likes": likes})
}

// GetTopArticles: 返回 Top N 排行（从 Redis ZSET 获取，并尝试查询文章标题）
func GetTopArticles(ctx *gin.Context) {
	topStr := ctx.DefaultQuery("top", "10")
	top, err := strconv.Atoi(topStr)
	if err != nil || top <= 0 {
		top = 10
	}

	rankKey := "rank:article:likes"
	// ZREVRANGE with scores
	zres, err := global.RedisDB.ZRevRangeWithScores(rankKey, 0, int64(top-1)).Result()
	if err != nil {
		// 如果 key 不存在，也返回空列表
		if err == redis.Nil {
			ctx.JSON(http.StatusOK, gin.H{"list": []string{}})
			return
		}
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	list := make([]map[string]interface{}, 0, len(zres))
	for idx, z := range zres {
		memberStr, _ := z.Member.(string)
		score := int64(z.Score)
		item := map[string]interface{}{"id": memberStr, "score": score, "rank": idx + 1}
		// 尝试从 DB 查询文章标题（容错）
		var art models.Article
		if err := global.Db.First(&art, memberStr).Error; err == nil {
			item["title"] = art.Title
		}
		list = append(list, item)
	}

	ctx.JSON(http.StatusOK, gin.H{"list": list})
}
