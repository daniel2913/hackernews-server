package main

import (
	"context"
	"embed"
	"net/http"

	"github.com/redis/go-redis/v9"
)

var base = "https://hacker-news.firebaseio.com/v0/"
var item = "item/"
var ext = ".json"

//go:embed dist/*
var app embed.FS

func main() {
	redisClient := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "",
		DB:       0,
	})

	ctx := context.WithValue(context.Background(), "redis", redisClient)
	server := http.Server{Addr: "localhost:3005"}

	paths := []string{"newstories", "topstories", "beststories"}
	generateHandlers(paths, ctx)

	http.HandleFunc("INFO /*", contextHandler(corsHandler, ctx))

	http.HandleFunc("GET /*", SPAHandler)

	http.HandleFunc("GET /api/story/{id}", contextHandler(getFullStoryHandler, ctx))
	http.HandleFunc("GET /api/comments/{id}", contextHandler(getCommentsHandler, ctx))
	http.HandleFunc("POST /api/comments/{id}", contextHandler(updateCommentsHandler, ctx))
	err := server.ListenAndServe()
	println(err.Error())
}
