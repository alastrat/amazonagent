package service_test

import (
	"strings"
	"testing"

	"github.com/pluriza/fba-agent-orchestrator/internal/service"
)

func TestPriceListScanner_ParseCSV(t *testing.T) {
	csv := `UPC,Product Name,Wholesale Cost,Brand,MSRP
012345678901,Kitchen Knife Set,12.99,ChefPro,24.99
,,invalid row,,
023456789012,Silicone Mat Set,8.50,BakeRight,16.99
,Missing UPC,5.00,NoBrand,10.00
034567890123,Cast Iron Skillet,15.00,IronForge,29.99
`

	scanner := service.NewPriceListScanner(nil)
	items, err := scanner.ParseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Row 1: valid, Row 2: empty UPC+cost, Row 3: valid, Row 4: no UPC, Row 5: valid
	// Expect 3 valid items (rows 1, 3, 5) — row 2 has empty UPC and zero cost, row 4 has no UPC
	if len(items) != 3 {
		t.Fatalf("expected 3 valid items, got %d", len(items))
	}

	if items[0].UPC != "012345678901" {
		t.Errorf("expected UPC 012345678901, got %s", items[0].UPC)
	}
	if items[0].ProductName != "Kitchen Knife Set" {
		t.Errorf("expected 'Kitchen Knife Set', got %s", items[0].ProductName)
	}
	if items[0].WholesaleCost != 12.99 {
		t.Errorf("expected cost 12.99, got %f", items[0].WholesaleCost)
	}
	if items[0].Brand != "ChefPro" {
		t.Errorf("expected brand ChefPro, got %s", items[0].Brand)
	}
	if items[0].MSRP != 24.99 {
		t.Errorf("expected MSRP 24.99, got %f", items[0].MSRP)
	}

	if items[1].UPC != "023456789012" {
		t.Errorf("expected UPC 023456789012, got %s", items[1].UPC)
	}
	if items[1].WholesaleCost != 8.50 {
		t.Errorf("expected cost 8.50, got %f", items[1].WholesaleCost)
	}

	if items[2].UPC != "034567890123" {
		t.Errorf("expected UPC 034567890123, got %s", items[2].UPC)
	}
}

func TestPriceListScanner_ParseCSV_FlexibleHeaders(t *testing.T) {
	// Use alternative header names that distributors commonly use
	csv := `Barcode,Description,Unit Cost,Manufacturer,Retail Price,Case Pack
012345678901,Premium Knife Set,$12.99,ChefPro,$24.99,6
023456789012,Silicone Baking Mat,$8.50,BakeRight,$16.99,12
`

	scanner := service.NewPriceListScanner(nil)
	items, err := scanner.ParseCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// "Barcode" should map to UPC
	if items[0].UPC != "012345678901" {
		t.Errorf("expected UPC from 'Barcode' column, got %s", items[0].UPC)
	}

	// "Description" should map to ProductName
	if items[0].ProductName != "Premium Knife Set" {
		t.Errorf("expected product name from 'Description' column, got %s", items[0].ProductName)
	}

	// "Unit Cost" with $ should parse correctly
	if items[0].WholesaleCost != 12.99 {
		t.Errorf("expected cost 12.99 from '$12.99', got %f", items[0].WholesaleCost)
	}

	// "Manufacturer" should map to Brand
	if items[0].Brand != "ChefPro" {
		t.Errorf("expected brand from 'Manufacturer' column, got %s", items[0].Brand)
	}

	// "Retail Price" should map to MSRP
	if items[0].MSRP != 24.99 {
		t.Errorf("expected MSRP from 'Retail Price' column, got %f", items[0].MSRP)
	}

	// "Case Pack" should map to CasePack
	if items[0].CasePack != 6 {
		t.Errorf("expected case pack 6, got %d", items[0].CasePack)
	}

	if items[1].CasePack != 12 {
		t.Errorf("expected case pack 12, got %d", items[1].CasePack)
	}
}
