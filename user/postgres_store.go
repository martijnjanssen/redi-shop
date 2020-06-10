package user

import (
	"errors"
	"fmt"

	"github.com/jinzhu/gorm"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/sirupsen/logrus"
	"github.com/valyala/fasthttp"

	errwrap "github.com/pkg/errors"
)

type postgresUserStore struct {
	db   *gorm.DB
	urls *util.Services
}

func newPostgresUserStore(db *gorm.DB, urls *util.Services) *postgresUserStore {
	// AutoMigrate structs to create or update database tables
	err := db.AutoMigrate(&User{}).Error
	if err != nil {
		panic(err)
	}

	return &postgresUserStore{
		db:   db,
		urls: urls,
	}
}

func (s *postgresUserStore) Create(ctx *fasthttp.RequestCtx) {
	user := &User{}
	err := s.db.Model(&User{}).
		Create(user).
		Error
	if err != nil {
		logrus.WithError(err).Error("unable to create new user")
		util.InternalServerError(ctx)
		return
	}

	util.JSONResponse(ctx, fasthttp.StatusCreated, fmt.Sprintf("{\"user_id\": \"%s\"}", user.ID))
}

func (s *postgresUserStore) Remove(ctx *fasthttp.RequestCtx, userID string) {
	err := s.db.Model(&User{}).
		Delete(&User{ID: userID}).
		Error
	if err != nil {
		logrus.WithError(err).Error("unable to remove user")
		util.InternalServerError(ctx)
	}

	util.Ok(ctx)
}

func (s *postgresUserStore) Find(ctx *fasthttp.RequestCtx, userID string) {
	user := &User{}
	err := s.db.Model(&User{}).
		Where("id = ?", userID).
		First(user).
		Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		return
	} else if err != nil {
		logrus.WithError(err).Error("unable to find user")
		util.InternalServerError(ctx)
		return
	}

	util.JSONResponse(ctx, fasthttp.StatusOK, fmt.Sprintf("{\"user_id\": \"%s\", \"credit\": %d}", user.ID, user.Credit))
}

func (s *postgresUserStore) SubtractCredit(ctx *fasthttp.RequestCtx, userID string, amount int) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		user := &User{}
		err := tx.Model(&User{}).
			Where("id = ?", userID).
			First(user).
			Error
		if err == gorm.ErrRecordNotFound {
			util.NotFound(ctx)
			return errors.New("user not found")
		} else if err != nil {
			util.InternalServerError(ctx)
			return errwrap.Wrap(err, "unable to get user")
		}

		if user.Credit-amount < 0 {
			util.BadRequest(ctx)
			return errors.New("credit cannot go below 0")
		}

		err = tx.Model(&User{}).
			Where("id = ?", userID).
			Update("credit", gorm.Expr("credit - ?", amount)).
			Error
		if err == gorm.ErrRecordNotFound {
			util.NotFound(ctx)
			return errwrap.Wrap(err, "user not found")
		} else if err != nil {
			util.InternalServerError(ctx)
			return errwrap.Wrap(err, "unable to update credit")
		}

		return nil
	})
	if err != nil {
		logrus.WithError(err).Error("unable to subtract credit")
		return
	}

	util.Ok(ctx)
}

func (s *postgresUserStore) AddCredit(ctx *fasthttp.RequestCtx, userID string, amount int) {
	err := s.db.Model(&User{}).
		Where("id = ?", userID).
		Update("credit", gorm.Expr("credit + ?", amount)).
		Error
	if err == gorm.ErrRecordNotFound {
		util.NotFound(ctx)
		return
	} else if err != nil {
		logrus.WithError(err).Error("unable to add credit")
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}
