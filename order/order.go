package order

import "github.com/martijnjanssen/redi-shop/stock"

type Order struct {
	ID     string `sql:"type:uuid;primary_key;default:uuid_generate_v4()"`
	UserID string
	Paid   bool
	Items  []stock.Stock `gorm:"many2many:orderitems"`
	Cost   int
}

// TODO: this method should probably be a query, also in the transaction, which sums the items
func (o *Order) CalculateCost() {
	o.Cost = 0

	for _, item := range o.Items {
		o.Cost += item.Price
	}
}
