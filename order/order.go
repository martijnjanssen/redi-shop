package order

type Order struct {
	ID     string
	UserID string
	Paid   bool
	Items  string
	Cost   int
}
