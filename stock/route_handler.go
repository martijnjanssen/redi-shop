package stock

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-redis/redis/v8"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type stockStore interface {
	Create(*fasthttp.RequestCtx, int)
	Find(*fasthttp.RequestCtx, string)
	AddStock(*fasthttp.RequestCtx, string, int)
	SubtractStock(*fasthttp.RequestCtx, string, int)

	add(context.Context, string, int) error
	subtract(context.Context, string, int) error
}

type stockRouteHandler struct {
	stockStore stockStore
	broker     *redis.Client
	urls       *util.Services
}

func NewRouteHandler(conn *util.Connection) *stockRouteHandler {
	var store stockStore

	switch conn.Backend {
	case util.POSTGRES:
		store = newPostgresStockStore(conn.Postgres, &conn.URL)
	case util.REDIS:
		store = newRedisStockStore(conn.Redis)
	}

	h := &stockRouteHandler{
		stockStore: store,
		broker:     conn.Broker,
		urls:       &conn.URL,
	}

	return h
}

func (h *stockRouteHandler) HandleMessage(ctx *fasthttp.RequestCtx) {
	message := string(ctx.PostBody())

	s := strings.Split(message, "#")
	switch s[2] {
	case util.MESSAGE_STOCK:
		h.SubtractStockItems(ctx, s[0], s[1], s[3])
	}

	util.Ok(ctx)
}

func (h *stockRouteHandler) SubtractStockItems(ctx context.Context, orderChannelID string, tracker string, order string) {
	items := strings.Split(strings.Split(order, "\"items\": [")[1], "]")[0]

	if items == "" {
		h.broker.Publish(ctx, fmt.Sprintf("%s.%s", util.CHANNEL_ORDER, orderChannelID), fmt.Sprintf("%s#%s#%s", tracker, util.MESSAGE_ORDER_SUCCESS, ""))
		return
	}

	var err error
	done := []string{}
	for _, item := range strings.Split(items[1:len(items)-1], "\",\"") {
		err = h.stockStore.subtract(ctx, item, 1)
		if err != nil {
			break
		}
		done = append(done, item)
	}

	if err != nil {
		for _, i := range done {
			err = h.stockStore.add(ctx, i, 1)
			if err != nil {
				logrus.WithField("item_id", i).WithError(err).Error("UNABLE TO REVERT STOCK SUBTRACTION")
			}
		}

		util.Pub(h.urls.Payment, "payment", orderChannelID, tracker, util.MESSAGE_PAY_REVERT, order)
		if err == util.BAD_REQUEST {
			util.PubToOrder(h.broker, ctx, orderChannelID, tracker, util.MESSAGE_ORDER_BADREQUEST)
		} else {
			util.PubToOrder(h.broker, ctx, orderChannelID, tracker, util.MESSAGE_ORDER_INTERNAL)
		}

		return
	}

	util.PubToOrder(h.broker, ctx, orderChannelID, tracker, util.MESSAGE_ORDER_SUCCESS)
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
