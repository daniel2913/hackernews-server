package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"sync"

	"github.com/rs/zerolog/log"
)

func populateComments(ctx context.Context, ids []int) ([]Comment, error) {
	wg := sync.WaitGroup{}
	kids := make([]Comment, len(ids))

	for idx, comment := range ids {
		wg.Add(1)

		go func() {
			defer wg.Done()
			dataI, err := fetchItem(comment, ctx)
			if err != nil {
				return
			}
			res := Comment{}
			err = json.Unmarshal(dataI, &res)
			if err != nil {
				return
			}
			go populateComments(ctx, res.Kids)
			kids[idx] = res
		}()
	}
	wg.Wait()
	sort.Slice(kids, func(i, j int) bool {
		return kids[i].Time > kids[j].Time
	})
	return kids, nil
}

type withKids struct {
	Kids []int `json:"kids"`
}

func getComments(id int, ctx context.Context) ([]byte, error) {
	reqctx := mustGetReqCtx(ctx)
	itemBytes, err := fetchItem(int(id), ctx)
	if err != nil {
		return nil, err
	}
	item := withKids{}
	err = json.Unmarshal(itemBytes, &item)
	if err != nil {
		log.Error().Err(err).Caller()
		reqctx.status = http.StatusInternalServerError
		return nil, err
	}
	comments, err := populateComments(ctx, item.Kids)
	if err != nil {
		log.Error().Err(err).Caller()
		reqctx.status = http.StatusInternalServerError
		return nil, err
	}
	commentsBytes, err := json.Marshal(comments)
	if err != nil {
		log.Error().Err(err).Caller()
		reqctx.status = http.StatusInternalServerError
		return nil, err
	}
	return commentsBytes, nil
}
