package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

func validateVideo(url string) error {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		panic(err)
	}
	req.Header.Set("Referer", url)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		return errors.New("invalid URL, not returning 200 response or 206 response")
	}
	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return nil
}

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
		err = validateVideo(tiktokVid)
		if err != nil {
			panic(err)
		}
		err = rdb.Set(ctx, key.String(), tiktokVid, 0).Err()
		if err != nil {
			panic(err)
		}
		fmt.Printf("Set %s value to %s\n", key.String(), tiktokVid)
	}
}
