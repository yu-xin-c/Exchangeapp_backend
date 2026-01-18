package config

import (
	"log"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	App struct {
		Name string
		Port string
	}
	Database struct {
		Dsn          string
		MaxIdleConns int
		MaxOpenConns int
	}
	Redis struct {
		Addr     string
		DB       int
		Password string
	}
	RabbitMQ struct {
		Url   string
		Queue string
	}
	RAG struct {
		ArkAPIKey         string
		ArkChatModel      string
		ArkEmbeddingModel string
		ArkAPIBaseURL     string
		RedisAddr         string
		TopK              int
	}
}

var AppConfig *Config

func InitConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yml")
	viper.AddConfigPath("./config")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	AppConfig = &Config{}

	if err := viper.Unmarshal(AppConfig); err != nil {
		log.Fatalf("Unable to decode into struct: %v", err)
	}

	// 初始化RAG配置
	AppConfig.RAG.ArkAPIKey = getEnvOrDefault("ARK_API_KEY", "")
	AppConfig.RAG.ArkChatModel = getEnvOrDefault("ARK_CHAT_MODEL", "doubao-pro-32k-241215")
	AppConfig.RAG.ArkEmbeddingModel = getEnvOrDefault("ARK_EMBEDDING_MODEL", "doubao-embedding-large-text-240915")
	AppConfig.RAG.ArkAPIBaseURL = getEnvOrDefault("ARK_API_BASE_URL", "https://ark.cn-beijing.volces.com/api/v3")
	AppConfig.RAG.RedisAddr = getEnvOrDefault("REDIS_ADDR", "localhost:6379")
	AppConfig.RAG.TopK = 3 // 默认值

	initDB()
	initRedis()
	initRabbit()
}

// getEnvOrDefault 获取环境变量，如果不存在则返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
