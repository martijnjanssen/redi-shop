package stock

import (
	"strconv"

	"github.com/martijnjanssen/redi-shop/util"
	"github.com/valyala/fasthttp"
)

type stockStore interface {
	Create(*fasthttp.RequestCtx, int)
	Find(*fasthttp.RequestCtx, string)
	AddStock(*fasthttp.RequestCtx, string, int)
	SubtractStock(*fasthttp.RequestCtx, string, int)
}
type stockRouteHandler struct {
	stockStore stockStore
}

func NewRouteHandler(conn *util.Connection) *stockRouteHandler {
	var store stockStore

	switch conn.Backend {
	case util.POSTGRES:
		store = newPostgresStockStore(conn.Postgres)
	case util.REDIS:
		store = newRedisStockStore(conn.Redis)
	}

	return &stockRouteHandler{
		stockStore: store,
	}

}

// Returns success/failure, depending on the price status.
// Returns an ID for the created stock item with the given price
func (h *stockRouteHandler) CreateStockItem(ctx *fasthttp.RequestCtx) {
	price, err := strconv.Atoi(ctx.UserValue("price").(string))
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetBodyString("price should be an integer")
		return
	}

	h.stockStore.Create(ctx, price)
}

// Returns a stock item with their details (stockNumber, price)
func (h *stockRouteHandler) FindStockItem(ctx *fasthttp.RequestCtx) {
	itemID := ctx.UserValue("item_id").(string)

	h.stockStore.Find(ctx, itemID)
}

// Returns success/failure, depending on the stockNumber status.
// Adds the amount to the stock of the item.
func (h *stockRouteHandler) AddStockNumber(ctx *fasthttp.RequestCtx) {
	itemID := ctx.UserValue("item_id").(string)
	number, err := strconv.Atoi(ctx.UserValue("number").(string))
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetBodyString("number should be an integer")
		return
	}

	h.stockStore.AddStock(ctx, itemID, number)
}

// Returns success/failure, depending on the stockNumber status.
// Subtracts the amount to the stock of the item.
func (h *stockRouteHandler) SubtractStockNumber(ctx *fasthttp.RequestCtx) {
	itemID := ctx.UserValue("item_id").(string)
	number, err := strconv.Atoi(ctx.UserValue("number").(string))
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetBodyString("number should be an integer")
		return
	}

	h.stockStore.SubtractStock(ctx, itemID, number)
}
