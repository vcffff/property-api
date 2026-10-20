package viewing

import (
	"fmt"

	"gorm.io/gorm"
)

const activeSlotIndex = "idx_viewings_active_slot"

// EnsureActiveSlotIndex runs after AutoMigrate; never deletes existing bookings.
func EnsureActiveSlotIndex(db *gorm.DB) error {
	var duplicates bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM viewings
			WHERE status IN ('pending', 'confirmed')
			GROUP BY property_id, starts_at HAVING COUNT(*) > 1
		)
	`).Scan(&duplicates).Error; err != nil {
		return fmt.Errorf("check active viewing duplicates: %w", err)
	}
	if duplicates {
		return fmt.Errorf("cannot create %s: resolve duplicate pending/confirmed viewings for the same property_id and starts_at", activeSlotIndex)
	}
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_viewings_active_slot
		ON viewings (property_id, starts_at)
		WHERE status IN ('pending', 'confirmed')
	`).Error; err != nil {
		return fmt.Errorf("create active viewing slot index: %w", err)
	}
	return nil
}
