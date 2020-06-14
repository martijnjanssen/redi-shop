package payment

import (
	"context"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type paymentStore interface {
	Pay(context.Context, string, string, int) error
	Cancel(context.Context, string, string) error
	PaymentStatus(*fasthttp.RequestCtx, string)
}

type paymentRouteHandler struct {
	paymentStore paymentStore
	broker       *redis.Client
	urls         util.Services
}

func NewRouteHandler(conn *util.Connection) *paymentRouteHandler {
	var store paymentStore

	switch conn.Backend {
	case util.POSTGRES:
		store = newPostgresPaymentStore(conn.Postgres, &conn.URL)
	case util.REDIS:
		store = newRedisPaymentStore(conn.Redis, &conn.URL)
	}

	h := &paymentRouteHandler{
		paymentStore: store,
		broker:       conn.Broker,
		urls:         conn.URL,
	}

	return h
}

func (h *paymentRouteHandler) HandleMessage(ctx *fasthttp.RequestCtx) {
	message := string(ctx.Request.Body())

	s := strings.Split(message, "#")
	switch s[2] {
	case util.MESSAGE_PAY:
		h.PayOrder(ctx, s[0], s[1], s[3])
	case util.MESSAGE_PAY_REVERT:
		h.CancelOrder(ctx, s[3])
	}

	util.Ok(ctx)
}

func (h *paymentRouteHandler) PayOrder(ctx context.Context, orderChannelID string, tracker string, order string) {
	userID := strings.Split(strings.Split(order, "\"user_id\": \"")[1], "\"")[0]
	orderID := strings.Split(strings.Split(order, "\"order_id\": \"")[1], "\"")[0]
	amount, _ := strconv.Atoi(strings.Split(strings.Split(order, "\"cost\": ")[1], "}")[0])

	err := h.paymentStore.Pay(ctx, userID, orderID, amount)
	if err != nil {
		if err == util.INTERNAL_ERR {
			util.PubToOrder(h.broker, ctx, orderChannelID, tracker, util.MESSAGE_ORDER_INTERNAL)
		} else {
			util.PubToOrder(h.broker, ctx, orderChannelID, tracker, util.MESSAGE_ORDER_BADREQUEST)
		}

		return
	}

	util.Pub(h.urls.Stock, "stock", orderChannelID, tracker, util.MESSAGE_STOCK, order)
}

func (h *paymentRouteHandler) CancelOrder(ctx context.Context, order string) {
	userID := strings.Split(strings.Split(order, "\"user_id\": \"")[1], "\"")[0]
	orderID := strings.Split(strings.Split(order, "\"order_id\": \"")[1], "\"")[0]

	err := h.paymentStore.Cancel(ctx, userID, orderID)
	if err != nil {
		logrus.WithError(err).Info("unable to revert order payment")
	}
}

// Return the status of a payment
func (h *paymentRouteHandler) GetPaymentStatus(ctx *fasthttp.RequestCtx) {
	orderID := ctx.UserValue("order_id").(string)

	h.paymentStore.PaymentStatus(ctx, orderID)
}
