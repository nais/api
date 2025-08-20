package alerts

import "time"

type Alerts struct {
	// The name of the alert.
	Name string `json:"name"`
	// The severity level of the alert.
	Severity string `json:"severity"`
	// The current status of the alert.
	Status string `json:"status"`
	// The time when the alert was created.
	CreatedAt time.Time `json:"createdAt"`
	// The time when the alert was last updated.
	UpdatedAt time.Time `json:"updatedAt"`
}
