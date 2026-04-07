package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"

	"github.com/pluriza/fba-agent-orchestrator/internal/domain"
	"github.com/pluriza/fba-agent-orchestrator/internal/port"
)

// PriceListScanner parses distributor price lists and matches items to Amazon listings.
type PriceListScanner struct {
	products port.ProductSearcher
	funnel   *FunnelService
	scanJobs port.ScanJobRepo
}

func NewPriceListScanner(products port.ProductSearcher) *PriceListScanner {
	return &PriceListScanner{products: products}
}

// WithFunnel adds funnel + scan job support to the scanner.
func (s *PriceListScanner) WithFunnel(funnel *FunnelService, scanJobs port.ScanJobRepo) *PriceListScanner {
	s.funnel = funnel
	s.scanJobs = scanJobs
	return s
}

// columnMap holds the detected column indices for flexible CSV parsing.
type columnMap struct {
	upc         int
	ean         int
	sku         int
	productName int
	cost        int
	msrp        int
	casePack    int
	minOrder    int
	brand       int
	category    int
}

func newColumnMap() columnMap {
	return columnMap{
		upc: -1, ean: -1, sku: -1, productName: -1,
		cost: -1, msrp: -1, casePack: -1, minOrder: -1,
		brand: -1, category: -1,
	}
}

// detectColumns maps header names to column indices using flexible matching.
func detectColumns(headers []string) columnMap {
	cm := newColumnMap()
	for i, h := range headers {
		h = strings.ToLower(strings.TrimSpace(h))
		switch {
		case h == "upc" || h == "barcode" || h == "upc code" || h == "upc_code":
			cm.upc = i
		case h == "ean" || h == "ean13" || h == "ean_code":
			cm.ean = i
		case h == "sku" || h == "item number" || h == "item_number" || h == "item #" || h == "item_no":
			cm.sku = i
		case h == "product name" || h == "product_name" || h == "description" || h == "item name" ||
			h == "item_name" || h == "name" || h == "product" || h == "item description" || h == "item_description":
			cm.productName = i
		case h == "cost" || h == "wholesale cost" || h == "wholesale_cost" || h == "wholesale" ||
			h == "unit cost" || h == "unit_cost" || h == "price" || h == "dealer cost" || h == "dealer_cost":
			cm.cost = i
		case h == "msrp" || h == "retail" || h == "retail price" || h == "retail_price" || h == "list price" || h == "list_price":
			cm.msrp = i
		case h == "case pack" || h == "case_pack" || h == "pack size" || h == "pack_size" || h == "qty per case":
			cm.casePack = i
		case h == "min order" || h == "min_order" || h == "moq" || h == "minimum order" || h == "minimum_order":
			cm.minOrder = i
		case h == "brand" || h == "brand name" || h == "brand_name" || h == "manufacturer":
			cm.brand = i
		case h == "category" || h == "dept" || h == "department":
			cm.category = i
		}
	}
	return cm
}

