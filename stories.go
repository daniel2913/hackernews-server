package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

func getStory(ctx context.Context, id int) (Story, error) {
	story := Story{}
	reqctx := mustGetReqCtx(ctx)
	itemBytes, err := fetchItem(id, ctx)
	if err != nil {
		return story, fmt.Errorf("Server Error")
	}
	err = json.Unmarshal(itemBytes, &story)
	if err != nil {
		log.Error().Err(err).Caller(1).Msg("Error unmarshaling fetched story")
		reqctx.status = http.StatusInternalServerError
		return story, fmt.Errorf("Server Error")
	}
	if story.Id == 0 {
		log.Debug().Msg("Got story with id == 0")
		reqctx.status = http.StatusNotFound
		return story, fmt.Errorf("Not Found")
	}
	return story, nil
}

func getStoryList(ctx context.Context, name string) ([]byte, error) {
	redis := mustGetRedis(ctx)
	news := make([]Story, 100, 100)
	reqctx := mustGetReqCtx(ctx)
	ids, err := fetchStoryList(ctx, name)
	if err != nil {
		return nil, err
	}
	wg := sync.WaitGroup{}
	for idx, id := range ids[:int(math.Min(float64(100), float64(len(ids))))] {
		wg.Add(1)
		go func(idx int, id int) {
			defer wg.Done()
			story, err := getStory(ctx, id)
			if err != nil {
				return
			}
			news[idx] = story
		}(idx, id)
	}
	wg.Wait()
	sort.Slice(news, func(i, j int) bool {
		return news[i].Time > news[j].Time
	})
	newsBytes, err := json.Marshal(news)
	if err != nil {
		log.Error().Err(err).Caller()
		return nil, err
	}
	reqctx.status = http.StatusOK
	redis.Set(ctx, name, string(newsBytes), time.Minute)
	return newsBytes, nil
}

func fetchStoryList(ctx context.Context, name string) ([]int, error) {
	redisClient := mustGetRedis(ctx)
	reqctx := mustGetReqCtx(ctx)
	data, err := redisClient.Get(ctx, name+"ids").Result()
	if err == nil {
		res := make([]int, 0)
		err = json.Unmarshal([]byte(data), &res)
		if err == nil {
			log.Error().Err(err).Caller()
			reqctx.status = http.StatusInternalServerError
			return res, nil
		}
	}
	req, err := retry(func() (*http.Response, error) {
		return http.Get(BASE + name + EXT)
	}, 3)

	if err != nil {
		reqctx.status = http.StatusServiceUnavailable
		log.Debug().Err(err).Caller()
		return nil, err
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		log.Error().Err(err).Caller()
		reqctx.status = http.StatusInternalServerError
		return nil, err
	}
	ids := make([]int, 0, 200)
	err = json.Unmarshal(body, &ids)
	if err != nil {
		log.Error().Err(err).Caller()
		reqctx.status = http.StatusInternalServerError
		return nil, err
	}
	w, err := json.Marshal(ids)
	if err != nil {
		reqctx.status = http.StatusInternalServerError
		log.Error().Err(err).Caller()
		return nil, err
	}
	redisClient.Set(ctx, name+"ids", string(w), time.Minute)
	return ids, nil
}

func getStoriesGeneral(w http.ResponseWriter, _ *http.Request, ctx context.Context, name string) error {
	redis := mustGetRedis(ctx)
	reqctx := mustGetReqCtx(ctx)
	saved, err := redis.Get(ctx, name).Result()
	if err == nil {
		res, err := shortenStories([]byte(saved))
		if err != nil {
			reqctx.status = http.StatusInternalServerError
			log.Error().Err(err).Caller()
			return err
		}
		reqctx.status = -1
		w.Write(res)
		return err
	}
	respBytes, err := getStoryList(ctx, name)
	if err != nil {
		http.Error(w, "Server Error", 500)
	}
	reqctx.status = -1
	w.Write(respBytes)
	return nil
}

func refreshStoriesGeneral(ctx context.Context, name string) error {
	redis := mustGetRedis(ctx)
	log.Debug().Msgf("%s list refreshed", name)
	redis.Del(ctx, name)
	getStoryList(ctx, name)
	return nil
}
