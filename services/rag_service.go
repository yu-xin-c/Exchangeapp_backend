package services

// RAGService 封装Eino RAG功能的服务
type RAGService struct {
	// 当前实现为空，可按需扩展
}

// NewRAGService 创建RAG服务实例
func NewRAGService() (*RAGService, error) {
	return &RAGService{}, nil
}

// AnswerQuestion 使用RAG系统回答问题
func (s *RAGService) AnswerQuestion(question string) (string, []ArticleSummary, error) {
	// 暂不实现完整RAG
	return "", []ArticleSummary{}, nil
}
