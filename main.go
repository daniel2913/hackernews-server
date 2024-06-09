package main

import (
	"context"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	BASE = "https://hacker-news.firebaseio.com/v0/"
	ITEM = "item/"
	EXT  = ".json"
)

func main() {

	err := godotenv.Load(".env")
	if err != nil {
		log.Panic().Err(err).Msgf("Error getting enviroment variables from .env")
	}

	lvl := os.Getenv("LOG_VERB")
	if lvl == "DEBUG" {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else if lvl == "INFO" {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	} else {
		lvl = "ERROR"
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	}
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Info().Msgf("Log Level: %s", lvl)

	log.Info().Msg("Loaded .env")

	validateClient()

	redisClient := redis.NewClient(&redis.Options{
		Addr:     os.Getenv("REDIS_ADDRES") + ":" + os.Getenv("REDIS_PORT"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	})

	ctx := context.WithValue(context.Background(), "redis", redisClient)

	err = redisClient.Echo(ctx, "test").Err()
	if err != nil {
		log.Panic().Err(err).Msgf("Couldn't connect to redis database")
	}
	log.Info().Msg("Connected to redis database")

	server := http.Server{Addr: mustGetEnv("ADDRES")}

	paths := []string{"newstories", "topstories", "beststories"}
	generateHandlers(paths, ctx)

	http.HandleFunc("INFO /*", contextHandler(corsHandler, ctx))

	http.HandleFunc("GET /*", SPAHandler)

	http.HandleFunc("GET /api/story/{id}", contextHandler(getFullStoryHandler, ctx))
	http.HandleFunc("GET /api/comments/{id}", contextHandler(getCommentsHandler, ctx))
	http.HandleFunc("POST /api/comments/{id}", contextHandler(updateCommentsHandler, ctx))
	log.Info().Msgf("Listening at %s ...", server.Addr)
	log.Panic().Err(server.ListenAndServe())
}
