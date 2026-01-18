package controllers

import (
	"exchangeapp/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

type QARequest struct {
	Question string `json:"question" binding:"required"`
	TopK     int    `json:"topk"`
}

type QAResponse struct {
	Answer  string                    `json:"answer"`
	Sources []services.ArticleSummary `json:"sources"`
}

func AnswerQuestion(c *gin.Context) {
	var req QARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.TopK <= 0 {
		req.TopK = 3
	}

	ans, sources, err := services.AnswerQuestion(req.Question, req.TopK)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, QAResponse{Answer: ans, Sources: sources})
}
