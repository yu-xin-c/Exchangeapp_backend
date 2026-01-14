package models

import "gorm.io/gorm"

// Like 表示用户对文章的点赞记录（用于持久化审计/去重/分析）
type Like struct {
	gorm.Model
	UserID    uint `gorm:"index"`
	ArticleID uint `gorm:"index"`
}
