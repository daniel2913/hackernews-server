package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
)

func getFullStoryHandler(w http.ResponseWriter, r *http.Request, ctx context.Context) error {
	reqctx := mustGetReqCtx(ctx)
	id, err := getReqId(w, r)
	if err != nil {
		reqctx.status = http.StatusBadRequest
		log.Debug().Msgf("Request to %s has invalid id", r.RequestURI)
		return err
	}
	story, err := getStory(ctx, int(id))
	if err != nil {
		return err
	} else if story.Id == 0 {
		log.Debug().Caller().Msgf("Story with id == 0")
		return fmt.Errorf("Not found")
	}

	storyBytes, err := json.Marshal(story)
	if err != nil {
		reqctx.status = http.StatusInternalServerError
		log.Error().Err(err).Caller()
		return err
	}
	reqctx.status = -1
	w.Write(storyBytes)
	return nil
}

func updateCommentsHandler(w http.ResponseWriter, r *http.Request, ctx context.Context) error {
	id, err := getReqId(w, r)
	reqctx := mustGetReqCtx(ctx)
	if err != nil {
		reqctx.status = http.StatusBadRequest
		return err
	}
	redis := mustGetRedis(ctx)
	_, err = redis.Get(ctx, fmt.Sprint(id)).Result()
	if err == nil {
		story, err := getStory(ctx, int(id))
		if err != nil {
			log.Error().Err(err).Msgf("story %d is in database, but can't be obtained", id)
			reqctx.status = http.StatusInternalServerError
			return err
		}
		deleteFromCache(story.Kids, ctx)
		redis.Del(ctx, fmt.Sprint(story.Id))
	}
	story, err := getStory(ctx, int(id))
	if err != nil {
		return err
	}
	storyBytes, err := json.Marshal(story)
	if err != nil {
		log.Error().Err(err).Caller()
		reqctx.status = http.StatusInternalServerError
		return err
	}
	reqctx.status = -1
	redis.Set(ctx, fmt.Sprint(story.Id), string(storyBytes), 1*time.Minute)
	return nil
}

func getCommentsHandler(w http.ResponseWriter, r *http.Request, ctx context.Context) error {
	id, err := getReqId(w, r)
	reqctx := mustGetReqCtx(ctx)
	if err != nil {
		reqctx.status = http.StatusBadRequest
		return err
	}
	comments, err := getComments(int(id), ctx)
	if err != nil {
		return err
	}
	reqctx.status = -1
	w.Write(comments)
	return nil
}

func corsHandler(w http.ResponseWriter, r *http.Request, ctx context.Context) error {
	return nil
}

func generateHandlers(names []string, ctx context.Context) {
	for _, name := range names {

		ticker := time.NewTicker(1 * time.Minute)
		counter := 0

		get := func(w http.ResponseWriter, r *http.Request, _ctx context.Context) error {
			counter = 3
			w.Header().Add("Cache-Control", "max-age=50, stale-while-revalidate=120")
			return getStoriesGeneral(w, r, _ctx, name)
		}

		reload := func(w http.ResponseWriter, _ *http.Request, _ctx context.Context) error {
			counter = 3
			w.Header().Add("Clear-Site-Data", "cache")
			reqctx := mustGetReqCtx(_ctx)
			reqctx.status = -1
			return refreshStoriesGeneral(_ctx, name)
		}

		http.HandleFunc("GET /api/"+name, contextHandler(get, ctx))
		http.HandleFunc("POST /api/"+name, contextHandler(reload, ctx))

		go func(name string, ctx context.Context) {
			for range ticker.C {
				if counter > 0 {
					counter--
					dummyctx := ReqContext{status: -1}
					ctx = context.WithValue(ctx, "pContext", &dummyctx)
					refreshStoriesGeneral(ctx, name)
				}
			}
		}(name, ctx)
	}
}
