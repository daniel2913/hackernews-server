package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func SPAHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "max-age=24939360")
	if len(strings.Split(r.URL.Path, ".")) > 1 {
		http.ServeFileFS(w, r, app, "dist"+r.URL.Path)
		return
	}
	http.ServeFileFS(w, r, app, "dist/index.html")
}

func getFullStoryHandler(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	id := getReqId(w, r)
	story, err := getStory(ctx, int(id))
	if err != nil || story.Id == 0 {
		serverError(w, NotFound)
		return
	}
	storyBytes, err := json.Marshal(story)
	if err != nil {
		serverError(w, ServerError)
		return
	}
	w.Write(storyBytes)
}

func updateCommentsHandler(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	id := getReqId(w, r)
	redis := getRedis(ctx)
	_, err := redis.Get(ctx, fmt.Sprint(id)).Result()

	if err == nil {
		story, err := getStory(ctx, int(id))
		if err != nil {
			serverError(w, NotFound)
			return
		}
		deleteFromCache(story.Kids, ctx)
		redis.Del(ctx, fmt.Sprint(story.Id))
	}
	story, err := getStory(ctx, int(id))
	if err != nil {
		http.Error(w, "Not Found", 404)
		return
	}
	storyBytes, err := json.Marshal(story)
	if err != nil {
		http.Error(w, "Server Error", 500)
		return
	}
	redis.Set(ctx, fmt.Sprint(story.Id), string(storyBytes), 1*time.Minute)
	return
}

func getCommentsHandler(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	id := getReqId(w, r)
	comments, err := getComments(int(id), ctx)
	if err != nil {
		serverError(w, err)
	}
	w.Write(comments)
}


func corsHandler(w http.ResponseWriter, r *http.Request, ctx context.Context) {
	return
}

func generateHandlers(names []string, ctx context.Context) {
	for _, name := range names {

		ticker := time.NewTicker(1 * time.Minute)
		counter := 0

		get := func(w http.ResponseWriter, r *http.Request, ctx context.Context) {
			counter = 3
			w.Header().Add("Cache-Control", "max-age=50, stale-while-revalidate=120")
			getStoriesGeneral(w, r, ctx, name)
		}

		reload := func(w http.ResponseWriter, _ *http.Request, ctx context.Context) {
			counter = 3
			w.Header().Add("Clear-Site-Data", "cache")
			refreshStoriesGeneral(ctx, name)
		}

		http.HandleFunc("GET /api/"+name, contextHandler(get, ctx))
		http.HandleFunc("POST /api/"+name, contextHandler(reload, ctx))

		go func() {
			for range ticker.C {
				if counter > 0 {
					counter--
					refreshStoriesGeneral(ctx, name)
				}
			}
		}()

	}
}
