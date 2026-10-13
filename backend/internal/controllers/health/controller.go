// Package healthcontroller handles dependency readiness checks.
package healthcontroller

import (
	"context"
	"net/http"
	"time"
	"uptime-app/backend/internal/httpapi/response"
)

// Check must honor context cancellation and deadlines; nil means not ready.
type Check func(context.Context) error

type Controller struct{ check Check }

func New(check Check) *Controller { return &Controller{check: check} }

type Status struct {
	Status string `json:"status" binding:"required"`
}

// Ready checks PostgreSQL within one second or the request's earlier deadline.
// @Summary Check API readiness
// @Description Public readiness check. Checks PostgreSQL connectivity within one second; does not validate migrations or application workflows. Requests with Origin still require an allowed origin. Responses are not cached and contain no dependency error details.
// @Tags health
// @ID healthReady
// @Produce json
// @Success 200 {object} Status "Ready"
// @Header 200 {string} Cache-Control "no-store"
// @Failure 503 {object} Status "Not ready"
// @Header 503 {string} Cache-Control "no-store"
// @Failure 403 {object} response.ErrorResponse "Origin rejected"
// @Router /health/ready [get]
func (c *Controller) Ready(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	ctx, cancel := context.WithTimeout(r.Context(), time.Second)
	defer cancel()
	if c.check == nil || c.check(ctx) != nil || ctx.Err() != nil {
		response.JSON(w, http.StatusServiceUnavailable, Status{Status: "not_ready"})
		return
	}
	response.JSON(w, http.StatusOK, Status{Status: "ready"})
}
