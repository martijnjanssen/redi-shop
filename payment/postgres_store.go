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
}

if user.credit-amount < 0 {
	util.BadRequest(ctx)
	return
}

err := s.db.Model(&Payment{}).
Where("id = ?", userID).
		Update("amount", gorm.Expr("credit - ?", amount)).
		Error

	if err != nil {
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}

func (s *postgresPaymentStore) Cancel(ctx *fasthttp.RequestCtx, userID string, orderID string) {
	payment := &Payment{}
	err := s.db.Model(&Payment{}).
	Where("id = ?", orderID).
		First(payment).
		Error

	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		return
	} else if err != nil {
		util.InternalServerError(ctx)
		return
	}
}

err = s.db.Model(&Payment{}).
		Where("userID = ? AND orderID = ?", userID, orderID).
		Update("status", gorm.Expr("cancel")).
		Error

	if err != nil {
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}