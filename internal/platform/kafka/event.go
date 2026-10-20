package kafka

type PropertyCreatedEvent struct {
	EventID        string `json:"id"`
	PropertyID     string `json:"property_id"`
	Title		 string `json:"title"`
	CreatedAt int64  `json:"created_at"`
}