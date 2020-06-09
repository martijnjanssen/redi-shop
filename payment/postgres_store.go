package payment

import (
	"fmt"

	"github.com/jinzhu/gorm"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"
)

type postgresPaymentStore struct {
	db   *gorm.DB
	urls *util.Services
}

func newPostgresPaymentStore(db *gorm.DB, urls *util.Services) *postgresPaymentStore {
	// AutoMigrate structs to create or update database tables
	err := db.AutoMigrate(&Payment{}).Error
	if err != nil {
		panic(err)
	}

	return &postgresPaymentStore{
		db:   db,
		urls: urls,
	}
}

func (s *postgresPaymentStore) Pay(ctx *fasthttp.RequestCtx, userID string, orderID string, amount int) {
	tx := util.StartTX(s.db)

	exists := false
	payment := &Payment{}
	err := tx.Model(&Payment{}).
		Where("order_id = ?", orderID).
		First(payment).
		Error
	if err == gorm.ErrRecordNotFound {
		// Do nothing, record has to be created
		exists = false
	} else if err != nil {
		logrus.WithError(err).Error("unable to retrieve payment status")
		util.InternalServerError(ctx)
		util.Rollback(tx)
		return
	}

	// If record is found, check that it is not already paid
	if exists && payment.Status == "paid" {
		logrus.WithField("order_id", orderID).Info("order was already paid")
		util.BadRequest(ctx)
		util.Rollback(tx)
		return
	}

	c := fasthttp.Client{}
	status, _, err := c.Post([]byte{}, fmt.Sprintf("%s/users/credit/subtract/%s/%d", s.urls.User, userID, amount), nil)
	if err != nil {
		logrus.WithError(err).Error("unable to subtract credit")
		util.InternalServerError(ctx)
		util.Rollback(tx)
		return
	} else if status != fasthttp.StatusOK {
		logrus.WithField("status", status).Error("error while subtracting credit")
		ctx.SetStatusCode(status)
		util.Rollback(tx)
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
		logrus.WithField("exists", exists).WithError(err).Error("unable to update payment status")
		util.InternalServerError(ctx)
		util.Rollback(tx)
		return
	}

	if !util.Commit(tx) {
		util.InternalServerError(ctx)
		return
	}
	util.Ok(ctx)
}

func (s *postgresPaymentStore) Cancel(ctx *fasthttp.RequestCtx, userID string, orderID string) {
	tx := util.StartTX(s.db)

	// Retrieve the payment which needs to be cancelled
	payment := &Payment{}
	err := tx.Model(&Payment{}).
		Where("order_id = ?", orderID).
		First(payment).
		Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		util.Rollback(tx)
		return
	} else if err != nil {
		logrus.WithError(err).Error("unable to retrieve payment to cancel")
		util.InternalServerError(ctx)
		util.Rollback(tx)
		return
	}

	if payment.Status == "cancelled" {
		logrus.Info("payment was already cancelled")
		util.BadRequest(ctx)
		util.Rollback(tx)
		return
	}

	// Refund the credit to the user
	c := fasthttp.Client{}
	status, _, err := c.Post([]byte{}, fmt.Sprintf("%s/users/credit/add/%s/%d", s.urls.User, userID, payment.Amount), nil)
	if err != nil {
		logrus.WithError(err).Error("unable to refund credit to user")
		util.InternalServerError(ctx)
		util.Rollback(tx)
		return
	}
	if status != fasthttp.StatusOK {
		logrus.WithField("status", status).Error("error while refunding credit to user")
		ctx.SetStatusCode(status)
		util.Rollback(tx)
		return
	}

	// Update the status of the payment to "cancelled"
	err = tx.Model(&Payment{}).
		Where("order_id = ?", orderID).
		Update("status", "cancelled").
		Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		util.Rollback(tx)
		return
	} else if err != nil {
		logrus.WithError(err).Error("unable to update payment status")
		util.InternalServerError(ctx)
		util.Rollback(tx)
		return
	}

	if !util.Commit(tx) {
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
