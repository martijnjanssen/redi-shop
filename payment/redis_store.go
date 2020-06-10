package payment

import (
	"fmt"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type redisPaymentStore struct {
	store *redis.Client
	urls  *util.Services
}

func newRedisPaymentStore(c *redis.Client, urls *util.Services) *redisPaymentStore {
	// AutoMigrate structs to create or update database tables
	return &redisPaymentStore{
		store: c,
		urls:  urls,
	}
}

func (s *redisPaymentStore) Pay(ctx *fasthttp.RequestCtx, userID string, orderID string, amount int) {
	exists := true
	get := s.store.Get(ctx, orderID)
	if get.Err() == redis.Nil {
		exists = false
	} else if get.Err() != nil {
		logrus.WithError(get.Err()).Error("unable to retrieve payment")
		util.InternalServerError(ctx)
		return
	}

	if exists && strings.Contains(get.Val(), "paid") {
		logrus.Info("order was already paid")
		util.BadRequest(ctx)
		return
	}

	//Call the user service to subtract the order amount from the users' credit
	c := fasthttp.Client{}
	status, _, err := c.Post([]byte{}, fmt.Sprintf("%s/users/credit/subtract/%s/%d", s.urls.User, userID, amount), nil)
	if err != nil {
		logrus.WithError(err).Error("unable to subtract credit")
		util.InternalServerError(ctx)
		return
	} else if status != fasthttp.StatusOK {
		logrus.WithField("status", status).Error("error while subtracting credit")
		ctx.SetStatusCode(status)
		return
	}

	//Set payment status to paid. SETNX command will set key to hold a string value if key does not exist. If key already exists, no operation is performed.
	set := s.store.SetNX(ctx, orderID, fmt.Sprintf("{\"amount\": %d, \"status\": \"paid\"}", amount), 0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to persist payment")
		util.InternalServerError(ctx)
		return
	}
	util.Ok(ctx)
}

func (s *redisPaymentStore) Cancel(ctx *fasthttp.RequestCtx, userID string, orderID string) {
	// Retrieve the payment which needs to be cancelled
	get := s.store.Get(ctx, orderID)
	if get.Err() == redis.Nil {
		util.NotFound(ctx)
		return
	} else if get.Err() != nil {
		logrus.WithError(get.Err()).Error("unable to retrieve payment to cancel")
		util.InternalServerError(ctx)
		return
	}

	// get the amount and status in this format --> {"amount": int, "status": "string"}
	json := get.Val()

	// code for retrieving only the status of the payment from the json (used to check if the payment has already been cancelled)
	// Retrieve the string between "\"status\": \"" and "\"}"
	payment_status := strings.Split(strings.Split(json, "\"status\": \"")[1], "\"}")[0]

	// code for retrieving only the amount of the payment from the json (used to refund credit to the user)
	// Retrieve the string between "\"amount\": " and ","
	amount := strings.Split(strings.Split(json, "\"amount\": ")[1], ",")[0]

	if payment_status == "cancelled" {
		logrus.Info("payment is already cancelled")
		util.BadRequest(ctx)
		return
	}

	// Refund the credit to the user
	c := fasthttp.Client{}
	status, _, err := c.Post([]byte{}, fmt.Sprintf("%s/users/credit/add/%s/%s", s.urls.User, userID, amount), nil)
	if err != nil {
		logrus.WithError(err).Error("unable to refund credit to user")
		util.InternalServerError(ctx)
		return
	} else if status != fasthttp.StatusOK {
		logrus.WithField("status", status).Error("error while refunding credit to user")
		ctx.SetStatusCode(status)
		return
	}

	// Update the status of the payment to cancelled
	set := s.store.Set(ctx, orderID, fmt.Sprintf("{\"amount\": %s, \"status\": \"cancelled\"}", amount), 0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to update payment status")
		util.InternalServerError(ctx)
		return
	}
	util.Ok(ctx)
}

func (s *redisPaymentStore) PaymentStatus(ctx *fasthttp.RequestCtx, orderID string) {
	get := s.store.Get(ctx, orderID)
	if get.Err() == redis.Nil {
		util.NotFound(ctx)
		return
	} else if get.Err() != nil {
		logrus.WithError(get.Err()).Error("unable to retrieve payment")
		util.InternalServerError(ctx)
		return
	}

	paid := "false"
	if strings.Contains(get.Val(), "paid") {
		paid = "true"
	}

	util.JSONResponse(ctx, fasthttp.StatusOK, fmt.Sprintf("{\"paid\": %s}", paid))
}