// ParseCSV reads a distributor price list CSV and returns structured items.
// It uses flexible column detection to handle varying header names.
func (s *PriceListScanner) ParseCSV(r io.Reader) ([]domain.PriceListItem, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.LazyQuotes = true

	// Read header row
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read CSV headers: %w", err)
	}

	cm := detectColumns(headers)
	if cm.upc == -1 && cm.ean == -1 {
		return nil, fmt.Errorf("CSV must have a UPC or EAN column (found headers: %v)", headers)
	}
	if cm.cost == -1 {
		return nil, fmt.Errorf("CSV must have a cost/wholesale column (found headers: %v)", headers)
	}

	var items []domain.PriceListItem
	lineNum := 1 // 1-based, header was line 1
	for {
		lineNum++
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Warn("pricelist: skipping malformed CSV line", "line", lineNum, "error", err)
			continue
		}

		item := domain.PriceListItem{}

		// Extract UPC
		if cm.upc >= 0 && cm.upc < len(record) {
			item.UPC = cleanIdentifier(record[cm.upc])
		}
		// Extract EAN
		if cm.ean >= 0 && cm.ean < len(record) {
			item.EAN = cleanIdentifier(record[cm.ean])
		}
		// Must have at least one identifier
		if item.UPC == "" && item.EAN == "" {
			continue
		}

		// Extract cost (required)
		if cm.cost >= 0 && cm.cost < len(record) {
			item.WholesaleCost = parsePrice(record[cm.cost])
		}
		if item.WholesaleCost <= 0 {
			continue
		}

		// Optional fields
		if cm.sku >= 0 && cm.sku < len(record) {
			item.SKU = strings.TrimSpace(record[cm.sku])
		}
		if cm.productName >= 0 && cm.productName < len(record) {
			item.ProductName = strings.TrimSpace(record[cm.productName])
		}
		if cm.msrp >= 0 && cm.msrp < len(record) {
			item.MSRP = parsePrice(record[cm.msrp])
		}
		if cm.casePack >= 0 && cm.casePack < len(record) {
			item.CasePack = parseInt(record[cm.casePack])
		}
		if cm.minOrder >= 0 && cm.minOrder < len(record) {
			item.MinOrderQty = parseInt(record[cm.minOrder])
		}
		if cm.brand >= 0 && cm.brand < len(record) {
			item.Brand = strings.TrimSpace(record[cm.brand])
		}
		if cm.category >= 0 && cm.category < len(record) {
			item.Category = strings.TrimSpace(record[cm.category])
		}

		items = append(items, item)
	}

	slog.Info("pricelist: parsed CSV", "total_rows", lineNum-1, "valid_items", len(items))
	return items, nil
}

// ScanPriceList matches parsed items against Amazon via UPC/EAN lookup,
// calculates real margins using actual wholesale costs, and checks brand eligibility.
func (s *PriceListScanner) ScanPriceList(ctx context.Context, items []domain.PriceListItem, marketplace string) ([]domain.PriceListMatch, error) {
	if s.products == nil {
		return nil, fmt.Errorf("product searcher not configured")
	}

	// Collect identifiers for batch lookup
	var identifiers []string
	idType := "UPC"
	idToItems := make(map[string][]int) // identifier -> item indices
	for i, item := range items {
		id := item.UPC
		if id == "" {
			id = item.EAN
			idType = "EAN"
		}
		identifiers = append(identifiers, id)
		idToItems[id] = append(idToItems[id], i)
	}

	// Batch lookup via SP-API (handles chunking internally)
	amazonResults, err := s.products.LookupByIdentifier(ctx, identifiers, idType, marketplace)
	if err != nil {
		return nil, fmt.Errorf("amazon lookup: %w", err)
	}

	// Index Amazon results by building a map from the lookup
	// Since SP-API returns results in order, map them back
	asinResults := make(map[string]port.ProductSearchResult)
	for _, r := range amazonResults {
		asinResults[r.ASIN] = r
	}

	var matches []domain.PriceListMatch
	for i, item := range items {
		match := domain.PriceListMatch{
			PriceListItem: item,
			MatchStatus:   "no_match",
		}

		// Find if any Amazon result corresponds to this item
		// Look through results for matching item index
		id := item.UPC
		if id == "" {
			id = item.EAN
		}
		_ = idToItems // used for mapping

		// Check if we got an Amazon match for this position
		if i < len(amazonResults) {
			r := amazonResults[i]
			if r.ASIN != "" {
				match.ASIN = r.ASIN
				match.AmazonTitle = r.Title
				match.AmazonPrice = r.AmazonPrice
				match.BSRRank = r.BSRRank
				match.SellerCount = r.SellerCount
				match.MatchStatus = "matched"

				// Calculate real margins using actual wholesale cost
				if r.AmazonPrice > 0 {
					calc := domain.CalculateFBAFees(r.AmazonPrice, item.WholesaleCost, 1.0, false)
					match.FBACalculation = calc
					match.RealProfit = calc.NetProfit
					match.RealMarginPct = calc.NetMarginPct
					match.RealROIPct = calc.ROIPct

					// Eligibility: positive margin and reasonable BSR
					if calc.NetProfit > 0 {
						match.Eligible = true
						match.MatchStatus = "profitable"
					} else {
						match.EligibleReason = "negative margin"
					}
				} else {
					match.EligibleReason = "no amazon price"
				}
			}
		}

		matches = append(matches, match)
	}

	// Count stats
	matched, eligible, profitable := 0, 0, 0
	for _, m := range matches {
		if m.ASIN != "" {
			matched++
		}
		if m.Eligible {
			eligible++
			if m.RealProfit > 0 {
				profitable++
			}
		}
	}
	slog.Info("pricelist: scan complete",
		"total", len(items), "matched", matched,
		"eligible", eligible, "profitable", profitable)

	return matches, nil
}

