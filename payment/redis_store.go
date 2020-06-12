package payment

import (
	"context"
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

func (s *redisPaymentStore) Pay(ctx context.Context, userID string, orderID string, amount int) error {
	exists := true
	get := s.store.Get(ctx, orderID)
	if get.Err() == redis.Nil {
		exists = false
	} else if get.Err() != nil {
		logrus.WithError(get.Err()).Error("unable to retrieve payment")
		return util.INTERNAL_ERR
	}

	if exists && strings.Contains(get.Val(), "paid") {
		logrus.Info("order was already paid")
		return util.BAD_REQUEST
	}

	//Call the user service to subtract the order amount from the users' credit
	c := fasthttp.Client{}
	status, _, err := c.Post([]byte{}, fmt.Sprintf("%s/users/credit/subtract/%s/%d", s.urls.User, userID, amount), nil)
	if err != nil {
		logrus.WithError(err).Error("unable to subtract credit")
		return util.INTERNAL_ERR
	} else if status != fasthttp.StatusOK {
		return util.HTTPErrorToSAGAError(status)
	}

	set := s.store.Set(ctx, orderID, fmt.Sprintf("{\"amount\": %d, \"status\": \"paid\"}", amount), 0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to persist payment")
		return util.INTERNAL_ERR
	}

	return nil
}

func (s *redisPaymentStore) Cancel(ctx context.Context, userID string, orderID string) error {
	// Retrieve the payment which needs to be canceled
	get := s.store.Get(ctx, orderID)
	if get.Err() == redis.Nil {
		return util.BAD_REQUEST
	} else if get.Err() != nil {
		logrus.WithError(get.Err()).Error("unable to retrieve payment to cancel")
		return util.INTERNAL_ERR
	}

	// get the amount and status in this format --> {"amount": int, "status": "string"}
	json := get.Val()

	// code for retrieving only the status of the payment from the json (used to check if the payment has already been canceled)
	// Retrieve the string between "\"status\": \"" and "\"}"
	payment_status := strings.Split(strings.Split(json, "\"status\": \"")[1], "\"}")[0]

	// code for retrieving only the amount of the payment from the json (used to refund credit to the user)
	// Retrieve the string between "\"amount\": " and ","
	amount := strings.Split(strings.Split(json, "\"amount\": ")[1], ",")[0]

	if payment_status == "canceled" {
		logrus.Info("payment is already canceled")
		return util.BAD_REQUEST
	}

	// Refund the credit to the user
	c := fasthttp.Client{}
	status, _, err := c.Post([]byte{}, fmt.Sprintf("%s/users/credit/add/%s/%s", s.urls.User, userID, amount), nil)
	if err != nil {
		logrus.WithError(err).Error("unable to refund credit to user")
		return util.INTERNAL_ERR
	} else if status != fasthttp.StatusOK {
		logrus.WithField("status", status).Error("error while refunding credit to user")
		return util.HTTPErrorToSAGAError(status)
	}

	// Update the status of the payment to canceled
	set := s.store.Set(ctx, orderID, fmt.Sprintf("{\"amount\": %s, \"status\": \"canceled\"}", amount), 0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("unable to update payment status")
		return util.INTERNAL_ERR
	}

	return nil
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
