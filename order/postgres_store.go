package order

import (
	"fmt"

	"github.com/jinzhu/gorm"
	"github.com/martijnjanssen/redi-shop/stock"
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
	}
	err := s.db.Model(&Order{}).
		Create(order).
		Error
	if err != nil {
		util.InternalServerError(ctx)
		return
	}

	util.JSONResponse(ctx, fasthttp.StatusCreated, fmt.Sprintf("{\"order_id\" : %s}", order.ID))
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
		Preload("Items"). // Preload loads the linked items in the order
		First(order).
		Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		return
	} else if err != nil {
		util.InternalServerError(ctx)
		return
	}

	// Calculate the latest price for the order
	order.CalculateCost()

	// Build items JSON string representation
	items := "["
	for i := range order.Items {
		items += order.Items[i].ID + ","
	}
	items = items[:len(items)-1] + "]"

	response := fmt.Sprintf("{\"order_id\" : %s, \"paid\": %t, \"items\": %s, \"user_id\": %s, \"total_cost\": %d}", order.ID, order.Paid, items, order.UserID, order.Cost)
	util.JSONResponse(ctx, fasthttp.StatusOK, response)

}

func (s *postgresOrderStore) AddItem(ctx *fasthttp.RequestCtx, orderID string, itemID string) {
	order := &Order{}
	err := s.db.Model(&Order{}).
		Where("id = ?", orderID).
		Preload("Items"). // Preload loads the linked items in the order
		First(order).
		Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		return
	} else if err != nil {
		util.InternalServerError(ctx)
		return
	}

	err = s.db.Model(&Order{}).
		Where("id = ?", orderID).
		Update("items", append(order.Items, stock.Stock{ID: itemID})).
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

	items := order.Items
	for i := range items {
		// Find the item we want to remove
		if items[i].ID == itemID {
			// Remove the item from the list
			items[i] = items[len(items)-1]
			items = items[:len(items)-1]
			break
		}
	}

	err = s.db.Model(&Order{}).
		Where("id = ?", orderID).
		Update("items", items).
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
		Preload("Items"). // Preload loads the linked items in the order
		First(order).
		Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		return
	} else if err != nil {
		util.InternalServerError(ctx)
		return
	}

	// Update the latest price on the order
	order.CalculateCost()

	c := fasthttp.Client{}
	// Subtract stock for each item in the order
	for i := range order.Items {
		status, _, err := c.Post([]byte{}, fmt.Sprintf("http://localhost/stock/subtract/%s/1", order.Items[i].ID), nil)
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

	// make the payment (via payment service)
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

	// TODO: Commit transaction
	util.Ok(ctx)
}
