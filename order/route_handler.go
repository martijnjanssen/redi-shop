package order

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/gofrs/uuid"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type orderStore interface {
	Create(*fasthttp.RequestCtx, string)
	Remove(*fasthttp.RequestCtx, string)
	Find(*fasthttp.RequestCtx, string)
	AddItem(*fasthttp.RequestCtx, string, string)
	RemoveItem(*fasthttp.RequestCtx, string, string)

	GetOrder(*fasthttp.RequestCtx, string) (string, error)
}

var ErrNil = errors.New("value does not exist")

type orderRouteHandler struct {
	orderStore orderStore
	broker     *redis.Client
	urls       util.Services

	wgs   map[string]*sync.WaitGroup
	resps map[string]string
	lock  *sync.Mutex

	channelID string
}

func NewRouteHandler(conn *util.Connection) *orderRouteHandler {
	var store orderStore

	switch conn.Backend {
	case util.POSTGRES:
		store = newPostgresOrderStore(conn.Postgres, &conn.URL)
	case util.REDIS:
		store = newRedisOrderStore(conn.Redis, &conn.URL)
	}

	h := &orderRouteHandler{
		orderStore: store,
		broker:     conn.Broker,
		urls:       conn.URL,
		wgs:        map[string]*sync.WaitGroup{},
		resps:      map[string]string{},
		lock:       &sync.Mutex{},
		channelID:  uuid.Must(uuid.NewV4()).String(),
	}

	go h.handleEvents()

	return h
}

func (h *orderRouteHandler) handleEvents() {
	ctx := context.Background()

	pubsub := h.broker.PSubscribe(ctx, fmt.Sprintf("%s.%s", util.CHANNEL_ORDER, h.channelID))

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

		h.lock.Lock()
		h.resps[s[1]] = s[2]
		wg, ok := h.wgs[s[1]]
		h.lock.Unlock()

		if !ok {
			logrus.Error("could not get waitgroup")
			continue
		}
		wg.Done()
	}

	logrus.Fatal("SHOULD NEVER REACH THIS")
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

	// h.orderStore.Checkout(ctx, orderID)
	order, err := h.orderStore.GetOrder(ctx, orderID)
	if err == ErrNil {
		util.NotFound(ctx)
		return
	} else if err != nil {
		logrus.WithError(err).Error("unable to get order")
		util.InternalServerError(ctx)
		return
	}

	trackID := uuid.Must(uuid.NewV4()).String()
	wg := &sync.WaitGroup{}
	wg.Add(1)
	h.lock.Lock()
	h.wgs[trackID] = wg
	h.lock.Unlock()

	// Send message to issue order payment
	util.Pub(h.urls.Payment, "payment", h.channelID, trackID, util.MESSAGE_PAY, order)

	wg.Wait()

	h.lock.Lock()
	message, ok := h.resps[trackID]
	delete(h.resps, trackID)
	delete(h.wgs, trackID)
	h.lock.Unlock()

	if !ok {
		logrus.Error("could not get response from map")
		util.InternalServerError(ctx)
		return
	}

	switch message {
	case util.MESSAGE_ORDER_SUCCESS:
		util.Ok(ctx)
	case util.MESSAGE_ORDER_BADREQUEST:
		util.BadRequest(ctx)
	case util.MESSAGE_ORDER_INTERNAL:
		util.InternalServerError(ctx)
	default:
		logrus.WithField("message", message).Error("unknown message")
	}
}
