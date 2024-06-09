package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

type Comment struct {
	Id          int       `json:"id"`
	By          string    `json:"by"`
	Time        uint32    `json:"time"`
	Descendants uint16    `json:"descendants"`
	Kids        []int     `json:"kids"`
	Comments    []Comment `json:"comments"`
	Text        string    `json:"text"`
	Parent      int       `json:"parent"`
}

type Story struct {
	Id          int       `json:"id"`
	By          string    `json:"by"`
	Score       uint16    `json:"score"`
	Title       string    `json:"title"`
	Time        uint32    `json:"time"`
	Descendants uint16    `json:"descendants"`
	Url         string    `json:"url"`
	Kids        []int     `json:"kids"`
	Comments    []Comment `json:"comments"`
	Text        string    `json:"text"`
}

type ShortStory struct {
	Id          int    `json:"id"`
	By          string `json:"by"`
	Score       uint16 `json:"score"`
	Title       string `json:"title"`
	Time        uint32 `json:"time"`
	Descendants uint16 `json:"descendants"`
}

type itemType struct {
	Type string `json:"type"`
}

type MutexSlice[T any] struct {
	mutn  sync.Mutex
	slice []T
}

func deleteFromCache(ids []int, ctx context.Context) {
	redis := mustGetRedis(ctx)
	for _, id := range ids {
		commentStr, err := redis.Get(ctx, fmt.Sprint(id)).Result()
		if err != nil {
			continue
		}
		comment := Comment{}
		err = json.Unmarshal([]byte(commentStr), &comment)
		if err == nil {
			deleteFromCache(comment.Kids, ctx)
		}
		redis.Del(ctx, fmt.Sprint(comment.Id))
		log.Debug().Msgf("item %d deleted from cache", id)
	}
}

type ReqContext struct {
	status int
}

func mustGetReqCtx(ctx context.Context) *ReqContext {
	reqctx, ok := ctx.Value("pContext").(*ReqContext)
	if !ok {
		log.Panic().Caller(1).Msg("No Request Context in Context")
	}
	return reqctx
}

func contextHandler(fn func(w http.ResponseWriter, r *http.Request, ctx context.Context) error, ctx context.Context) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		reqctx := ReqContext{status: http.StatusTeapot}
		ctx = context.WithValue(ctx, "pContext", &reqctx)
		w.Header().Add("Access-Control-Allow-Origin", "*")
		fn(w, r, ctx)
		if reqctx.status == http.StatusTeapot {
			log.Warn().Msgf("Request to %s returned default status", r.RequestURI)
		}
		if reqctx.status != -1 {
			w.WriteHeader(reqctx.status)
		}
		return

	}
}

func mustGetRedis(ctx context.Context) *redis.Client {
	redisClient, ok := ctx.Value("redis").(*redis.Client)
	if !ok {
		log.Panic().Msg("Redis not in context!")
	}
	return redisClient
}

func fetchItem(id int, ctx context.Context) ([]byte, error) {
	redisClient := mustGetRedis(ctx)
	reqctx := mustGetReqCtx(ctx)
	data, err := redisClient.Get(ctx, fmt.Sprint(id)).Result()
	if err == nil {
		log.Debug().Msgf("Request for item %i served from cache", id)
		return []byte(data), nil
	}
	req, err := retry(func() (*http.Response, error) {
		return http.Get(BASE + ITEM + fmt.Sprint(id) + EXT)
	}, 3)

	if err != nil {
		reqctx.status = http.StatusServiceUnavailable
		return nil, err
	}
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		reqctx.status = http.StatusInternalServerError
		log.Error().Err(err).Caller(1).Msgf("Bad response to request for %i", id)
		return nil, err
	}
	redisClient.Set(ctx, fmt.Sprint(id), string(bodyBytes), 10*time.Minute)
	return bodyBytes, nil
}

func getReqId(w http.ResponseWriter, r *http.Request) (int, error) {
	idStr := r.PathValue("id")
	if idStr == "" {
		log.Debug().Msgf("Request without id to %s", r.RequestURI)
		return 0, fmt.Errorf("Bad Request")
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		log.Debug().Err(err).Msgf("Request with invalid id to %s", r.RequestURI)
		return 0, fmt.Errorf("BadRequest")
	}
	return int(id), nil
}

func retry(fn func() (*http.Response, error), n int) (*http.Response, error) {
	i := 0
	var res *http.Response
	var err error
	for i < n {
		res, err = fn()
		if err == nil {
			return res, nil
		}
		i++
	}
	log.Error().Err(err).Caller().Msgf("Unable to makke response after %i attempts", n)
	return res, err
}

func shortenStories(fulls []byte) ([]byte, error) {
	shorts := []ShortStory{}
	err := json.Unmarshal(fulls, &shorts)
	if err != nil {
		log.Error().Err(err).Caller().Msgf("Error unmarshaling stories")
		return nil, fmt.Errorf("Server Error")
	}
	newsBytes, err := json.Marshal(shorts)
	if err != nil {
		log.Error().Err(err).Caller().Msgf("Error marshaling stories")
		return nil, fmt.Errorf("Server Error")
	}
	return newsBytes, nil
}

func mustGetEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Panic().Msgf("Key %s not found in env", key)
	}
	return val
}
