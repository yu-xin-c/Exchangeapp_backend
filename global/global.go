package global

import (
	"github.com/go-redis/redis"
	"gorm.io/gorm"
	amqp "github.com/rabbitmq/amqp091-go"
)

var(
	Db *gorm.DB
	RedisDB *redis.Client
	RabbitConn *amqp.Connection
	RabbitChannel *amqp.Channel
)