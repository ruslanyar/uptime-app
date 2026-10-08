package monitorcontroller

type monitorRequest struct {
	URL             string `json:"url" binding:"required" maxLength:"2048" example:"https://example.com"`
	IntervalSeconds int32  `json:"interval_seconds" binding:"required" minimum:"60" maximum:"86400" example:"60"`
}
