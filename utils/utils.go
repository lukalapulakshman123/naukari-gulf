package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	redis "github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func GetLogger() *logrus.Logger {
	var log = logrus.New()
	// Set log format to JSON (better for structured logging)
	log.SetFormatter(&logrus.JSONFormatter{})

	// Output logs to both stdout and a file
	file, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(file) // Log to file
	} else {
		panic(err)
	}
	log.SetLevel(logrus.InfoLevel) // Set log level
	return log
}

func GetDb() *gorm.DB {
	dsn := "host=localhost user=myuser password=mypassword dbname=mydb port=5432 sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	return db
}

// Initialize Redis connection
func GetRedisClient() *redis.Client {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Default Redis port
		Password: "foobared",       // No password by default
		DB:       0,                // Default DB
	})
	return redisClient
}

func GetKafkaProducer() *kafka.Producer {
	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":            "localhost:9092",
		"queue.buffering.max.messages": 1000000,  // Default: 100000 (Increase if needed)
		"queue.buffering.max.kbytes":   102400,   // Default: 1024 KB (Increase if needed)
		"queue.buffering.max.ms":       10,       // Default: 5ms (Reduce if high latency)
		"batch.num.messages":           10000,    // Default: 1000 (Larger batch size)
		"compression.type":             "snappy", // Compress messages to reduce size
		"linger.ms":                    10,       // Delay sending to optimize batching
	})
	if err != nil {
		panic(err)
	}
	return producer
}

func ProduceMessage(topic string, p *kafka.Producer, message any) error {
	jsonMsg, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	err = p.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          jsonMsg,
	}, nil)

	if err != nil {
		return err
	}
	return nil
}

// Set a cache entry
func SetCache(key string, value interface{}, expiration time.Duration, redisClient *redis.Client, ctx context.Context) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	err = redisClient.Set(ctx, key, data, expiration).Err()
	if err != nil {
		return err
	}
	return nil
}

// Get a cache entry
func GetCache(key string, dest interface{}, redisClient *redis.Client, ctx context.Context) bool {
	data, err := redisClient.Get(ctx, key).Result()
	if err == redis.Nil {
		return false // Cache miss
	} else if err != nil {
		return false
	}

	err = json.Unmarshal([]byte(data), dest)
	if err != nil {
		return false
	}
	return true // Cache hit
}
