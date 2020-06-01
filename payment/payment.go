package payment

type paymentStore struct {
	ID     string `sql:"type:uuid;primary_key;default:uuid_generate_v4()"`
	amount   int
}

func newPaymentStore() *paymentStore {
	return &paymentStore{}
}
