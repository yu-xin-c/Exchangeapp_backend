package services

import (
	"exchangeapp/global"
	"exchangeapp/models"
	"fmt"
	"strings"
)

type ArticleSummary struct {
	ID      uint   `json:"id"`
	Title   string `json:"title"`
	Preview string `json:"preview"`
}

// RAG服务实例
var ragService *RAGService

// InitRAGService 初始化RAG服务
func InitRAGService() error {
	var err error
	ragService, err = NewRAGService()
	return err
}

func AnswerQuestion(question string, topK int) (string, []ArticleSummary, error) {
	// 优先使用RAG服务
	if ragService != nil {
		return ragService.AnswerQuestion(question)
	}

	// 回退到简单检索实现
	return answerWithSimpleRetrieval(question, topK)
}

// answerWithSimpleRetrieval 简单的基于关键词检索的回答
func answerWithSimpleRetrieval(question string, topK int) (string, []ArticleSummary, error) {
	// Simple retrieval: search articles by title/content containing keywords
	var articles []models.Article
	keywords := strings.Fields(question) // split by space
	query := global.Db.Model(&models.Article{})
	for _, kw := range keywords {
		query = query.Where("title LIKE ? OR content LIKE ?", "%"+kw+"%", "%"+kw+"%")
	}
	if err := query.Limit(topK).Find(&articles).Error; err != nil {
		return "", nil, err
	}

	if len(articles) == 0 {
		return "抱歉，没有找到相关文章。", nil, nil
	}

	// Build context from retrieved articles
	var context strings.Builder
	var sources []ArticleSummary
	for _, art := range articles {
		context.WriteString(fmt.Sprintf("标题: %s\n内容: %s\n\n", art.Title, art.Content))
		sources = append(sources, ArticleSummary{ID: art.ID, Title: art.Title, Preview: art.Preview})
	}

	// Build prompt for AI
	prompt := fmt.Sprintf("基于以下文档回答问题：\n\n%s\n\n问题：%s\n\n请提供简洁的中文回答。", context.String(), question)

	// Call AI (or fallback)
	answer, err := CallOpenAI(prompt)
	if err != nil {
		return "", nil, err
	}

	return answer, sources, nil
}
