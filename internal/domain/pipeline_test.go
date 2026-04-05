package domain

import "testing"

func TestBrandFilter_AllowAll(t *testing.T) {
	f := BrandFilter{}
	if !f.IsBrandAllowed("AnyBrand") {
		t.Error("empty filter should allow all brands")
	}
}

func TestBrandFilter_BlockList(t *testing.T) {
	f := BrandFilter{
		BlockList: []string{"Hydro Flask", "Owala", "Yeti"},
	}
	if f.IsBrandAllowed("Hydro Flask") {
		t.Error("should block Hydro Flask")
	}
	if f.IsBrandAllowed("hydro flask") {
		t.Error("should block case-insensitive")
	}
	if !f.IsBrandAllowed("Generic Brand") {
		t.Error("should allow non-blocked brand")
	}
}

func TestBrandFilter_AllowList(t *testing.T) {
	f := BrandFilter{
		AllowList: []string{"Lodge", "Cuisinart", "KitchenAid"},
	}
	if !f.IsBrandAllowed("Lodge") {
		t.Error("should allow Lodge")
	}
	if !f.IsBrandAllowed("lodge") {
		t.Error("should allow case-insensitive")
	}
	if f.IsBrandAllowed("Hydro Flask") {
		t.Error("should reject brand not in allow list")
	}
}

func TestBrandFilter_EmptyBrand(t *testing.T) {
	f := BrandFilter{BlockList: []string{"Hydro Flask"}}
	if !f.IsBrandAllowed("") {
		t.Error("empty brand should pass (can't filter)")
	}
}
