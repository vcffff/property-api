package property

type UpdatePropertyRequest struct {
	Title   *string `json:"title"`
	Address *string `json:"address"`
	Price   *int    `json:"price"`
}
