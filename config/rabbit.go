package config

import (
    "log"
    "exchangeapp/global"

    amqp "github.com/rabbitmq/amqp091-go"
)

func initRabbit(){
    url := AppConfig.RabbitMQ.Url
    if url == "" {
        log.Println("rabbitmq url empty, skipping rabbit init")
        return
    }

    conn, err := amqp.Dial(url)
    if err != nil {
        log.Fatalf("Failed to connect to RabbitMQ: %v", err)
    }

    ch, err := conn.Channel()
    if err != nil {
        log.Fatalf("Failed to open RabbitMQ channel: %v", err)
    }

    // declare queue
    qname := AppConfig.RabbitMQ.Queue
    if qname == "" {
        qname = "like.queue"
    }
    _, err = ch.QueueDeclare(qname, true, false, false, false, nil)
    if err != nil {
        log.Fatalf("Failed to declare RabbitMQ queue: %v", err)
    }

    global.RabbitConn = conn
    global.RabbitChannel = ch
    log.Println("RabbitMQ initialized, queue:", qname)
}
