package main

import (
	"context"

	"github.com/go-redis/redis/v8"
)

type RedisCli struct {
	client *redis.Client
}

func NewRedisCli() *RedisCli {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Replace with your Redis server's address
		Password: "",               // No password set
		DB:       0,                // Use default DB
	})

	return &RedisCli{
		client: client,
	}
}

func (c *RedisCli) Close() {
	c.client.Close()
}

func (c *RedisCli) GetValueByKey(key string) (string, error) {
	// Get the value of a key
	value, err := c.client.Get(context.Background(), key).Result()
	if err != nil {
		return "", err
	}

	return value, nil
}

func (c *RedisCli) SetValueByKey(key, value string) error {
	// Set a key-value pair
	err := c.client.Set(context.Background(), key, value, 0).Err()
	if err != nil {
		return err
	}

	return nil
}
