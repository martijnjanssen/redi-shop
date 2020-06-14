package user

import (
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/gofrs/uuid"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

// Script with condition checking
var decrByXX = redis.NewScript(`
		if tonumber(redis.call("GET", KEYS[1])) - ARGV[1] > -1 then
      return redis.call("DECRBY", KEYS[1], ARGV[1])
    end
    return false
	`)

type redisUserStore struct {
	store *redis.Client
}

func newRedisUserStore(c *redis.Client) *redisUserStore {
	// AutoMigrate structs to create or update database tables
	return &redisUserStore{
		store: c,
	}
}

func (s *redisUserStore) Create(ctx *fasthttp.RequestCtx) {
	var userID string
	created := false
	for !created {
		userID = uuid.Must(uuid.NewV4()).String()
		set := s.store.SetNX(ctx, userID, 0, 0)
		if set.Err() != nil {
			logrus.WithError(set.Err()).Error("unable to create new order")
			util.InternalServerError(ctx)
			return
		}

		created = set.Val()
	}

	util.JSONResponse(ctx, fasthttp.StatusCreated, fmt.Sprintf("{\"user_id\": \"%s\"}", userID))
}

func (s *redisUserStore) Remove(ctx *fasthttp.RequestCtx, userID string) {
	del := s.store.Del(ctx, userID)
	if del.Err() != nil {
		logrus.WithError(del.Err()).Error("unable to remove user")
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}

func (s *redisUserStore) Find(ctx *fasthttp.RequestCtx, userID string) {
	get := s.store.Get(ctx, userID)
	if get.Err() == redis.Nil {
		util.NotFound(ctx)
		return
	} else if get.Err() != nil {
		logrus.WithError(get.Err()).Error("unable to find user")
		util.InternalServerError(ctx)
		return
	}

	util.JSONResponse(ctx, fasthttp.StatusOK, fmt.Sprintf("{\"user_id\": \"%s\", \"credit\": %s}", userID, get.Val()))
}

func (s *redisUserStore) SubtractCredit(ctx *fasthttp.RequestCtx, userID string, amount int) {
	res := decrByXX.Run(ctx, s.store, []string{userID}, amount)
	if res.Err() == redis.Nil {
		util.NotFound(ctx)
		return
	} else if res.Err() != nil {
		logrus.WithError(res.Err()).Error("unable to subtract credit")
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}

func (s *redisUserStore) AddCredit(ctx *fasthttp.RequestCtx, userID string, amount int) {
	incr := s.store.IncrBy(ctx, userID, int64(amount))
	if incr.Err() != nil {
		logrus.WithError(incr.Err()).Error("unable to add credit")
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}
