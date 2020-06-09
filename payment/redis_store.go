package payment

import (
	"fmt"
	//"strconv"
	"strings"
	//"encoding/json"
	"github.com/go-redis/redis/v8"
	//"github.com/gofrs/uuid"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type redisPaymentStore struct {
	store *redis.Client
	urls *util.Services
}

func newRedisPaymentStore(c *redis.Client) *redisPaymentStore {
	// AutoMigrate structs to create or update database tables
	return &redisPaymentStore{
		store: c,
	}
}

func (s *redisPaymentStore) Pay(ctx *fasthttp.RequestCtx, userID string, orderID string, amount int) {

	var json string
	
	//Call the user service to subtract the order amount from the users' credit

	c := fasthttp.Client{}
	status, _, err := c.Post([]byte{}, fmt.Sprintf("http://%s:8000/users/credit/subtract/%s/%d", s.urls.User, userID, amount), nil)

	if err != nil {
		logrus.WithError(err).Error("unable to subtract credit")
		util.InternalServerError(ctx)
		return
	} else if status != fasthttp.StatusOK {
		logrus.WithField("status", status).Error("error while subtracting credit")
		ctx.SetStatusCode(status)
		return
	} 
		json = fmt.Sprintf("{\"amount\": %d, \"status\": \"paid\"}", amount)

	//Set payment status to paid. SETNX command will set key to hold a string value if key does not exist. If key already exists, no operation is performed.

	set := s.store.SetNX(ctx, orderID, json, 0)
	if set.Err() != nil {
		logrus.WithError(set.Err()).Error("payment unsuccessful")
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

	json := get.Val() //get the amount and status in this format --> {amount: int, status: string}
	jsonSplit := strings.Split(json, ": ") //split the json above on the ":". so now jsonSplit is an array of 3 elements - "{amount" "int,status" "string}"

	//code for retrieving only the status of the payment from the json

	paymentString := jsonSplit[2] //this will access the 3rd element of the jsonSplit string which is "string }" [we don't need the "}"]
	payment_status := paymentString[0 : len(paymentString)-1]

	//code for retrieving only the amount of the payment from the json (used to refund credit to the user)
	
	amountString := jsonSplit[1] //this will access the 2nd element of the jsonSplit string which is "int, status" [we only need the int]
	amount := amountString[0 : len(amountString)-2] 

	if payment_status == "cancelled" {
		logrus.Info("payment is already cancelled")
		util.BadRequest(ctx)
		return
	}
	
	// Refund the credit to the user
	c := fasthttp.Client{}
	status, _, err := c.Post([]byte{}, fmt.Sprintf("http://%s:8000/users/credit/add/%s/%d", s.urls.User, userID, amount), nil)
	if err != nil {
		logrus.WithError(err).Error("unable to refund credit to user")
		util.InternalServerError(ctx)
		return
	}
	if status != fasthttp.StatusOK {
		logrus.WithField("status", status).Error("error while refunding credit to user")
		ctx.SetStatusCode(status)
		return
	}

	// Update the status of the payment to cancelled
	set := s.store.Set(ctx, orderID, payment_status, 0)
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

	util.JSONResponse(ctx, fasthttp.StatusOK, get.Val())
}