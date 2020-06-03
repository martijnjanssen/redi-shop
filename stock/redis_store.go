package stock

import (
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
	ID := uuid.Must(uuid.NewV4()).String()
	json := fmt.Sprintf("{\"price\": %d, \"stock\": 0}", price)

	set := s.store.SetNX(ctx, ID, json, 0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to create stock item")
		util.InternalServerError(ctx)
		return
	}

	if !set.Val() {
		logrus.Error("stock item already exists")
		util.InternalServerError(ctx)
		return
	}

	response := fmt.Sprintf("{\"item_id\": \"%s\"}", ID)
	util.JSONResponse(ctx, fasthttp.StatusCreated, response)
}

func (s *redisStockStore) SubtractStock(ctx *fasthttp.RequestCtx, ID string, amount int) {
	get := s.store.Get(ctx, ID)
	if get.Err() == redis.Nil {
		util.NotFound(ctx)
		return
	} else if get.Err() != nil {
		logrus.WithError(get.Err()).Error("unable to find stock item")
		util.InternalServerError(ctx)
		return
	}

	json := get.Val()
	jsonSplit := strings.Split(json, ": ")
	stockString := jsonSplit[2]
	stock, err := strconv.Atoi(stockString[0 : len(stockString)-1])
	if err != nil {
		logrus.WithError(err).Error("cannot parse stock amount")
		util.InternalServerError(ctx)
		return
	}

	if stock-amount < 0 {
		util.BadRequest(ctx)
		return
	}

	jsonSplit[2] = fmt.Sprintf("%d}", stock-amount)
	updatedJson := strings.Join(jsonSplit, ": ")

	set := s.store.Set(ctx, ID, updatedJson, 0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to update stock item")
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)

}

func (s *redisStockStore) AddStock(ctx *fasthttp.RequestCtx, ID string, amount int) {
	get := s.store.Get(ctx, ID)
	if get.Err() == redis.Nil {
		util.NotFound(ctx)
		return
	} else if get.Err() != nil {
		logrus.WithError(get.Err()).Error("unable to find stock item")
		util.InternalServerError(ctx)
		return
	}

	json := get.Val()
	jsonSplit := strings.Split(json, ": ")
	stockString := jsonSplit[2]
	stock, err := strconv.Atoi(stockString[0 : len(stockString)-1])
	if err != nil {
		logrus.WithError(err).Error("cannot parse stock amount")
		util.InternalServerError(ctx)
		return
	}
	jsonSplit[2] = fmt.Sprintf("%d}", (stock + amount))
	updatedJson := strings.Join(jsonSplit, ": ")
	set := s.store.Set(ctx, ID, updatedJson, 0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to update stock item")
		util.InternalServerError(ctx)
		return
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
