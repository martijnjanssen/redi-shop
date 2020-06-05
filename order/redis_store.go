package order

import (
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/gofrs/uuid"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
	"strconv"
	"strings"
)

type redisOrderStore struct {
	store *redis.Client
}

func newOrderStore(c *redis.Client) *redisOrderStore {
	return &redisOrderStore{
		store: c,
	}
}

func (s *redisOrderStore) Create(ctx *fasthttp.RequestCtx, userID string){
	orderID :=  uuid.Must(uuid.NewV4()).String()
	json := fmt.Sprintf("{\"user_id\": \"%s\", \"items\": \"%s\"}", userID, "[]")

	set := s.store.SetNX(ctx, orderID, json, 0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to create new order")
		util.InternalServerError(ctx)
		return
	}

	if !set.Val(){
		logrus.Error("order with this ID already exists")
		util.InternalServerError(ctx)
		return
	}

	response := fmt.Sprintf("{\"order_id\": \"%s\"}", orderID)
	util.JSONResponse(ctx, fasthttp.StatusCreated, response)
}

func (s *redisOrderStore) Remove(ctx *fasthttp.RequestCtx, orderID string){
	del := s.store.Del(ctx, orderID)
	if del.Err() != nil {
		logrus.WithError(del.Err()).Error("unable to remove order")
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)

}

func (s *redisOrderStore) Find(ctx *fasthttp.RequestCtx, orderID string){
	get := s.store.Get(ctx, orderID)
	if get.Err() == redis.Nil {
		util.NotFound(ctx)
		return
	} else if get.Err() != nil {
		logrus.WithError(get.Err()).Error("unable to find order")
		util.InternalServerError(ctx)
		return
	}

	util.JSONResponse(ctx, fasthttp.StatusOK, get.Val())


}

func (s *redisOrderStore) AddItem(ctx *fasthttp.RequestCtx, orderID string, itemID string){
	get := s.store.Get(ctx, orderID)
	if get.Err() == redis.Nil {
		util.NotFound(ctx)
		return
	} else if get.Err() != nil {
		logrus.WithError(get.Err()).Error(" unable to get order to add item")
		util.InternalServerError(ctx)
		return
	}

	// Get the values
	json := get.Val()
	jsonSplit := strings.Split(json, ":")
	pricePart := jsonSplit[1] // todo: check if this is the correct part
	price, err := strconv.Atoi(pricePart[0 : len(pricePart)-1])
	if err != nil {
		logrus.WithError(err).Error("unable to parse price part")
		util.InternalServerError(ctx)
		return
	}





	// Add the item to the order and update total costs

	//
}

func (s *redisOrderStore) RemoveItem(ctx *fasthttp.RequestCtx, orderID string, itemID string){

}


