package main

import (
	"context"
	"fmt"
	"os"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Invalid usage, expected: ./manage <URL_TO_ADD_TO_DB>")
	} else {
		var ctx = context.TODO()
		var rdb = redis.NewClient(&redis.Options{
			Addr:     os.Getenv("REDISHOST"),
			Password: "", // no password set
			DB:       0,  // use default DB
		})
		key, err := uuid.NewRandom()
		if err != nil {
			panic(err)
		}
		tiktokVid := os.Args[1]
		err = rdb.Set(ctx, key.String(), tiktokVid, 0).Err()
		if err != nil {
			panic(err)
		}
		fmt.Printf("Set %s value to %s\n", key.String(), tiktokVid)
	}
}
