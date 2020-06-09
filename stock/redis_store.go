package stock

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gofrs/uuid"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type redisStockStore struct {
	store *redis.Client
	urls  *util.Services
}

func newRedisStockStore(c *redis.Client, urls *util.Services) *redisStockStore {
	// AutoMigrate structs to create or update database tables
	return &redisStockStore{
		store: c,
		urls:  urls,
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
	//Subscribe to channel "user_stock_channel"
	subscriber := s.store.Subscribe(ctx, "user_stock_channel")

	// Wait for confirmation that subscription is created before publishing anything.
	_, errSub := subscriber.Receive(ctx)
	if errSub != nil {
		logrus.WithError(errSub).Error("Subscribing of channel failed")
	}

	//Channel which receives messages
	channel := subscriber.Channel()

	//Close the channel after 2 seconds
	var duration int = 2
	time.AfterFunc(time.Duration(duration)*time.Second, func() {
		// When pubsub is closed channel is closed too.
		fmt.Printf("Closing subscriber after %v seconds\n", duration)
		_ = subscriber.Close()
	})

	//Make a call to user service to create user
	c := fasthttp.Client{}
	status, resp, err := c.Post([]byte{}, fmt.Sprintf("http://%s:8000/users/create/", s.urls.User), nil)
	if status != fasthttp.StatusCreated || err != nil {
		logrus.WithError(err).Error("Error from user service during user creation")
		util.InternalServerError(ctx)
		return
	}
	fmt.Println("Response from user service create call: " + string(resp))

	//Consume a message
	for msg := range channel {
		fmt.Println(msg.Channel, msg.Payload)
	}

	//Proceed once the channel has been closed and message has been received
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
