package order

import (
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/valyala/fasthttp"
)

type postgresOrderStore struct {
	db *gorm.DB
}

func newPostgresOrderStore(db *gorm.DB) *postgresOrderStore{
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
	}
	err := s.db.Model(&Order{}).
		Create(order).
		Error
	if err != nil {
		util.InternalServerError(ctx)
		return
	}

	response := fmt.Sprintf("{\"order_id\" : \"#{order.ID}\"}")
	util.StringResponse(ctx, fasthttp.StatusCreated, response)
}

func (s *postgresOrderStore) Remove(ctx *fasthttp.RequestCtx, orderID string){
	err := s.db.Model(&Order{}).
		Delete(&Order{ID: orderID}).
		Error
	if err != nil {
		util.InternalServerError(ctx)
	}

	util.StringResponse(ctx, fasthttp.StatusOK, "success")
}

func (s *postgresOrderStore) Find(ctx *fasthttp.RequestCtx, orderID string){
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

	response := fmt.Sprintf("{\"order_id\" : %d, \"paid\": %d, \"items\": %d, \"user_id\": %d, \"total_cost\": %d}", order.ID, order.Paid, order.Items, order.UserID, order.Cost)
	util.JsonResponse(ctx, fasthttp.StatusOK, response)

}

func (s *postgresOrderStore) AddItem(ctx *fasthttp.RequestCtx, orderID string, itemID string){
	//todo: check this out, how to update a list? And with an item object or just add ID?
	err := s.db.Model(&Order{}).
		Where("id = ?", orderID).
		Update("items", gorm.Expr("items + ?", itemID)).
		Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		return
	} else if err != nil {
		util.StringResponse(ctx, fasthttp.StatusInternalServerError, "failure")
		return
	}

	util.StringResponse(ctx, fasthttp.StatusOK, "success")

}

func (s *postgresOrderStore) RemoveItem(ctx *fasthttp.RequestCtx, orderID string, itemID string){
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

	//todo: same problem as with add item. How to update?



}

func (s *postgresOrderStore) Checkout(ctx *fasthttp.RequestCtx, orderID string){
	// make the payment (via payment service

	// subtract stock via stock service

	// return status
}