package server

import (
	"fmt"
	"runtime/debug"

	"github.com/martijnjanssen/redi-shop/order"

	"github.com/fasthttp/router"
	"github.com/martijnjanssen/redi-shop/payment"
	"github.com/martijnjanssen/redi-shop/stock"
	"github.com/martijnjanssen/redi-shop/user"
	"github.com/martijnjanssen/redi-shop/util"
	"github.com/valyala/fasthttp"
)

// returns the router with all user routes
func getUserRouter(conn *util.Connection) fasthttp.RequestHandler {
	h := user.NewRouteHandler(conn)

	r := router.New()
	r.PanicHandler = panicHandler

	r.POST("/users/create/", h.CreateUser)
	r.DELETE("/users/remove/{user_id}", h.RemoveUser)
	r.GET("/users/find/{user_id}", h.FindUser)

	r.POST("/users/credit/subtract/{user_id}/{amount}", h.SubtractUserCredit)
	r.POST("/users/credit/add/{user_id}/{amount}", h.AddUserCredit)

	return r.Handler
}

func getOrderRouter(conn *util.Connection) fasthttp.RequestHandler {
	h := order.NewRouteHandler(conn)

	r := router.New()
	r.PanicHandler = panicHandler

	r.POST("/orders/create/{user_id}", h.CreateOrder)
	r.DELETE("/orders/remove/{order_id}", h.RemoveOrder)
	r.GET("/orders/find/{order_id}", h.FindOrder)
	r.POST("/orders/additem/{order_id}/{item_id}", h.AddOrderItem)
	r.DELETE("/orders/removeitem/{order_id}/{item_id}", h.RemoveOrderItem)
	r.POST("/orders/checkout/{order_id}", h.CheckoutOrder)

	return r.Handler
}

func getStockRouter(conn *util.Connection) fasthttp.RequestHandler {
	h := stock.NewRouteHandler(conn)

	r := router.New()
	r.PanicHandler = panicHandler

	r.GET("/stock/find/{item_id}", h.FindStockItem)
	r.POST("/stock/subtract/{item_id}/{number}", h.SubtractStockNumber)
	r.POST("/stock/add/{item_id}/{number}", h.AddStockNumber)
	r.POST("/stock/item/create/{price}", h.CreateStockItem)
	r.POST("/stock/message", h.HandleMessage)

	return r.Handler
}

func getPaymentRouter(conn *util.Connection) fasthttp.RequestHandler {
	h := payment.NewRouteHandler(conn)

	r := router.New()
	r.PanicHandler = panicHandler

	r.GET("/payment/status/{order_id}", h.GetPaymentStatus)
	r.POST("/payment/message", h.HandleMessage)

	return r.Handler
}

func panicHandler(ctx *fasthttp.RequestCtx, p interface{}) {
	fmt.Println("Recovered in panicHandler", p, string(debug.Stack()))

	ctx.Response.Reset()
	ctx.SetStatusCode(fasthttp.StatusInternalServerError)
}
