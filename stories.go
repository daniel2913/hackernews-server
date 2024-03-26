package main

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"
)

func getStory(ctx context.Context, id int) (Story, error) {
	story := Story{}
	itemBytes, err := fetchItem(ctx, id)
	if err != nil {
		return story, ServerError
	}
	err = json.Unmarshal(itemBytes, &story)
	if err != nil {
		return story, ServerError
	}
	if story.Id == 0 {
		return story, NotFound
	}
	return story, nil
}

func getStoryList(ctx context.Context, name string) ([]byte, error) {
	redis := getRedis(ctx)
	news := make([]Story, 100, 100)
	ids, err := fetchStoryList(ctx, name)
	if err != nil {
		return nil, err
	}
	wg := sync.WaitGroup{}
	for idx, id := range ids[:int(math.Min(float64(100), float64(len(ids))))] {
		wg.Add(1)
		go func() {
			defer wg.Done()
			story, err := getStory(ctx, id)
			if err != nil {
				return
			}
			news[idx] = story
		}()
	}
	wg.Wait()
	sort.Slice(news, func(i, j int) bool {
		return news[i].Time > news[j].Time
	})
	newsBytes, err := json.Marshal(news)
	if err != nil {
		return nil, err
	}
	redis.Set(ctx, name, string(newsBytes), time.Minute)
	return newsBytes, nil
}

func fetchStoryList(ctx context.Context, name string) ([]int, error) {
	redisClient := getRedis(ctx)
	data, err := redisClient.Get(ctx, name+"ids").Result()
	if err == nil {
		res := make([]int, 0)
		err = json.Unmarshal([]byte(data), &res)
		if err == nil {
			return res, nil
		}
	}
	req, err := http.Get(base + name + ext)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, 200)
	err = json.Unmarshal(body, &ids)
	if err != nil {
		return nil, err
	}
	w, err := json.Marshal(ids)
	if err == nil {
		redisClient.Set(ctx, name+"ids", string(w), time.Minute)
	} else {
		panic("Redis write error")
	}
	return ids, nil
}

func getStoriesGeneral(w http.ResponseWriter, _ *http.Request, ctx context.Context, name string) {
	redis := getRedis(ctx)
	saved, err := redis.Get(ctx, name).Result()
	if err == nil {
		res, err := shortenStories([]byte(saved))
		if err != nil {
			serverError(w, err)
		}
		w.Write(res)
		return
	}
	respBytes, err := getStoryList(ctx, name)
	if err != nil {
		http.Error(w, "Server Error", 500)
	}
	w.Write(respBytes)

}

func refreshStoriesGeneral(ctx context.Context, name string) {
	redis := getRedis(ctx)
	redis.Del(ctx, name)
	getStoryList(ctx, name)
	return
}
