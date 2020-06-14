package order

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/gofrs/uuid"
	"github.com/martijnjanssen/redi-shop/util"
	errwrap "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type redisOrderStore struct {
	store *redis.Client
	urls  *util.Services
}

func newRedisOrderStore(c *redis.Client, urls *util.Services) *redisOrderStore {
	return &redisOrderStore{
		store: c,
		urls:  urls,
	}
}

func (s *redisOrderStore) Create(ctx *fasthttp.RequestCtx, userID string) {
	json := fmt.Sprintf("{\"user_id\": \"%s\", \"items\": [], \"cost\": 0}", userID)

	var orderID string
	created := false
	for !created {
		orderID = uuid.Must(uuid.NewV4()).String()
		set := s.store.SetNX(ctx, orderID, json, 0)
		if set.Err() != nil {
			logrus.WithError(set.Err()).Error("unable to create new order")
			util.InternalServerError(ctx)
			return
		}

		created = set.Val()
	}

	util.JSONResponse(ctx, fasthttp.StatusCreated, fmt.Sprintf("{\"order_id\": \"%s\"}", orderID))
}

func (s *redisOrderStore) Remove(ctx *fasthttp.RequestCtx, orderID string) {
	del := s.store.Del(ctx, orderID)
	if del.Err() != nil {
		logrus.WithError(del.Err()).Error("unable to remove order")
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}

func (s *redisOrderStore) Find(ctx *fasthttp.RequestCtx, orderID string) {
	get := s.store.Get(ctx, orderID)
	if get.Err() == redis.Nil {
		util.NotFound(ctx)
		return
	} else if get.Err() != nil {
		logrus.WithError(get.Err()).Error("unable to find order")
		util.InternalServerError(ctx)
		return
	}

	c := fasthttp.Client{}
	status, statusResp, err := c.Post([]byte{}, fmt.Sprintf("%s/payment/status/%s/", s.urls.Payment, orderID), nil)
	if err != nil {
		logrus.WithError(err).Error("unable to get payment status")
		util.InternalServerError(ctx)
		return
	} else if status != fasthttp.StatusOK {
		logrus.WithField("status", status).Error("error while getting payment status")
		ctx.SetStatusCode(status)
		return
	}

	// Extract [...] part of the order, remove "->#" (cost mapping) from string and assemble string again
	itemsSplit := strings.Split(get.Val(), "items\": ")
	arraySplit := strings.Split(itemsSplit[1], ", ")
	arraySplit[0] = itemStringToJSONString(arraySplit[0])
	itemsSplit[1] = strings.Join(arraySplit, ", ")
	json := strings.Join(itemsSplit, "items\": ")

	util.JSONResponse(ctx, fasthttp.StatusOK, fmt.Sprintf("%s, \"paid\": %t}", json[:len(json)-1], strings.Contains(string(statusResp), "true")))
}

func (s *redisOrderStore) AddItem(ctx *fasthttp.RequestCtx, orderID string, itemID string) {
	getOrder := s.store.Get(ctx, orderID)
	if getOrder.Err() == redis.Nil {
		util.NotFound(ctx)
		return
	} else if getOrder.Err() != nil {
		logrus.WithError(getOrder.Err()).Error("unable to get order to add item")
		util.InternalServerError(ctx)
		return
	}

	// Get price of the item
	c := fasthttp.Client{}
	status, resp, err := c.Get([]byte{}, fmt.Sprintf("%s/stock/find/%s", s.urls.Stock, itemID))
	if err != nil {
		logrus.WithError(err).Error("unable to get item price")
		util.InternalServerError(ctx)
		return
	} else if status != fasthttp.StatusOK {
		logrus.WithField("status", status).Error("error while getting item price")
		ctx.SetStatusCode(status)
		return
	}

	pricePart := strings.Split(strings.Split(string(resp), "\"price\": ")[1], ",")[0]
	price, err := strconv.Atoi(pricePart)
	if err != nil {
		logrus.WithError(err).WithField("stock", string(resp)).Error("malformed response from stock service")
		util.InternalServerError(ctx)
		return
	}

	// Get the items of the order
	json := getOrder.Val()
	jsonSplit := strings.Split(json, "\"items\": ")

	// Add the item to the order
	items := itemStringToMap(strings.Split(jsonSplit[1], ",")[0])
	items[itemID] = price
	itemsString := mapToItemString(items)

	//update the price of the order
	costPart := strings.Split(jsonSplit[1], "\"cost\": ")[1]
	cost, err := strconv.Atoi(costPart[0 : len(costPart)-1])
	if err != nil {
		logrus.WithField("cost", costPart).WithError(err).Error("cannot parse order cost")
		util.InternalServerError(ctx)
		return
	}

	// Update item list and total cost
	jsonSplit[1] = fmt.Sprintf("%s, \"cost\": %d}", itemsString, cost+price)
	updatedJson := strings.Join(jsonSplit, "\"items\": ")

	set := s.store.Set(ctx, orderID, updatedJson, 0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to update order item")
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}

func (s *redisOrderStore) RemoveItem(ctx *fasthttp.RequestCtx, orderID string, itemID string) {
	getOrder := s.store.Get(ctx, orderID)
	if getOrder.Err() == redis.Nil {
		util.NotFound(ctx)
		return
	} else if getOrder.Err() != nil {
		logrus.WithError(getOrder.Err()).Error("unable to get order to add item")
		util.InternalServerError(ctx)
		return
	}

	// Get the items of the order
	json := getOrder.Val()
	jsonSplit := strings.Split(json, "\"items\": ")

	// Get price of the order
	costPart := strings.Split(jsonSplit[1], "\"cost\": ")[1]
	cost, err := strconv.Atoi(costPart[0 : len(costPart)-1])
	if err != nil {
		logrus.WithField("cost", costPart).WithError(err).Error("cannot parse order cost")
		util.InternalServerError(ctx)
		return
	}

	// Get the price of the item to remove and remove the item
	items := itemStringToMap(strings.Split(jsonSplit[1], ",")[0])
	price := items[itemID]
	delete(items, itemID)
	itemsString := mapToItemString(items)

	// Update item list and total cost
	jsonSplit[1] = fmt.Sprintf("%s, \"cost\": %d}", itemsString, cost-price)
	updatedJson := strings.Join(jsonSplit, "\"items\": ")

	set := s.store.Set(ctx, orderID, updatedJson, 0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to update order item")
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}

func (s *redisOrderStore) GetOrder(ctx *fasthttp.RequestCtx, orderID string) (string, error) {
	get := s.store.Get(ctx, orderID)
	if get.Err() == redis.Nil {
		return "", ErrNil
	} else if get.Err() != nil {
		return "", errwrap.Wrap(get.Err(), "unable to get order to add item")
	}

	// Extract [...] part of the order, remove "->#" (cost mapping) from string and assemble string again
	itemsSplit := strings.Split(get.Val(), "items\": ")
	arraySplit := strings.Split(itemsSplit[1], ", ")
	arraySplit[0] = itemStringToJSONString(arraySplit[0])
	itemsSplit[1] = strings.Join(arraySplit, ", ")
	json := strings.Join(itemsSplit, "items\": ")

	return fmt.Sprintf("{\"order_id\": \"%s\", %s", orderID, json[1:]), nil
}