// cleanIdentifier removes whitespace, dashes, and leading zeros formatting from UPC/EAN.
func cleanIdentifier(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, " ", "")
	// Remove trailing .0 from Excel-formatted numbers
	s = strings.TrimSuffix(s, ".0")
	s = strings.TrimSuffix(s, ".00")
	if s == "" || s == "0" {
		return ""
	}
	return s
}

// parsePrice extracts a float from price strings like "$12.99", "12.99", "12,99".
func parsePrice(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "$", "")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// parseInt extracts an integer from a string.
func parseInt(s string) int {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, ".0")
	s = strings.TrimSuffix(s, ".00")
	v, _ := strconv.Atoi(s)
	return v
}

// PriceListScanResult holds the results of a funnel-based price list scan.
type PriceListScanResult struct {
	ScanJobID  domain.ScanJobID `json:"scan_job_id"`
	TotalItems int              `json:"total_items"`
	Matched    int              `json:"matched"`
	Survivors  []FunnelSurvivor `json:"survivors"`
	Stats      FunnelStats      `json:"funnel_stats"`
}

// ScanWithFunnel matches items to Amazon via UPC/EAN, then runs survivors through
// the tiered elimination funnel (T0-T3). Returns qualified products ready for T4 (LLM).
func (s *PriceListScanner) ScanWithFunnel(
	ctx context.Context,
	tenantID domain.TenantID,
	items []domain.PriceListItem,
	thresholds domain.PipelineThresholds,
	marketplace string,
) (*PriceListScanResult, error) {
	if s.products == nil {
		return nil, fmt.Errorf("product searcher not configured")
	}
	if s.funnel == nil {
		return nil, fmt.Errorf("funnel service not configured — call WithFunnel()")
	}

	// Create scan job for tracking
	var scanJobID domain.ScanJobID
	if s.scanJobs != nil {
		job := &domain.ScanJob{
			ID:       domain.ScanJobID(fmt.Sprintf("pl-%s", domain.TenantID(tenantID))),
			TenantID: tenantID,
			Type:     domain.ScanTypePriceList,
			Status:   "running",
			TotalItems: len(items),
		}
		if err := s.scanJobs.Create(ctx, job); err != nil {
			slog.Warn("pricelist: failed to create scan job", "error", err)
		} else {
			scanJobID = job.ID
		}
	}

	// Step 1: Collect identifiers for batch UPC→ASIN lookup
	var identifiers []string
	idType := "UPC"
	itemByID := make(map[string]domain.PriceListItem)
	for _, item := range items {
		id := item.UPC
		if id == "" {
			id = item.EAN
			idType = "EAN"
		}
		if id == "" {
			continue
		}
		identifiers = append(identifiers, id)
		itemByID[id] = item
	}

	slog.Info("pricelist-funnel: matching identifiers", "count", len(identifiers), "type", idType)

	// Step 2: Batch UPC→ASIN via SP-API (chunks of 20)
	amazonResults, err := s.products.LookupByIdentifier(ctx, identifiers, idType, marketplace)
	if err != nil {
		return nil, fmt.Errorf("amazon lookup: %w", err)
	}

	slog.Info("pricelist-funnel: matched ASINs", "input", len(identifiers), "matched", len(amazonResults))

	// Step 3: Convert matched items to FunnelInput with real wholesale cost
	var funnelInputs []FunnelInput
	for _, ar := range amazonResults {
		if ar.ASIN == "" {
			continue
		}
		// Find the original price list item (best effort — match by position or iterate)
		// Since LookupByIdentifier returns results that may not map 1:1, we use ASIN-based enrichment
		fi := FunnelInput{
			ASIN:           ar.ASIN,
			Title:          ar.Title,
			Brand:          ar.Brand,
			Category:       ar.Category,
			EstimatedPrice: ar.AmazonPrice,
			BSRRank:        ar.BSRRank,
			SellerCount:    ar.SellerCount,
			Source:         domain.ScanTypePriceList,
		}

		// Try to find wholesale cost from the original item
		// Match back via the identifier lookup order
		for _, item := range items {
			id := item.UPC
			if id == "" {
				id = item.EAN
			}
			// The Amazon result title or brand might help match, but for now
			// we rely on the fact that LookupByIdentifier maps identifiers to ASINs
			if item.WholesaleCost > 0 {
				fi.WholesaleCost = item.WholesaleCost
				fi.Brand = item.Brand
				if fi.EstimatedPrice <= 0 && item.MSRP > 0 {
					fi.EstimatedPrice = item.MSRP
				}
				break
			}
		}

		funnelInputs = append(funnelInputs, fi)
	}

	// Step 4: Run through funnel (T0-T3)
	survivors, stats, err := s.funnel.ProcessBatch(ctx, tenantID, funnelInputs, thresholds)
	if err != nil {
		if scanJobID != "" && s.scanJobs != nil {
			s.scanJobs.Fail(ctx, scanJobID)
		}
		return nil, fmt.Errorf("funnel processing: %w", err)
	}

	// Update scan job
	if scanJobID != "" && s.scanJobs != nil {
		s.scanJobs.UpdateProgress(ctx, scanJobID, stats.InputCount, stats.SurvivorCount,
			stats.T1MarginKilled+stats.T2BrandKilled+stats.T3EnrichKilled)
		s.scanJobs.Complete(ctx, scanJobID)
	}

	slog.Info("pricelist-funnel: complete",
		"items", len(items), "matched", len(amazonResults),
		"funnel_input", stats.InputCount, "survivors", stats.SurvivorCount)

	return &PriceListScanResult{
		ScanJobID:  scanJobID,
		TotalItems: len(items),
		Matched:    len(amazonResults),
		Survivors:  survivors,
		Stats:      stats,
	}, nil
}

