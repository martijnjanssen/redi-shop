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
	exists := false
	payment := &Payment{}
	err := s.db.Model(&Payment{}).
		Where("id = ?", userID).
		First(payment).
		Error
	if err == gorm.ErrRecordNotFound {
		// Do nothing, record has to be created
		exists = false
	} else if err != nil {
		util.InternalServerError(ctx)
		return
	}

	// If record is found, check that it is not already paid
	if exists && payment.Status == "paid" {
		util.BadRequest(ctx)
		return
	}

	c := fasthttp.Client{}
	status, _, err := c.Post([]byte{}, fmt.Sprintf("http://localhost/users/credit/subtract/%s/%d", userID, amount), nil)
	if err != nil {
		util.InternalServerError(ctx)
		return
	} else if status != fasthttp.StatusOK {
		ctx.SetStatusCode(status)
		return
	}

	payment = &Payment{OrderID: orderID, Amount: amount, Status: "paid"}
	q := s.db.Model(&Payment{})
	// If it exists, update, otherwise, create
	if exists {
		q = q.
			Where("order_id = ?", payment.OrderID).
			Update("status", payment.Status)
	} else {
		q = q.Create(payment)
	}
	err = q.Error
	if err != nil {
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}

func (s *postgresPaymentStore) Cancel(ctx *fasthttp.RequestCtx, userID string, orderID string) {
	// Retrieve the payment which needs to be cancelled
	payment := &Payment{}
	err := s.db.Model(&Payment{}).
		Where("order_id = ?", orderID).
		First(payment).
		Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		return
	} else if err != nil {
		util.InternalServerError(ctx)
		return
	}

	if payment.Status == "cancelled" {
		util.BadRequest(ctx)
		return
	}

	// Refund the credit to the user
	c := fasthttp.Client{}
	status, _, err := c.Post([]byte{}, fmt.Sprintf("http://localhost/users/credit/add/%s/%d", userID, payment.Amount), nil)
	if err != nil {
		util.InternalServerError(ctx)
		return
	}
	if status != fasthttp.StatusOK {
		ctx.SetStatusCode(status)
		return
	}

	// Update the status of the payment to "cancelled"
	err = s.db.Model(&Payment{}).
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

	if payment.Status == "paid" {
		paid = "true"
	}

	util.JSONResponse(ctx, fasthttp.StatusOK, fmt.Sprintf("{\"paid\": %s}", paid))
}
