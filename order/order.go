package order

type Order struct {
	ID				string
	Paid 			bool
	Items 			[]string
	UserID 			string
	Cost			int
}

