package order

import (
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/valyala/fasthttp"
)

type orderStore interface {
	Create(*fasthttp.RequestCtx, string)
	Remove(*fasthttp.RequestCtx, string)
	Find(*fasthttp.RequestCtx, string)
	AddItem(*fasthttp.RequestCtx, string, string)
	RemoveItem(*fasthttp.RequestCtx, string, string)
	Checkout(*fasthttp.RequestCtx, string)
}

type orderRouteHandler struct {
	orderStore orderStore
}

func NewRouteHandler(conn *util.Connection) *orderRouteHandler {
	var store orderStore

	switch conn.Backend {
	case util.POSTGRES:
		store = newPostgresOrderStore(conn.Postgres)
	case util.REDIS:
		panic("NOT IMPLEMENTED")
	}

	return &orderRouteHandler{
		orderStore: store,
	}
}

// Creates order for given user, and returns an order ID
func (h *orderRouteHandler) CreateOrder(ctx *fasthttp.RequestCtx) {
	userID := ctx.UserValue("user_id").(string)
	h.orderStore.Create(ctx, userID)
}

// Deletes an order by ID
func (h *orderRouteHandler) RemoveOrder(ctx *fasthttp.RequestCtx) {
	orderID := ctx.UserValue("order_id").(string)
	h.orderStore.Remove(ctx, orderID)
}

// Retrieves information of an order
func (h *orderRouteHandler) FindOrder(ctx *fasthttp.RequestCtx) {
	orderID := ctx.UserValue("order_id").(string)
	h.orderStore.Find(ctx, orderID)
}

// Adds a g given item in the order given
func (h *orderRouteHandler) AddOrderItem(ctx *fasthttp.RequestCtx) {
	orderID := ctx.UserValue("order_id").(string)
	itemID := ctx.UserValue("item_id").(string)
	h.orderStore.AddItem(ctx, orderID, itemID)
}

// Removes the given item from the give order
func (h *orderRouteHandler) RemoveOrderItem(ctx *fasthttp.RequestCtx) {
	orderID := ctx.UserValue("order_id").(string)
	itemID := ctx.UserValue("item_id").(string)
	h.orderStore.RemoveItem(ctx, orderID, itemID)
}

// Make the payment, subtract the stock and return a status
func (h *orderRouteHandler) CheckoutOrder(ctx *fasthttp.RequestCtx) {
	orderID := ctx.UserValue("order_id").(string)
	h.orderStore.Checkout(ctx, orderID)
}
