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
	}

	go h.handleEvents()

	return h
}

func (h *paymentRouteHandler) handleEvents() {
	ctx := context.Background()

	channel := util.SetupSubChannel(ctx, h.broker, util.CHANNEL_PAYMENT)
	pubsub := h.broker.Subscribe(ctx, channel)

	// Wait for confirmation that subscription is created before publishing anything.
	_, err := pubsub.Receive(ctx)
	if err != nil {
		logrus.WithError(err).Panic("error listening to channel")
	}

	// Go channel which receives messages.
	var rm *redis.Message
	ch := pubsub.Channel()
	for rm = range ch {
		s := strings.Split(rm.Payload, "#")
		switch s[1] {
		case util.MESSAGE_PAY:
			go h.PayOrder(ctx, s[0], s[2])
		case util.MESSAGE_PAY_REVERT:
			go h.CancelOrder(ctx, s[2])
		}
	}
}

func (h *paymentRouteHandler) PayOrder(ctx context.Context, tracker string, order string) {
	userID := strings.Split(strings.Split(order, "\"user_id\": \"")[1], "\"")[0]
	orderID := strings.Split(strings.Split(order, "\"order_id\": \"")[1], "\"")[0]
	amount, _ := strconv.Atoi(strings.Split(strings.Split(order, "\"cost\": ")[1], "}")[0])

	err := h.paymentStore.Pay(ctx, userID, orderID, amount)
	if err != nil {
		if err == util.INTERNAL_ERR {
			util.Pub(h.broker, ctx, util.CHANNEL_ORDER, tracker, util.MESSAGE_ORDER_INTERNAL, "")
		} else {
			util.Pub(h.broker, ctx, util.CHANNEL_ORDER, tracker, util.MESSAGE_ORDER_BADREQUEST, "")
		}

		return
	}

	util.Pub(h.broker, ctx, util.CHANNEL_STOCK, tracker, util.MESSAGE_STOCK, order)
}

func (h *paymentRouteHandler) CancelOrder(ctx context.Context, order string) {
	userID := strings.Split(strings.Split(order, "\"user_id\": \"")[1], "\"")[0]
	orderID := strings.Split(strings.Split(order, "\"order_id\": \"")[1], "\"")[0]

	err := h.paymentStore.Cancel(ctx, userID, orderID)
	if err != nil {
		logrus.WithError(err).Info("unable to revert order payment")
	}
}

// Payment subtracts the amount of the order from the user’s credit
// func (h *paymentRouteHandler) PayOrder(ctx *fasthttp.RequestCtx) {
// 	userID := ctx.UserValue("user_id").(string)
// 	orderID := ctx.UserValue("order_id").(string)
// 	amount, err := strconv.Atoi(ctx.UserValue("amount").(string))
// 	if err != nil {
// 		ctx.SetStatusCode(fasthttp.StatusBadRequest)
// 		ctx.SetBodyString("amount should be an integer")
// 		return
// 	}

// 	h.paymentStore.Pay(ctx, userID, orderID, amount)
// }

// Cancel the payment made by a user
// func (h *paymentRouteHandler) CancelOrder(ctx *fasthttp.RequestCtx) {
// 	userID := ctx.UserValue("user_id").(string)
// 	orderID := ctx.UserValue("order_id").(string)

// 	h.paymentStore.Cancel(ctx, userID, orderID)
// }

// Return the status of a payment
func (h *paymentRouteHandler) GetPaymentStatus(ctx *fasthttp.RequestCtx) {
	orderID := ctx.UserValue("order_id").(string)

	h.paymentStore.PaymentStatus(ctx, orderID)
}
