package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var base = "https://hacker-news.firebaseio.com/v0/"
var newIds = "newstories"
var item = "item/"

var ext = ".json"

type ids []uint32

func (this *ids) Write(targ io.Writer) (int, error) {
	s, err := json.Marshal(this)
	if err != nil {
		return 0, err
	}
	return targ.Write(s)
}

func (this *ids) Read(src []byte) (int, error) {
	err := json.Unmarshal(src, this)
	if err != nil {
		return 0, err
	}
	return len(src), nil
}

func fetchIds(ctx context.Context) ([]uint32, error) {
	redisClient, ok := ctx.Value("redis").(*redis.Client)
	if !ok {
		fmt.Println(redisClient)
		panic(":(")
	}
	data, err := redisClient.Get(ctx, "newIds").Result()
	if err == nil {
		var res ids
		_, err = res.Read([]byte(data))
		if err == nil {
			return res, nil
		}
	}
	req, err := http.Get(base + newIds + ext)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	ids := make([]uint32, 0, 200)

	err = json.Unmarshal(body, &ids)
	if err != nil {
		return nil, err
	}
	w, err := json.Marshal(ids)
	if err == nil {
		redisClient.Set(ctx, "newIds", string(w), time.Duration(1000*1000*1000*60))
	} else {
		println("Redit write error")
	}
	return ids, nil
}

func main() {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "",
		DB:       0,
	})
	ctx := context.WithValue(context.Background(), "redis", redisClient)
	ids, err := fetchIds(ctx)
	if err != nil {
		panic("fetchIds")
	}
	res := make([]byte, 0, 102400)
	res = append(res, '[')
	cur := make(chan []byte)
	wg := sync.WaitGroup{}
	for _, id := range ids[0:99] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fetchItem(ctx, id, cur)
		}()
	}
	go func() {
		wg.Wait()
		close(cur)
	}()
	for story := range cur {
		println(story)
		res = append(res, story...)
		res = append(res, ',')
	}
	res = res[0 : len(res)-1]
	res = append(res, ']')
	fmt.Println(string(res))
}
