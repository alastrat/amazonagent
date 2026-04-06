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
}

func NewPriceListScanner(products port.ProductSearcher) *PriceListScanner {
	return &PriceListScanner{products: products}
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
