package order

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

type redisOrderStore struct {
	store *redis.Client
	urls  *util.Services
}

func newRedisOrderStore(c *redis.Client) *redisOrderStore {
	return &redisOrderStore{
		store: c,
	}
}

func (s *redisOrderStore) Create(ctx *fasthttp.RequestCtx, userID string) {
	orderID := uuid.Must(uuid.NewV4()).String()
	json := fmt.Sprintf("{\"user_id\": \"%s\", \"paid\": false, \"items\": [], \"cost\": 0}", userID)

	set := s.store.SetNX(ctx, orderID, json, 0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to create new order")
		util.InternalServerError(ctx)
		return
	}

	if !set.Val() {
		logrus.Error("order with this ID already exists")
		util.InternalServerError(ctx)
		return
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

	// Extract [...] part of the order, remove "->#" (cost mapping) from string and assemble string again
	itemsSplit := strings.Split(get.Val(), "items\": ")
	arraySplit := strings.Split(itemsSplit[1], ",")
	arraySplit[0] = itemStringToJSONString(arraySplit[0])
	itemsSplit[1] = strings.Join(arraySplit, ",")
	json := strings.Join(itemsSplit, "items\": ")

	util.JSONResponse(ctx, fasthttp.StatusOK, json)
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

	pricePart := strings.Split(string(resp), "\"price\": ")[1]
	price, err := strconv.Atoi(pricePart[:len(pricePart)-1])
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
	costPart := strings.Split(strings.Split(jsonSplit[1], "\"cost\": ")[1], "}")[0]
	cost, err := strconv.Atoi(costPart[0 : len(costPart)-1])
	if err != nil {
		logrus.WithError(err).Error("cannot parse order cost")
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
	costPart := strings.Split(strings.Split(jsonSplit[1], "\"cost\": ")[1], "}")[0]
	cost, err := strconv.Atoi(costPart[0 : len(costPart)-1])
	if err != nil {
		logrus.WithError(err).Error("cannot parse order cost")
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

func (s *redisOrderStore) Checkout(ctx *fasthttp.RequestCtx, orderID string) {
	getOrder := s.store.Get(ctx, orderID)
	if getOrder.Err() == redis.Nil {
		util.NotFound(ctx)
		return
	} else if getOrder.Err() != nil {
		logrus.WithError(getOrder.Err()).Error("unable to get order to add item")
		util.InternalServerError(ctx)
		return
	}
	// Get the values of the order
	order := stringToStruct(getOrder.Val())

	if order.Paid == "paid" {
		util.BadRequest(ctx)
		return
	}

	// Make the payment by calling payment service
	c := fasthttp.Client{}
	status, _, err := c.Post([]byte{}, fmt.Sprintf("%s/payment/pay/%s/%s/%s", s.urls.Payment, order.UserID, orderID, order.Cost), nil)
	if err != nil {
		logrus.WithError(err).Error("unable to pay for the order")
		util.InternalServerError(ctx)
		return
	} else if status != fasthttp.StatusOK {
		logrus.WithField("status", status).Error("error while paying for the order")
		ctx.SetStatusCode(status)
		return
	}

	// Subtract stock for each item in the order
	items := itemStringToMap(order.Items)
	for k := range items {
		status, _, err := c.Post([]byte{}, fmt.Sprintf("%s/stock/subtract/%s/1", s.urls.Stock, k), nil)
		if err != nil {
			logrus.WithError(err).Error("unable to subtract stock")
			util.InternalServerError(ctx)
			return
		} else if status != fasthttp.StatusOK {
			logrus.WithField("status", status).Error("error while subtracting stock")
			ctx.SetStatusCode(status)
			return
		}
	}

	json := fmt.Sprintf("{\"user_id\": \"%s\", \"paid\": true, \"items\": %s, \"cost\": %s}", order.UserID, order.Items, order.Cost)
	set := s.store.Set(ctx, orderID, json, 0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to update order item")
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}

type simpleOrder struct {
	UserID string
	Paid   string
	Items  string
	Cost   string
}

func stringToStruct(order string) *simpleOrder {
	// {\"user_id\": \"%s\", \"paid\": false, \"items\": [], \"cost\": 0}
	jsonSplit := strings.Split(order, ": ")

	// {\"user_id\":
	// \"%s\", \"paid\":
	// false, \"items\":
	// [], \"cost\":
	// 0}
	userID := strings.Split(jsonSplit[1], ",")[0]
	paid := strings.Split(jsonSplit[2], ",")[0]
	items := strings.Split(jsonSplit[3], ",")[0]
	cost := strings.Split(jsonSplit[4], "}")[0]

	return &simpleOrder{
		UserID: userID[1 : len(userID)-1],
		Paid:   paid,
		Items:  items,
		Cost:   cost,
	}
}
