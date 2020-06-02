package payment

type Payment struct {
	OrderID string `sql:"type:uuid;primary_key"`
	Amount  int
	Status  string
}
