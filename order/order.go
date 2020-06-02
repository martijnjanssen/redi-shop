package order

type Order struct {
	ID     string `sql:"type:uuid;primary_key;default:uuid_generate_v4()"`
	UserID string
	Paid   bool
	Items  string
	Cost   int
}
