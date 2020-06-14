package stock

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/gofrs/uuid"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type redisStockStore struct {
	store *redis.Client
}

func newRedisStockStore(c *redis.Client) *redisStockStore {
	// AutoMigrate structs to create or update database tables
	return &redisStockStore{
		store: c,
	}
}

func (s *redisStockStore) Create(ctx *fasthttp.RequestCtx, price int) {
	json := fmt.Sprintf("{\"price\": %d, \"stock\": 0}", price)

	var itemID string
	created := false
	for !created {
		itemID = uuid.Must(uuid.NewV4()).String()
		set := s.store.SetNX(ctx, itemID, json, 0)
		if set.Err() != nil {
			logrus.WithError(set.Err()).Error("unable to create new order")
			util.InternalServerError(ctx)
			return
		}

		created = set.Val()
	}

	util.JSONResponse(ctx, fasthttp.StatusCreated, fmt.Sprintf("{\"item_id\": \"%s\"}", itemID))
}

func (s *redisStockStore) SubtractStock(ctx *fasthttp.RequestCtx, itemID string, amount int) {
	err := s.subtract(ctx, itemID, amount)
	if err == util.INTERNAL_ERR {
		util.InternalServerError(ctx)
	} else if err == util.BAD_REQUEST {
		util.BadRequest(ctx)
	}

	util.Ok(ctx)
}

func (s *redisStockStore) AddStock(ctx *fasthttp.RequestCtx, itemID string, amount int) {
	err := s.add(ctx, itemID, amount)
	if err == util.INTERNAL_ERR {
		util.InternalServerError(ctx)
	} else if err == util.BAD_REQUEST {
		util.BadRequest(ctx)
	}

	util.Ok(ctx)
}

func (s *redisStockStore) Find(ctx *fasthttp.RequestCtx, ID string) {
	get := s.store.Get(ctx, ID)
	if get.Err() == redis.Nil {
		util.NotFound(ctx)
		return
	} else if get.Err() != nil {
		logrus.WithError(get.Err()).Error("unable to find stock item")
		util.InternalServerError(ctx)
		return
	}

	util.JSONResponse(ctx, fasthttp.StatusOK, get.Val())
}

func (s *redisStockStore) subtract(ctx context.Context, ID string, amount int) error {
	get := s.store.Get(ctx, ID)
	if get.Err() == redis.Nil {
		return util.BAD_REQUEST
	} else if get.Err() != nil {
		logrus.WithError(get.Err()).Error("unable to find stock item")
		return util.INTERNAL_ERR
	}

	json := get.Val()
	jsonSplit := strings.Split(json, ": ")
	stock, err := strconv.Atoi(jsonSplit[2][0 : len(jsonSplit[2])-1])
	if err != nil {
		logrus.WithError(err).Error("cannot parse stock amount")
		return util.INTERNAL_ERR
	}

	if stock-amount < 0 {
		return util.BAD_REQUEST
	}

	jsonSplit[2] = fmt.Sprintf("%d}", stock-amount)
	updatedJson := strings.Join(jsonSplit, ": ")

	set := s.store.Set(ctx, ID, updatedJson, 0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to update stock item")
		return util.INTERNAL_ERR
	}

	return nil
}

func (s *redisStockStore) add(ctx context.Context, ID string, amount int) error {
	get := s.store.Get(ctx, ID)
	if get.Err() == redis.Nil {
		return util.BAD_REQUEST
	} else if get.Err() != nil {
		logrus.WithError(get.Err()).Error("unable to find stock item")
		return util.INTERNAL_ERR
	}

	json := get.Val()
	jsonSplit := strings.Split(json, ": ")
	stockString := jsonSplit[2]
	stock, err := strconv.Atoi(stockString[0 : len(stockString)-1])
	if err != nil {
		logrus.WithError(err).Error("cannot parse stock amount")
		return util.INTERNAL_ERR
	}
	jsonSplit[2] = fmt.Sprintf("%d}", (stock + amount))
	updatedJson := strings.Join(jsonSplit, ": ")
	set := s.store.Set(ctx, ID, updatedJson, 0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to update stock item")
		return util.INTERNAL_ERR
	}

	return nil
}
