package order

import (
	"fmt"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/martijnjanssen/redi-shop/util"
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
		Items:  "",
	}
	err := s.db.Model(&Order{}).
		Create(order).
		Error
	if err != nil {
		util.InternalServerError(ctx)
		return
	}

	response := fmt.Sprintf("{\"order_id\" : %s}", order.ID)
	util.JsonResponse(ctx, fasthttp.StatusCreated, response)
}

func (s *postgresOrderStore) Remove(ctx *fasthttp.RequestCtx, orderID string) {
	err := s.db.Model(&Order{}).
		Delete(&Order{ID: orderID}).
		Error
	if err != nil {
		util.StringResponse(ctx, fasthttp.StatusInternalServerError, "failure")
		return
	}

	util.StringResponse(ctx, fasthttp.StatusOK, "success")
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

	response := fmt.Sprintf("{\"order_id\" : %s, \"paid\": %t, \"items\": %s, \"user_id\": %s, \"total_cost\": %d}", order.ID, order.Paid, order.Items, order.UserID, order.Cost)
	util.JsonResponse(ctx, fasthttp.StatusOK, response)

}

func (s *postgresOrderStore) AddItem(ctx *fasthttp.RequestCtx, orderID string, itemID string) {
	order := &Order{}
	err := s.db.Model(&Order{}).
		Where("id = ?", orderID).
		First(order).
		Error
	if err == gorm.ErrRecordNotFound {
		util.StringResponse(ctx, fasthttp.StatusNotFound, "failure")
		return
	} else if err != nil {
		util.StringResponse(ctx, fasthttp.StatusInternalServerError, "failure")
		return
	}

	// Convert items and add item to the map
	items := stringToMap(order.Items)
	items[orderID] = true

	err = s.db.Model(&Order{}).
		Where("id = ?", orderID).
		Update("items", mapToString(items)).
		Error
	if err == gorm.ErrRecordNotFound {
		util.StringResponse(ctx, fasthttp.StatusNotFound, "failure")
		return
	} else if err != nil {
		util.StringResponse(ctx, fasthttp.StatusInternalServerError, "failure")
		return
	}

	util.StringResponse(ctx, fasthttp.StatusOK, "success")
}

func (s *postgresOrderStore) RemoveItem(ctx *fasthttp.RequestCtx, orderID string, itemID string) {
	order := &Order{}
	err := s.db.Model(&Order{}).
		Where("id = ?", orderID).
		First(order).
		Error
	if err == gorm.ErrRecordNotFound {
		util.StringResponse(ctx, fasthttp.StatusNotFound, "failure")
		return
	} else if err != nil {
		util.StringResponse(ctx, fasthttp.StatusInternalServerError, "failure")
		return
	}

	// Convert items and remove the item from the map
	items := stringToMap(order.Items)
	delete(items, itemID)

	err = s.db.Model(&Order{}).
		Where("id = ?", orderID).
		Update("items", mapToString(items)).
		Error
	if err == gorm.ErrRecordNotFound {
		util.StringResponse(ctx, fasthttp.StatusNotFound, "failure")
		return
	} else if err != nil {
		util.StringResponse(ctx, fasthttp.StatusInternalServerError, "failure")
		return
	}

	util.StringResponse(ctx, fasthttp.StatusOK, "success")

}

func (s *postgresOrderStore) Checkout(ctx *fasthttp.RequestCtx, orderID string) {
	// make the payment (via payment service)

	// subtract stock via stock service

	// return status
}

func stringToMap(itemString string) map[string]bool {
	items := strings.Split(itemString, ",")
	m := map[string]bool{}

	for _, item := range items {
		m[item] = true
	}

	return m
}

func mapToString(items map[string]bool) string {
	s := ""

	for item, _ := range items {
		s = fmt.Sprintf("%s,%s", s, item)
	}

	return s
}
