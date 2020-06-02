package payment

import (
	"strconv"

	"github.com/martijnjanssen/redi-shop/util"
	"github.com/valyala/fasthttp"
)

type paymentStore interface {
	Pay(*fasthttp.RequestCtx, string, string, int)
	Cancel(*fasthttp.RequestCtx, string, string)
	PaymentStatus(*fasthttp.RequestCtx, string)
}

type paymentRouteHandler struct {
	paymentStore paymentStore
}

func NewRouteHandler(conn *util.Connection) *paymentRouteHandler {
	var store paymentStore

	switch conn.Backend {
	case util.POSTGRES:
		store = newPostgresPaymentStore(conn.Postgres)
	case util.REDIS:
		panic("NOT IMPLEMENTED")
	}

	return &paymentRouteHandler{
		paymentStore: store,
	}
}

// Payment subtracts the amount of the order from the user’s credit
func (h *paymentRouteHandler) PayOrder(ctx *fasthttp.RequestCtx) {
	userID := ctx.UserValue("user_id").(string)
	orderID := ctx.UserValue("order_id").(string)
	amount, err := strconv.Atoi(ctx.UserValue("amount").(string))
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetBodyString("amount should be an integer")
		return
	}

	h.paymentStore.Pay(ctx, userID, orderID, amount)
}

// Cancel the payment made by a user
func (h *paymentRouteHandler) CancelOrder(ctx *fasthttp.RequestCtx) {
	userID := ctx.UserValue("user_id").(string)
	orderID := ctx.UserValue("order_id").(string)

	h.paymentStore.Cancel(ctx, userID, orderID)
}

// Return the status of a payment
func (h *paymentRouteHandler) GetPaymentStatus(ctx *fasthttp.RequestCtx) {
	orderID := ctx.UserValue("order_id").(string)

	h.paymentStore.PaymentStatus(ctx, orderID)
}
