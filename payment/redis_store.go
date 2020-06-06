package payment

import (
	"fmt"

	"github.com/go-redis/redis/v8"
	"github.com/gofrs/uuid"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type redisPaymentStore struct {
	store *redis.Client
}

func newRedisPaymentStore(c *redis.Client) *redisPaymentStore {
	// AutoMigrate structs to create or update database tables
	return &redisPaymentStore{
		store: c,
	}
}

func (s *redisPaymentStore) Pay(ctx *fasthttp.RequestCtx, userID string, orderID string, amount int) {

}

func (s *redisPaymentStore) Cancel(ctx *fasthttp.RequestCtx, userID string, orderID string) {
	
}

func (s *redisPaymentStore) PaymentStatus(ctx *fasthttp.RequestCtx, orderID string) {

	get := s.store.Get(ctx, orderID)
	if get.Err() == redis.Nil {
		util.NotFound(ctx)
		return
	} else if get.Err() != nil {
		logrus.WithError(get.Err()).Error("order ID does not exist")
		util.InternalServerError(ctx)
		return
	}

	paid := "false"

	if payment.Status == "paid" {
		paid = "true"
	}
	
	util.JSONResponse(ctx, fasthttp.StatusOK, fmt.Sprintf("{\"paid\": %s}", paid))
}