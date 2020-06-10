package order

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/martijnjanssen/redi-shop/util"
	errwrap "github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type postgresOrderStore struct {
	db   *gorm.DB
	urls *util.Services
}

func newPostgresOrderStore(db *gorm.DB, urls *util.Services) *postgresOrderStore {
	err := db.AutoMigrate(&Order{}).Error
	if err != nil {
		panic(err)
	}
	return &postgresOrderStore{
		db:   db,
		urls: urls,
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
		logrus.WithError(err).Error("unable to create new order")
		util.InternalServerError(ctx)
		return
	}

	util.JSONResponse(ctx, fasthttp.StatusCreated, fmt.Sprintf("{\"order_id\": \"%s\"}", order.ID))
}

func (s *postgresOrderStore) Remove(ctx *fasthttp.RequestCtx, orderID string) {
	err := s.db.Model(&Order{}).
		Delete(&Order{ID: orderID}).
		Error
	if err != nil {
		logrus.WithError(err).Error("unable to remove order")
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
		logrus.WithError(err).Error("unable to find order")
		util.InternalServerError(ctx)
		return
	}

	c := fasthttp.Client{}
	status, statusResp, err := c.Post([]byte{}, fmt.Sprintf("%s/payment/status/%s/", s.urls.Payment, orderID), nil)
	if err != nil {
		logrus.WithError(err).Error("unable to get payment status")
		util.InternalServerError(ctx)
		return
	} else if status != fasthttp.StatusOK {
		logrus.WithField("status", status).Error("error while getting payment status")
		ctx.SetStatusCode(status)
		return
	}

	itemsString := itemStringToJSONString(order.Items)
	response := fmt.Sprintf("{\"order_id\": \"%s\", \"paid\": %t, \"items\": [%s], \"user_id\": \"%s\", \"total_cost\": %d}", order.ID, strings.Contains(string(statusResp), "true"), itemsString, order.UserID, order.Cost)
	util.JSONResponse(ctx, fasthttp.StatusOK, response)
}

func (s *postgresOrderStore) AddItem(ctx *fasthttp.RequestCtx, orderID string, itemID string) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Get the order from the database
		order := &Order{}
		err := tx.Model(&Order{}).
			Where("id = ?", orderID).
			First(order).
			Error
		if err == gorm.ErrRecordNotFound {
			util.NotFound(ctx)
			return errors.New("order not found")
		} else if err != nil {
			util.InternalServerError(ctx)
			return errwrap.Wrap(err, "unable to get order")
		}

		// Get the price of the item
		c := fasthttp.Client{}
		status, resp, err := c.Get([]byte{}, fmt.Sprintf("%s/stock/find/%s", s.urls.Stock, itemID))
		if err != nil {
			util.InternalServerError(ctx)
			return errwrap.Wrap(err, "unable to get item price")
		} else if status != fasthttp.StatusOK {
			ctx.SetStatusCode(status)
			return errors.New("error while getting item price")
		}
		pricePart := strings.Split(string(resp), "\"price\": ")[1]
		price, err := strconv.Atoi(pricePart[:len(pricePart)-1])
		if err != nil {
			util.InternalServerError(ctx)
			return errwrap.Wrap(err, "malformed response from stock service")
		}

		// Add the item to the order and update the price of the order
		items := itemStringToMap(order.Items)
		items[itemID] = price
		itemsString := mapToItemString(items)
		cost := order.Cost + price

		// Save the updated order in the database
		err = tx.Model(&Order{}).
			Where("id = ?", orderID).
			Update("items", itemsString).
			Update("cost", cost).
			Error
		if err == gorm.ErrRecordNotFound {
			util.NotFound(ctx)
			return errwrap.Wrap(err, "unable to find order")
		} else if err != nil {
			util.InternalServerError(ctx)
			return errwrap.Wrap(err, "unable to update order")
		}

		return nil
	})
	if err != nil {
		logrus.WithError(err).Error("unable to add item to order")
		return
	}
	util.Ok(ctx)
}

func (s *postgresOrderStore) RemoveItem(ctx *fasthttp.RequestCtx, orderID string, itemID string) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		order := &Order{}
		err := tx.Model(&Order{}).
			Where("id = ?", orderID).
			First(order).
			Error
		if err == gorm.ErrRecordNotFound {
			util.NotFound(ctx)
			return errors.New("order not found")
		} else if err != nil {
			util.InternalServerError(ctx)
			return errwrap.Wrap(err, "unable to get order from database")
		}

		// Remove the item from the order and update the price of the order
		items := itemStringToMap(order.Items)
		cost := order.Cost - items[itemID]
		delete(items, itemID)
		itemsString := mapToItemString(items)

		err = tx.Model(&Order{}).
			Where("id = ?", orderID).
			Update("items", itemsString).
			Update("cost", cost).
			Error
		if err == gorm.ErrRecordNotFound {
			util.NotFound(ctx)
			return errors.New("order not found")
		} else if err != nil {
			util.InternalServerError(ctx)
			return errwrap.Wrap(err, "unable to update order")
		}

		return nil
	})
	if err != nil {
		logrus.WithError(err).Error("unable to remove item from order")
		return
	}

	util.Ok(ctx)
}

func (s *postgresOrderStore) Checkout(ctx *fasthttp.RequestCtx, orderID string) {
	order := &Order{}
	err := s.db.Model(&Order{}).
		Where("id = ?", orderID).
		First(order).
		Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		return
	} else if err != nil {
		logrus.WithError(err).Error("unable to find order for checkout")
		util.InternalServerError(ctx)
		return
	}

	// Make the payment
	c := fasthttp.Client{}
	status, _, err := c.Post([]byte{}, fmt.Sprintf("%s/payment/pay/%s/%s/%d", s.urls.Payment, order.UserID, orderID, order.Cost), nil)
	if err != nil {
		logrus.WithError(err).Error("unable to pay for the order")
		util.InternalServerError(ctx)
		return
	} else if status != fasthttp.StatusOK {
		logrus.WithField("status", status).Error("error while paying for the order")
		ctx.SetStatusCode(status)
		return
	}

	// Subtract stock for each item in the order
	items := itemStringToMap(order.Items)
	for k := range items {
		status, _, err := c.Post([]byte{}, fmt.Sprintf("%s/stock/subtract/%s/1", s.urls.Stock, k), nil)
		if err != nil {
			logrus.WithError(err).Error("unable to subtract stock")
			util.InternalServerError(ctx)
			return
		}
		if status != fasthttp.StatusOK {
			logrus.WithField("status", status).Error("error while subtracting stock")
			ctx.SetStatusCode(status)
			return
		}
	}

	util.Ok(ctx)
}
