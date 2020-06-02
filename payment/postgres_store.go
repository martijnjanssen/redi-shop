package payment

import (
	"fmt"
	"github.com/jinzhu/gorm"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/valyala/fasthttp"
)

type postgresPaymentStore struct {
	db *gorm.DB
}

func newPostgresPaymentStore(db *gorm.DB) *postgresPaymentStore {
	// AutoMigrate structs to create or update database tables
	err := db.AutoMigrate(&Payment{}).Error
	if err != nil {
		panic(err)
	}

	return &postgresPaymentStore{
		db: db,
	}
}

func (s *postgresPaymentStore) Pay(ctx *fasthttp.RequestCtx, userID string, orderID string, amount int) {
	payment := &Payment{}
	err := s.db.Model(&Payment{}).
	Where("id = ?", userID).
		First(user).
		Error

	if err == gorm.ErrRecordNotFound {
			util.NotFound(ctx)
			return
	} else if err != nil {
			util.InternalServerError(ctx)
			return
	}

c := fasthttp.Client{}
status, _, err := c.Post([]byte{}, fmt.Sprintf("http://localhost/users/credit/subtract/%s/%d", userID, amount), nil)
if err != nil{
	util.InternalServerError(ctx)
	return
}
if status != fasthttp.StatusOK {
	util.StringResponse(ctx, status, "")
	return
}

payment := &Payment{OrderID:orderID, Amount:amount, Status:"paid"}
err := s.db.Model(&Payment{}).
	   Create(payment).
	   Error
	if err != nil {
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}

func (s *postgresPaymentStore) Cancel(ctx *fasthttp.RequestCtx, userID string, orderID string) {
	// TODO: retrieve the payment which needs to be cancelled
	payment := &Payment{}
	err := s.db.Model(&Payment{}).
		Where("order_id = ?" orderID).
		First(payment).
		Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		return
	} else if err != nil {
		util.InternalServerError(ctx)
		return
	}

	if payment.status == "cancelled" {
		util.BadRequest(ctx)
		return
	}

	// TODO: add the credit back to the user
	c := fasthttp.Client{}
	status, _, err := c.Post([]byte{}, fmt.Sprintf("http://localhost/users/credit/add/%s/%d", userID, payment.amount), nil)
	if err != nil{
		util.InternalServerError(ctx)
		return
	}
	if status != fasthttp.StatusOK {
		util.StringResponse(ctx, status, "")
		return
	}
	// TODO: update the status of the payment as "cancelled"
	err := s.db.Model(&Payment{}).
		Where("order_id = ?", orderID).
		Update("status", "cancelled").
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

func (s *postgresPaymentStore) PaymentStatus(ctx *fasthttp.RequestCtx, orderID string) {
	payment := &Payment{}
	err := s.db.Model(&Payment{}).
	Where("order_id = ?", orderID).
	Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		return
	} else if err != nil {
		util.InternalServerError(ctx)
		return
	}
	paid := "false"
	
	if payment.status == "paid" {
		paid = "true"
	}
	
	util.JSONResponse(ctx, fasthttp.StatusOK, fmt.Sprintf("{\"paid\": %s}", paid))
}
	
