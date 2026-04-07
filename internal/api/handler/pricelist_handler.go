package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type PriceListHandler struct {
	scanner  *service.PriceListScanner
	scanJobs port.ScanJobRepo
}

func NewPriceListHandler(scanner *service.PriceListScanner, scanJobs port.ScanJobRepo) *PriceListHandler {
	return &PriceListHandler{scanner: scanner, scanJobs: scanJobs}
}

// Upload handles CSV upload and processes through the legacy scanner (synchronous).
func (h *PriceListHandler) Upload(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	if err := r.ParseMultipartForm(50 << 20); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid form data")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	items, err := h.scanner.ParseCSV(file)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid CSV: "+err.Error())
		return
	}

	if len(items) == 0 {
		response.Error(w, http.StatusBadRequest, "no valid items found in CSV")
		return
	}

	_ = ac // tenant context available for future use
	matches, err := h.scanner.ScanPriceList(r.Context(), items, "US")
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "scan failed: "+err.Error())
		return
	}

	var qualified, restricted, lowMargin int
	for _, m := range matches {
		switch m.MatchStatus {
		case "qualified":
			qualified++
		case "restricted":
			restricted++
		case "low_margin":
			lowMargin++
		}
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"file_name":  header.Filename,
		"total":      len(items),
		"matched":    len(matches),
		"qualified":  qualified,
		"restricted": restricted,
		"low_margin": lowMargin,
		"matches":    matches,
	})
}

// UploadWithFunnel handles CSV upload and processes through the tiered funnel.
func (h *PriceListHandler) UploadWithFunnel(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	if err := r.ParseMultipartForm(50 << 20); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid form data")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		response.Error(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	items, err := h.scanner.ParseCSV(file)
	if err != nil {
		response.Error(w, http.StatusBadRequest, "invalid CSV: "+err.Error())
		return
	}

	if len(items) == 0 {
		response.Error(w, http.StatusBadRequest, "no valid items found in CSV")
		return
	}

	thresholds := domain.DefaultPipelineThresholds()
	result, err := h.scanner.ScanWithFunnel(r.Context(), ac.TenantID, items, thresholds, "US")
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "scan failed: "+err.Error())
		return
	}

	response.JSON(w, http.StatusOK, map[string]any{
		"file_name":    header.Filename,
		"scan_job_id":  result.ScanJobID,
		"total_items":  result.TotalItems,
		"matched":      result.Matched,
		"survivors":    len(result.Survivors),
		"funnel_stats": result.Stats,
	})
}

// GetScanJob returns the status/progress of a scan job.
func (h *PriceListHandler) GetScanJob(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())
	jobID := chi.URLParam(r, "id")

	if h.scanJobs == nil {
		response.Error(w, http.StatusNotImplemented, "scan jobs not available")
		return
	}

	job, err := h.scanJobs.GetByID(r.Context(), ac.TenantID, domain.ScanJobID(jobID))
	if err != nil {
		response.Error(w, http.StatusNotFound, "scan job not found")
		return
	}

	response.JSON(w, http.StatusOK, job)
}

// ListScanJobs returns recent scan jobs.
func (h *PriceListHandler) ListScanJobs(w http.ResponseWriter, r *http.Request) {
	ac := middleware.GetAuthContext(r.Context())

	if h.scanJobs == nil {
		response.JSON(w, http.StatusOK, []any{})
		return
	}

	jobs, err := h.scanJobs.List(r.Context(), ac.TenantID, 20)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, "failed to list scan jobs")
		return
	}

	response.JSON(w, http.StatusOK, jobs)
}
