package payment

//type paymentRouteHandler struct {
	//paymentStore *paymentStore
//}

//func NewRouteHandler() *paymentRouteHandler {
//	return &paymentRouteHandler{
//		paymentStore: newPaymentStore(),
//	}
//}
import (
	"strconv"
    "fmt"
	"github.com/jinzhu/gorm"
	"github.com/valyala/fasthttp"
)

type paymentStore interface {
	Pay(*fasthttp.RequestCtx, string, string, int)
	Cancel(*fasthttp.RequestCtx, string, string)
	PaymentStatus(*fasthttp.RequestCtx, string)
}

type paymentRouteHandler struct {
	paymentStore paymentStore
}

func NewRouteHandler(db *gorm.DB) *paymentRouteHandler {
	return &paymentRouteHandler{
		paymentStore: newPostgresPaymentStore(db),
	}
}

//Payment - subtracts the amount of the order from the user’s credit (returns failure if credit is not enough)
func ( *paymentRouteHandler) PayForOrder(ctx *fasthttp.RequestCtx) {
	userID := ctx.UserValue("user_id").(string)
	orderID := ctx.UserValue("order_id").(string)
	amount, err := strconv.Atoi(ctx.UserValue("amount").(string))
	
	if err != nil {
		ctx.SetStatusCode(fasthttp.StatusBadRequest)
		ctx.SetBodyString("insufficient credit")
		return
	}
	h.paymentStore.Pay(ctx, userID, orderID, amount)
}

//Cancel the payment made by a user
func ( *paymentRouteHandler) CancelOrder(ctx *fasthttp.RequestCtx) {
	 userID :=ctx.UserValue("user_id").(string
	 orderID := ctx.UserValue("order_id").(string)		
	 h.paymentStore.Cancel(ctx, userID, orderID)
}

//Return the status of a payment
func ( *paymentRouteHandler) GetPaymentStatus(ctx *fasthttp.RequestCtx) {
	orderID := ctx.UserValue("order_id").(string)
	h.paymentStore.PaymentStatus(ctx, orderID)
}
