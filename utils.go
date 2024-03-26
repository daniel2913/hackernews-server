package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
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
	redis := getRedis(ctx)
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
	}
}

func contextHandler(fn func(w http.ResponseWriter, r *http.Request, ctx context.Context), ctx context.Context) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Access-Control-Allow-Origin", "*")
		fn(w, r, ctx)
	}
}
func getRedis(ctx context.Context) *redis.Client {
	redisClient, ok := ctx.Value("redis").(*redis.Client)
	if !ok {
		panic("Redis not available!")
	}
	return redisClient
}

func fetchItem(ctx context.Context, id int) ([]byte, error) {
	redisClient := getRedis(ctx)
	data, err := redisClient.Get(ctx, fmt.Sprint(id)).Result()
	if err == nil {
		return []byte(data), nil
	}
	req, err := retry(func() (*http.Response, error) {
		return http.Get(base + item + fmt.Sprint(id) + ext)
	}, 5)

	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	redisClient.Set(ctx, fmt.Sprint(id), string(bodyBytes), 1*time.Minute)
	return bodyBytes, nil
}

func serverError(w http.ResponseWriter, err error) {
	switch err.Error() {
	case "Not Found":
		http.Error(w, "Not Found", 404)
		return
	case "Bad Request":
		http.Error(w, "Bad Request", 400)
		return
	case "Server Error":
		http.Error(w, "Internal Server Error", 500)
		return
	case "Unavailable":
		http.Error(w, "Service Unavailable", 503)
		return
	default:
		http.Error(w, "Unknown Error", 500)
		return
	}
}

var NotFound = errors.New("Not Found")
var BadRequest = errors.New("Bad Request")
var ServerError = errors.New("Server Error")
var Unavailable = errors.New("Unavailable")

func getReqId(w http.ResponseWriter, r *http.Request) int {
	idStr := r.PathValue("id")
	if idStr == "" {
		serverError(w, BadRequest)
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		serverError(w, BadRequest)
	}
	return int(id)
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
	return res, err
}

func shortenStories(fulls []byte) ([]byte, error) {
	shorts := []ShortStory{}
	err := json.Unmarshal(fulls, &shorts)
	if err != nil {
		return nil, ServerError
	}
	newsBytes, err := json.Marshal(shorts)
	if err != nil {
		return nil, ServerError
	}
	return newsBytes, nil
}
