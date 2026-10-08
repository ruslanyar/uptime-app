package monitorcontroller

import "uptime-app/backend/internal/monitor"

type listResponse struct {
	Monitors []monitor.Monitor `json:"monitors" binding:"required"`
}
