package main

import (
	"github.com/go-redis/redis"
	"log"
	"os"
)

const localRedisUrl string = "redis://localhost:6379"

func NewRedis() *redis.Client {
	url := localRedisUrl
	// TODO: This probably doesn't belong here
	envRedisUrl := os.Getenv("REDIS_URL")

	if len(envRedisUrl) > 0 {
		url = envRedisUrl
	}

	options, err := redis.ParseURL(url)

	if err != nil {
		log.Panic("Unable to parse error. Error: ", err.Error())
	}

	// TODO: Lets add some connection pooling here
	client := redis.NewClient(options)

	pong := client.Ping()

	if pong.Err() != nil {
		log.Printf("Unable to connect to redis at %s", options.Addr)
	}

	return client
}