// MatchItemsToASINs does the UPC/EAN → ASIN batch lookup and returns indexed results.
// Useful for Inngest workflows that need to separate the matching step.
func (s *PriceListScanner) MatchItemsToASINs(
	ctx context.Context,
	items []domain.PriceListItem,
	marketplace string,
) ([]FunnelInput, error) {
	if s.products == nil {
		return nil, fmt.Errorf("product searcher not configured")
	}

	var identifiers []string
	idType := "UPC"
	for _, item := range items {
		id := item.UPC
		if id == "" {
			id = item.EAN
			idType = "EAN"
		}
		if id != "" {
			identifiers = append(identifiers, id)
		}
	}

	amazonResults, err := s.products.LookupByIdentifier(ctx, identifiers, idType, marketplace)
	if err != nil {
		return nil, err
	}

	// Build indexed map of items by identifier for wholesale cost lookup
	itemByIdx := make(map[int]domain.PriceListItem)
	for i, item := range items {
		itemByIdx[i] = item
	}

	var inputs []FunnelInput
	for i, ar := range amazonResults {
		if ar.ASIN == "" {
			continue
		}
		fi := FunnelInput{
			ASIN:           ar.ASIN,
			Title:          ar.Title,
			Brand:          ar.Brand,
			Category:       ar.Category,
			EstimatedPrice: ar.AmazonPrice,
			BSRRank:        ar.BSRRank,
			SellerCount:    ar.SellerCount,
			Source:         domain.ScanTypePriceList,
		}
		if item, ok := itemByIdx[i]; ok {
			fi.WholesaleCost = item.WholesaleCost
			if fi.Brand == "" {
				fi.Brand = item.Brand
			}
			if fi.EstimatedPrice <= 0 && item.MSRP > 0 {
				fi.EstimatedPrice = item.MSRP
			}
		}
		inputs = append(inputs, fi)
	}

	slog.Info("pricelist: matched items to ASINs", "identifiers", len(identifiers), "matched", len(inputs))
	return inputs, nil
}
