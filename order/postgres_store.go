package order

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type postgresOrderStore struct {
	db *gorm.DB
}

func newPostgresOrderStore(db *gorm.DB) *postgresOrderStore {
	err := db.AutoMigrate(&Order{}).Error
	if err != nil {
		panic(err)
	}
	return &postgresOrderStore{
		db: db,
	}
}

func (s *postgresOrderStore) Create(ctx *fasthttp.RequestCtx, userID string) {
	order := &Order{
		UserID: userID,
		Items:  "[]",
	}
	err := s.db.Model(&Order{}).
		Create(order).
		Error
	if err != nil {
		util.InternalServerError(ctx)
		return
	}

	util.JSONResponse(ctx, fasthttp.StatusCreated, fmt.Sprintf("{\"order_id\": %s}", order.ID))
}

func (s *postgresOrderStore) Remove(ctx *fasthttp.RequestCtx, orderID string) {
	err := s.db.Model(&Order{}).
		Delete(&Order{ID: orderID}).
		Error
	if err != nil {
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}

func (s *postgresOrderStore) Find(ctx *fasthttp.RequestCtx, orderID string) {
	order := &Order{}
	err := s.db.Model(&Order{}).
		Where("id = ?", orderID).
		First(order).
		Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		return
	} else if err != nil {
		util.InternalServerError(ctx)
		return
	}

	items := itemStringToMap(order.Items)
	itemString := ""
	for k := range items {
		itemString = fmt.Sprintf("%s%s,", itemString, k)
	}

	response := fmt.Sprintf("{\"order_id\" : %s, \"paid\": %t, \"items\": [%s], \"user_id\": %s, \"total_cost\": %d}", order.ID, order.Paid, itemString[:len(itemString)-1], order.UserID, order.Cost)
	util.JSONResponse(ctx, fasthttp.StatusOK, response)
}

func (s *postgresOrderStore) AddItem(ctx *fasthttp.RequestCtx, orderID string, itemID string) {
	// Get the order from the database
	order := &Order{}
	err := s.db.Model(&Order{}).
		Where("id = ?", orderID).
		First(order).
		Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		return
	} else if err != nil {
		util.InternalServerError(ctx)
		return
	}

	// Get the price of the item
	c := fasthttp.Client{}
	status, resp, err := c.Post([]byte{}, fmt.Sprintf("http://localhost/stock/find/%s", itemID), nil)
	if err != nil {
		// TODO: Abort transaction here
		util.InternalServerError(ctx)
		return
	}
	if status != fasthttp.StatusOK {
		// TODO: Maybe relay the response?
		ctx.SetStatusCode(status)
		return
	}
	pricePart := strings.Split(string(resp), "price: ")[1]
	price, err := strconv.Atoi(pricePart[:len(pricePart)-1])
	if err != nil {
		logrus.WithError(err).WithField("stock", string(resp)).Error("malformed response from stock service")
		util.InternalServerError(ctx)
		return
	}

	// Add the item to the order and update the price of the order
	items := itemStringToMap(order.Items)
	items[itemID] = price
	order.Items = mapToItemString(items)
	order.Cost += price

	// Save the updated order in the database
	err = s.db.Model(&Order{}).
		Where("id = ?", orderID).
		Update("items", order.Items).
		Update("cost", order.Cost).
		Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		return
	} else if err != nil {
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}

func (s *postgresOrderStore) RemoveItem(ctx *fasthttp.RequestCtx, orderID string, itemID string) {
	order := &Order{}
	err := s.db.Model(&Order{}).
		Where("id = ?", orderID).
		First(order).
		Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		return
	} else if err != nil {
		util.InternalServerError(ctx)
		return
	}

	// Remove the item from the order and update the price of the order
	items := itemStringToMap(order.Items)
	order.Cost -= items[itemID]
	delete(items, itemID)
	order.Items = mapToItemString(items)

	err = s.db.Model(&Order{}).
		Where("id = ?", orderID).
		Update("items", order.Items).
		Update("cost", order.Cost).
		Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		return
	} else if err != nil {
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}

// NOTE: function is highly experimental, has to be changed/tweaked to handle transactions and other services better
func (s *postgresOrderStore) Checkout(ctx *fasthttp.RequestCtx, orderID string) {
	order := &Order{}
	err := s.db.Model(&Order{}).
		Where("id = ? AND NOT paid", orderID).
		First(order).
		Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		return
	} else if err != nil {
		util.InternalServerError(ctx)
		return
	}

	c := fasthttp.Client{}
	// Make the payment
	status, _, err := c.Post([]byte{}, fmt.Sprintf("http://localhost/payment/pay/%s/%s/%d", order.UserID, orderID, order.Cost), nil)
	if err != nil {
		// TODO: Abort transaction here
		util.InternalServerError(ctx)
		return
	}
	if status != fasthttp.StatusOK {
		// TODO: Maybe relay the response?
		ctx.SetStatusCode(status)
		return
	}

	// Set the order as paid in the database
	err = s.db.Model(&Order{}).
		Where("id = ? AND NOT paid", orderID).
		Update("paid", true).
		Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		return
	} else if err != nil {
		util.InternalServerError(ctx)
		return
	}

	// Subtract stock for each item in the order
	items := itemStringToMap(order.Items)
	for k := range items {
		status, _, err := c.Post([]byte{}, fmt.Sprintf("http://localhost/stock/subtract/%s/1", k), nil)
		if err != nil {
			// TODO: Abort transaction here

			util.InternalServerError(ctx)
			return
		}
		if status != fasthttp.StatusOK {
			// TODO: Maybe relay the response?
			ctx.SetStatusCode(status)
			return
		}
	}

	// TODO: Commit transaction
	util.Ok(ctx)
}

func itemStringToMap(itemString string) map[string]int {
	m := map[string]int{}
	var item []string

	items := strings.Split(itemString[1:len(itemString)-1], ",")
	for i := range items {
		item = strings.Split(items[i], "->")
		val, err := strconv.Atoi(item[1])
		if err != nil {
			panic(fmt.Sprintf("invalid string representation of item, %s", items[i]))
		}
		m[item[0]] = val
	}

	return m
}

func mapToItemString(items map[string]int) string {
	s := ""

	for k, v := range items {
		s = fmt.Sprintf("%s%s->%d,", s, k, v)
	}

	return fmt.Sprintf("[%s]", s[:len(s)-1])
}
