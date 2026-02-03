package health

import (
	"context"
	"net/http"

	"github.com/controlplane-com/libs-go/pkg/web"
)

// LivenessResponse represents the response for the liveness endpoint
type LivenessResponse struct {
	Status       string `json:"status"`
	BrokerID     int32  `json:"brokerId"`
	BrokerFound  bool   `json:"brokerFound"`
	ErrorMessage string `json:"error,omitempty"`
}

// LivenessHandler handles GET /health/live requests
func (c *Checker) LivenessHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	response := LivenessResponse{
		BrokerID: c.brokerID,
	}

	adm, cleanup, err := c.clientFactory()
	if err != nil {
		c.logger.Error("failed to create kafka client", "error", err)
		response.Status = "unhealthy"
		response.ErrorMessage = err.Error()
		_, _ = web.ReturnResponseWithCode(w, response, http.StatusServiceUnavailable)
		return
	}
	defer cleanup()

	brokerFound, err := c.BrokerInMetadata(ctx, adm)
	if err != nil {
		c.logger.Error("failed to check broker in metadata", "error", err)
		response.Status = "unhealthy"
		response.ErrorMessage = err.Error()
		_, _ = web.ReturnResponseWithCode(w, response, http.StatusServiceUnavailable)
		return
	}

	response.BrokerFound = brokerFound

	if !brokerFound {
		c.logger.Warn("broker not found in cluster metadata", "brokerId", c.brokerID)
		response.Status = "unhealthy"
		response.ErrorMessage = "broker not found in cluster metadata"
		_, _ = web.ReturnResponseWithCode(w, response, http.StatusServiceUnavailable)
		return
	}

	response.Status = "healthy"
	_, _ = web.ReturnResponse(w, response)
}

// CheckLiveness performs a liveness check and returns the result
func (c *Checker) CheckLiveness(ctx context.Context) CheckResult {
	adm, cleanup, err := c.clientFactory()
	if err != nil {
		return CheckResult{
			Healthy: false,
			Message: err.Error(),
		}
	}
	defer cleanup()

	brokerFound, err := c.BrokerInMetadata(ctx, adm)
	if err != nil {
		return CheckResult{
			Healthy: false,
			Message: err.Error(),
		}
	}

	if !brokerFound {
		return CheckResult{
			Healthy: false,
			Message: "broker not found in cluster metadata",
		}
	}

	return CheckResult{
		Healthy: true,
	}
}
