package main

import (
	"embed"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

//go:embed client/dist/*
var app embed.FS

func validateClient() {
	_, err := app.ReadFile("client/dist/index.html")
	if err != nil {
		log.Panic().Err(err).Msg("Client application is not found")
	}
	log.Info().Msg("Client application loaded")
}

func SPAHandler(w http.ResponseWriter, r *http.Request) {
	// If request has dot (index.js, index.css) serve it, otherwise serve index.html
	if len(strings.Split(r.URL.Path, ".")) > 1 {
		w.Header().Add("Cache-Control", "max-age=86400")
		http.ServeFileFS(w, r, app, "client/dist"+r.URL.Path)
		return
	}
	w.Header().Add("Cache-Control", "max-age=6000")
	http.ServeFileFS(w, r, app, "client/dist/index.html")
}
