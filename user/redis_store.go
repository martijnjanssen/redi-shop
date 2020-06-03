package user

import (
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/gofrs/uuid"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

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
	userID := uuid.Must(uuid.NewV4()).String()

	set := s.store.SetNX(ctx, userID, 0, 0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to create user")
		util.InternalServerError(ctx)
		return
	}

	if !set.Val() {
		logrus.Error("user already exists")
		util.InternalServerError(ctx)
		return
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
	get := s.store.Get(ctx, userID)
	if get.Err() != nil {
		logrus.WithError(get.Err()).Error("unable to get credit")
		util.InternalServerError(ctx)
		return
	}

	credit, err := get.Int()
	if err != nil {
		logrus.WithError(err).Error("unable to convert credit")
		util.InternalServerError(ctx)
		return
	}

	if credit-amount < 0 {
		util.BadRequest(ctx)
		return
	}

	decr := s.store.DecrBy(ctx, userID, int64(amount))
	if decr.Err() != nil {
		logrus.WithError(decr.Err()).Error("unable to decrement credit")
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
