package payment

import (
	"errors"
	"fmt"

	"github.com/jinzhu/gorm"
	"github.com/martijnjanssen/redi-shop/util"
	errwrap "github.com/pkg/errors"
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
	err := s.db.Transaction(func(tx *gorm.DB) error {
		exists := true
		payment := &Payment{}
		err := tx.Model(&Payment{}).
			Where("order_id = ?", orderID).
			First(payment).
			Error
		if err == gorm.ErrRecordNotFound {
			// Do nothing, record has to be created
			exists = false
		} else if err != nil {
			util.InternalServerError(ctx)
			return errwrap.Wrap(err, "unable to retrieve payment status")
		}

		// If record is found, check that it is not already paid
		if exists && payment.Status == "paid" {
			util.BadRequest(ctx)
			return errors.New("order was already paid")
		}

		c := fasthttp.Client{}
		status, _, err := c.Post([]byte{}, fmt.Sprintf("%s/users/credit/subtract/%s/%d", s.urls.User, userID, amount), nil)
		if err != nil {
			util.InternalServerError(ctx)
			return errwrap.Wrap(err, "unable to subtract credit")
		} else if status != fasthttp.StatusOK {
			ctx.SetStatusCode(status)
			return errors.New("error while subtracting credit")
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
			return errwrap.Wrap(err, "unable to update payment status")
		}

		return nil
	})
	if err != nil {
		logrus.WithError(err).Error("unable to pay for order")
		return
	}

	util.Ok(ctx)
}

func (s *postgresPaymentStore) Cancel(ctx *fasthttp.RequestCtx, userID string, orderID string) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Retrieve the payment which needs to be cancelled
		payment := &Payment{}
		err := tx.Model(&Payment{}).
			Where("order_id = ?", orderID).
			First(payment).
			Error
		if err == gorm.ErrRecordNotFound {
			util.NotFound(ctx)
			return errwrap.Wrap(err, "payment to cancel not found")
		} else if err != nil {
			util.InternalServerError(ctx)
			return errwrap.Wrap(err, "unable to retrieve payment to cancel")
		}

		if payment.Status == "cancelled" {
			util.BadRequest(ctx)
			return errors.New("payment already canceled")
		}

		// Refund the credit to the user
		c := fasthttp.Client{}
		status, _, err := c.Post([]byte{}, fmt.Sprintf("%s/users/credit/add/%s/%d", s.urls.User, userID, payment.Amount), nil)
		if err != nil {
			util.InternalServerError(ctx)
			return errwrap.Wrap(err, "unable to refund user credit")
		} else if status != fasthttp.StatusOK {
			ctx.SetStatusCode(status)
			return errors.New("error refunding user credit")
		}

		// Update the status of the payment to "cancelled"
		err = tx.Model(&Payment{}).
			Where("order_id = ?", orderID).
			Update("status", "cancelled").
			Error
		if err == gorm.ErrRecordNotFound {
			util.NotFound(ctx)
			return errwrap.Wrap(err, "unable to update payment status")
		} else if err != nil {
			util.InternalServerError(ctx)
			return errwrap.Wrap(err, "unable to update payment status")
		}

		return nil
	})
	if err != nil {
		logrus.WithError(err).Error("unable to cancel payment")
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
