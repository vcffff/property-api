package application

import (
	"fmt"

	"gorm.io/gorm"
)

const activeApplicationIndex = "idx_applications_one_active_per_user_property"

// EnsureActiveApplicationIndex runs after AutoMigrate. Existing duplicate data
// must be resolved explicitly; startup never deletes or changes applications.
func EnsureActiveApplicationIndex(db *gorm.DB) error {
	var duplicates bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM applications
			WHERE status IN ('pending', 'approved')
			GROUP BY user_id, property_id
			HAVING COUNT(*) > 1
		)
	`).Scan(&duplicates).Error; err != nil {
		return fmt.Errorf("check active application duplicates: %w", err)
	}
	if duplicates {
		return fmt.Errorf("cannot create %s: resolve duplicate pending/approved applications for the same user_id and property_id", activeApplicationIndex)
	}

	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_applications_one_active_per_user_property
		ON applications (user_id, property_id)
		WHERE status IN ('pending', 'approved')
	`).Error; err != nil {
		return fmt.Errorf("create active application index: %w", err)
	}
	return nil
}
