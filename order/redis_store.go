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
	urls *util.Services
}

func newRedisOrderStore(c *redis.Client) *redisOrderStore {
	return &redisOrderStore{
		store: c,
	}
}

func (s *redisOrderStore) Create(ctx *fasthttp.RequestCtx, userID string){
	orderID :=  uuid.Must(uuid.NewV4()).String()
	json := fmt.Sprintf("{\"user_id\": \"%s\", \"paid\": false, \"items\": \"[]\", \"cost\": 0}", userID)

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

	util.JSONResponse(ctx, fasthttp.StatusCreated, fmt.Sprintf("{\"order_id\": \"%s\"}", orderID))
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

	//get the order
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
	}
	if status != fasthttp.StatusOK {
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

	// Get the values of the order
	json := getOrder.Val()
	itemsPart := strings.Split(strings.Split(json, "\"items\": ")[1], ",")[0]

	// Add the item to the order
	items := itemStringToMap(itemsPart)
	items[itemID] = price
	itemString := mapToItemString(items)

	//update the price of the order
	costPart := strings.Split(strings.Split(json, "\"cost\": ")[1], "}")[0]
	cost, err := strconv.Atoi(costPart[0 : len(costPart)-1])
	if err != nil {
		logrus.WithError(err).Error("cannot parse order cost")
		util.InternalServerError(ctx)
		return
	}

	//update last part (item list and total cost)
	updateJsonPart := strings.Split(json, "\"items\": ")
	updateJsonPart[1] = fmt.Sprintf("\"items\": %s, \"cost\": %d}", itemString, cost+price)


	// Update the json object
	updatedJson := strings.Join(updateJsonPart, "")
	set := s.store.Set(ctx, orderID, updatedJson,0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to update order item")
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)

}

func (s *redisOrderStore) RemoveItem(ctx *fasthttp.RequestCtx, orderID string, itemID string){

	//get the order
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
	}
	if status != fasthttp.StatusOK {
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

	// Get the values of the order
	json := getOrder.Val()
	itemsPart := strings.Split(strings.Split(json, "\"items\": ")[1], ",")[0]


	// Convert string to map so we can update the map first
	items := itemStringToMap(itemsPart)

	// Update costs before you delete the item
	costPart := strings.Split(strings.Split(json, "\"cost\": ")[1], "}")[0]
	cost, err := strconv.Atoi(costPart[0 : len(costPart)-1])
	if err != nil {
		logrus.WithError(err).Error("cannot parse order cost")
		util.InternalServerError(ctx)
		return
	}

	// Now delete the item from the list
	delete(items, itemID)
	itemString := mapToItemString(items)

	//update last part (item list and total cost)
	updateJsonPart := strings.Split(json, "\"items\": ")
	updateJsonPart[1] = fmt.Sprintf("\"items\": %s, \"cost\": %d}", itemString, cost-price)

	// Update the json object
	updatedJson := strings.Join(updateJsonPart, ": ")
	set := s.store.Set(ctx, orderID, updatedJson,0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to update order item")
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}


func (s* redisOrderStore) Checkout(ctx *fasthttp.RequestCtx, orderID string){

	//Get the order
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
	json := getOrder.Val()

	// Parse the paid value (If paid, then simply return)
	paidPart := strings.Split(strings.Split(strings.Split(json, "\"items\": ")[0], "\"paid\": ")[1], ",")[0]
	if paidPart == "true" {
		return
	}

	// Parse the cost
	costPart := strings.Split(strings.Split(json, "\"cost\": ")[1], "}")[0]
	cost, err := strconv.Atoi(costPart[0 : len(costPart)-1])
	if err != nil {
		logrus.WithError(err).Error("cannot parse order cost")
		util.InternalServerError(ctx)
		return
	}


	userIDPart := strings.Split(strings.Split(strings.Split(json, "\"paid\": ")[0], ": ")[1], ",")[0]

	// Make the payment by calling payment service
	c := fasthttp.Client{}
	status, _, err := c.Post([]byte{}, fmt.Sprintf("%s/payment/pay/%s/%s/%d", s.urls.Payment, userIDPart, orderID, cost), nil)
	if err != nil {
		logrus.WithError(err).Error("unable to pay for the order")
		util.InternalServerError(ctx)
		return
	}
	if status != fasthttp.StatusOK {
		logrus.WithField("status", status).Error("error while paying for the order")
		ctx.SetStatusCode(status)
		return
	}

	// Update part of json object that is changed (set paid to true)
	itemsPart := strings.Split(strings.Split(json, "\"items\": ")[1], ",")[0]
	updateJsonPart := strings.Split(json, "\"paid\": ")
	updateJsonPart[1] = fmt.Sprintf("\"paid\": %t, \"items\": %s, \"cost\": %d}", true, itemsPart, cost)

	// Subtract stock for each item in the order
	items := itemStringToMap(itemsPart)
	for k := range items {
		status, _, err := c.Post([]byte{}, fmt.Sprintf("%s/stock/subtract/%s/1", s.urls.Stock, k), nil)
		if err != nil {
			logrus.WithError(err).Error("unable to subtract stock")
			util.InternalServerError(ctx)
			return
		}
		if status != fasthttp.StatusOK {
			logrus.WithField("status", status).Error("error while subtracting stock")
			ctx.SetStatusCode(status)
			return
		}
	}

	// Commit changes to json order object
	updatedJson := strings.Join(updateJsonPart, "")
	set := s.store.Set(ctx, orderID, updatedJson,0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to update order item")
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)


}
