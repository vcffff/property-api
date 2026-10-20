package viewing

type ViewingStatus string

const (
	ViewingStatusPending   ViewingStatus = "pending"
	ViewingStatusConfirmed ViewingStatus = "confirmed"
	ViewingStatusCancelled ViewingStatus = "cancelled"
	ViewingStatusCompleted ViewingStatus = "completed"
)

func (s ViewingStatus) IsValid() bool {
	switch s {
	case ViewingStatusPending, ViewingStatusConfirmed, ViewingStatusCancelled, ViewingStatusCompleted:
		return true
	default:
		return false
	}
}

// transitionSources is also the predicate used by the atomic database update.
func transitionSources(target ViewingStatus) []ViewingStatus {
	switch target {
	case ViewingStatusConfirmed:
		return []ViewingStatus{ViewingStatusPending}
	case ViewingStatusCancelled:
		return []ViewingStatus{ViewingStatusPending, ViewingStatusConfirmed}
	case ViewingStatusCompleted:
		return []ViewingStatus{ViewingStatusConfirmed}
	default:
		return nil
	}
}
