package property

type Property struct {
	ID      uint   `json:"id" gorm:"primaryKey"`
	Title   string `json:"title"`
	Address string `json:"address"`
	Price   int    `json:"price"`
}
