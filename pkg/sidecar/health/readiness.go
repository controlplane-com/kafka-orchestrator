package health

import (
	"context"
	"net/http"

	"github.com/controlplane-com/libs-go/pkg/web"
)

// ReadinessResponse represents the response for the readiness endpoint
type ReadinessResponse struct {
	Status                    string `json:"status"`
	BrokerID                  int32  `json:"brokerId"`
	BrokerRegistered          bool   `json:"brokerRegistered"`
	ControllerElected         bool   `json:"controllerElected"`
	UnderReplicatedPartitions int    `json:"underReplicatedPartitions"`
	LogDirsHealthy            bool   `json:"logDirsHealthy"`
	ErrorMessage              string `json:"error,omitempty"`
}

// ReadinessHandler handles GET /health/ready requests
func (c *Checker) ReadinessHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	response := ReadinessResponse{
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

	// Check 1: Broker registered in cluster metadata
	brokerRegistered, err := c.BrokerInMetadata(ctx, adm)
	if err != nil {
		c.logger.Error("failed to check broker in metadata", "error", err)
		response.Status = "unhealthy"
		response.ErrorMessage = err.Error()
		_, _ = web.ReturnResponseWithCode(w, response, http.StatusServiceUnavailable)
		return
	}
	response.BrokerRegistered = brokerRegistered

	if !brokerRegistered {
		c.logger.Warn("broker not registered in cluster metadata", "brokerId", c.brokerID)
		response.Status = "unhealthy"
		response.ErrorMessage = "broker not registered in cluster metadata"
		_, _ = web.ReturnResponseWithCode(w, response, http.StatusServiceUnavailable)
		return
	}

	// Check 2: Controller elected
	controllerElected, err := c.ControllerElected(ctx, adm)
	if err != nil {
		c.logger.Error("failed to check controller election", "error", err)
		response.Status = "unhealthy"
		response.ErrorMessage = err.Error()
		_, _ = web.ReturnResponseWithCode(w, response, http.StatusServiceUnavailable)
		return
	}
	response.ControllerElected = controllerElected

	if !controllerElected {
		c.logger.Warn("no controller elected")
		response.Status = "unhealthy"
		response.ErrorMessage = "no controller elected"
		_, _ = web.ReturnResponseWithCode(w, response, http.StatusServiceUnavailable)
		return
	}

	// Check 3: Zero under-replicated partitions
	underReplicated, err := c.UnderReplicatedPartitions(ctx, adm)
	if err != nil {
		c.logger.Error("failed to check under-replicated partitions", "error", err)
		response.Status = "unhealthy"
		response.ErrorMessage = err.Error()
		_, _ = web.ReturnResponseWithCode(w, response, http.StatusServiceUnavailable)
		return
	}
	response.UnderReplicatedPartitions = underReplicated

	if underReplicated > 0 {
		c.logger.Warn("broker has under-replicated partitions",
			"brokerId", c.brokerID,
			"count", underReplicated)
		response.Status = "unhealthy"
		response.ErrorMessage = "broker has under-replicated partitions"
		_, _ = web.ReturnResponseWithCode(w, response, http.StatusServiceUnavailable)
		return
	}

	// Check 4: Log directories healthy
	logDirsHealthy, err := c.LogDirsHealthy(ctx, adm)
	if err != nil {
		c.logger.Error("failed to check log directories", "error", err)
		response.Status = "unhealthy"
		response.ErrorMessage = err.Error()
		_, _ = web.ReturnResponseWithCode(w, response, http.StatusServiceUnavailable)
		return
	}
	response.LogDirsHealthy = logDirsHealthy

	if !logDirsHealthy {
		c.logger.Warn("log directories unhealthy", "brokerId", c.brokerID)
		response.Status = "unhealthy"
		response.ErrorMessage = "log directories unhealthy (future partitions detected)"
		_, _ = web.ReturnResponseWithCode(w, response, http.StatusServiceUnavailable)
		return
	}

	response.Status = "healthy"
	_, _ = web.ReturnResponse(w, response)
}

// CheckReadiness performs a full readiness check and returns the result
func (c *Checker) CheckReadiness(ctx context.Context) CheckResult {
	adm, cleanup, err := c.clientFactory()
	if err != nil {
		return CheckResult{
			Healthy: false,
			Message: err.Error(),
		}
	}
	defer cleanup()

	// Check 1: Broker registered
	brokerRegistered, err := c.BrokerInMetadata(ctx, adm)
	if err != nil {
		return CheckResult{Healthy: false, Message: err.Error()}
	}
	if !brokerRegistered {
		return CheckResult{Healthy: false, Message: "broker not registered in cluster metadata"}
	}

	// Check 2: Controller elected
	controllerElected, err := c.ControllerElected(ctx, adm)
	if err != nil {
		return CheckResult{Healthy: false, Message: err.Error()}
	}
	if !controllerElected {
		return CheckResult{Healthy: false, Message: "no controller elected"}
	}

	// Check 3: No under-replicated partitions
	underReplicated, err := c.UnderReplicatedPartitions(ctx, adm)
	if err != nil {
		return CheckResult{Healthy: false, Message: err.Error()}
	}
	if underReplicated > 0 {
		return CheckResult{Healthy: false, Message: "broker has under-replicated partitions"}
	}

	// Check 4: Log dirs healthy
	logDirsHealthy, err := c.LogDirsHealthy(ctx, adm)
	if err != nil {
		return CheckResult{Healthy: false, Message: err.Error()}
	}
	if !logDirsHealthy {
		return CheckResult{Healthy: false, Message: "log directories unhealthy"}
	}

	return CheckResult{Healthy: true}
}
