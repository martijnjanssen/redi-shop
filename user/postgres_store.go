package user

import (
	"fmt"

	"github.com/jinzhu/gorm"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/valyala/fasthttp"
)

type postgresUserStore struct {
	db *gorm.DB
}

func newPostgresUserStore(db *gorm.DB) *postgresUserStore {
	// AutoMigrate structs to create or update database tables
	err := db.AutoMigrate(&User{}).Error
	if err != nil {
		panic(err)
	}

	return &postgresUserStore{
		db: db,
	}
}

func (s *postgresUserStore) Create(ctx *fasthttp.RequestCtx) {
	user := &User{}
	err := s.db.Model(&User{}).
		Create(user).
		Error
	if err != nil {
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
		util.InternalServerError(ctx)
		return
	}

	util.JSONResponse(ctx, fasthttp.StatusOK, fmt.Sprintf("{\"user_id\": \"%s\", \"credit\": %d}", user.ID, user.Credit))
}

func (s *postgresUserStore) SubtractCredit(ctx *fasthttp.RequestCtx, userID string, amount int) {
	user := &User{}
	err := s.db.Model(&User{}).
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

	if user.Credit-amount < 0 {
		util.BadRequest(ctx)
		return
	}

	err = s.db.Model(&User{}).
		Where("id = ?", userID).
		Update("credit", gorm.Expr("credit - ?", amount)).
		Error

	if err != nil {
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}

func (s *postgresUserStore) AddCredit(ctx *fasthttp.RequestCtx, userID string, amount int) {
	err := s.db.Model(&User{}).
		Where("id = ?", userID).
		Update("credit", gorm.Expr("credit + ?", amount)).
		Error
	if err != nil {
		util.InternalServerError(ctx)
		return
	}

	util.Ok(ctx)
}
