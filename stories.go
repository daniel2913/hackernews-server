package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
)

func getRedis(ctx context.Context) *redis.Client {
	redisClient, ok := ctx.Value("redis").(*redis.Client)
	if !ok {
		fmt.Println(redisClient)
		panic(":(")
	}
	return redisClient
}

func fetchItem(ctx context.Context, id uint32, out chan []byte) {

	redisClient := getRedis(ctx)
	data, err := redisClient.Get(ctx, fmt.Sprint(id)).Result()
	if err == nil {
		out <- []byte(data)
		return
	}

	req, err := http.Get(base + item + fmt.Sprint(id) + ext)
	if err != nil {
	}
	body := make([]byte, 0, 1024)
	search := make([]byte, 14, 14)
	prev := search[:6]
	cur := search[6:]

	_, err = req.Body.Read(prev)
	if err != nil {
		return
	}

	text := []byte(`"text"`)
	for {
		_, err := req.Body.Read(search)
		if err != nil {
			if err == io.EOF {
				break
			}
			return
		}
		idx := bytes.Index(search, text)
		if idx == -1 {
			body = append(body, prev...)
			prev = cur
			cur = nil
			continue
		}
		body = append(body, prev[:idx-1]...)
		char := make([]byte, 1, 1)
		for char[0] != '"' {
			req.Body.Read(char)
		}
		char[0] = ' '
		for char[0] != '"' {
			req.Body.Read(char)
		}
		prev = nil
		cur = nil
		req.Body.Read(prev)
	}
	body = append(body, prev...)
	redisClient.Set(ctx, fmt.Sprint(id), body, time.Duration(1000*1000*1000*60*3))
	out <- body
}
