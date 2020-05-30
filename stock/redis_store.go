package stock

import (
	"fmt"
	"strconv"

	"github.com/Jeffail/gabs"
	"github.com/go-redis/redis"
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
	json := fmt.Sprintf("{\"price\" : %d, \"number\": %d}", price, 0)

	set := s.store.SetNX(ID, json, 0)
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

	response := fmt.Sprintf("{\"id\" : \"%s\"}", ID)
	util.JSONResponse(ctx, fasthttp.StatusCreated, response)
}

func (s *redisStockStore) SubtractStock(ctx *fasthttp.RequestCtx, ID string, amount int) {
	get := s.store.Get(ID)
	if get.Err() == redis.Nil {
		util.NotFound(ctx)
		return
	} else if get.Err() != nil {
		logrus.WithError(get.Err()).Error("unable to find stock item")
		util.InternalServerError(ctx)
		return
	}

	str, _ := get.Bytes()
	jsonParsed, _ := gabs.ParseJSON(str)
	price := jsonParsed.S("price")
	number := jsonParsed.S("number")

	number_temp, _ := strconv.Atoi(number.String())
	number_temp = number_temp - amount

	if number_temp < 0 {
		util.BadRequest(ctx)
		return
	}

	json := fmt.Sprintf("{\"price\" : %s, \"number\": %s}", price, strconv.Itoa(number_temp))

	set := s.store.Set(ID, json, 0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to update stock item")
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)

}

func (s *redisStockStore) AddStock(ctx *fasthttp.RequestCtx, ID string, amount int) {
	get := s.store.Get(ID)
	if get.Err() == redis.Nil {
		util.NotFound(ctx)
		return
	} else if get.Err() != nil {
		logrus.WithError(get.Err()).Error("unable to find stock item")
		util.InternalServerError(ctx)
		return
	}

	str, _ := get.Bytes()
	jsonParsed, _ := gabs.ParseJSON(str)
	price := jsonParsed.S("price")
	number := jsonParsed.S("number")

	number_temp, _ := strconv.Atoi(number.String())
	number_temp = number_temp + amount

	json := fmt.Sprintf("{\"price\" : %s, \"number\": %s}", price, strconv.Itoa(number_temp))

	set := s.store.Set(ID, json, 0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to update stock item")
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}

func (s *redisStockStore) Find(ctx *fasthttp.RequestCtx, ID string) {
	get := s.store.Get(ID)
	if get.Err() == redis.Nil {
		util.NotFound(ctx)
		return
	} else if get.Err() != nil {
		logrus.WithError(get.Err()).Error("unable to find stock item")
		util.InternalServerError(ctx)
		return
	}
	str, _ := get.Bytes()
	jsonParsed, _ := gabs.ParseJSON(str)
	price := jsonParsed.S("price")
	amount := jsonParsed.S("number")

	response := fmt.Sprintf("{\"price\" : %s, \"number\": %s}", price, amount)

	util.JSONResponse(ctx, fasthttp.StatusOK, response)
}
