package handler

import (
	"net/http"

	"github.com/pluriza/fba-agent-orchestrator/internal/api/middleware"
	"github.com/pluriza/fba-agent-orchestrator/internal/api/response"
	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

type PriceListHandler struct {
	scanner *service.PriceListScanner
}

func NewPriceListHandler(scanner *service.PriceListScanner) *PriceListHandler {
	return &PriceListHandler{scanner: scanner}
}

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
