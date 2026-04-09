package service

import "github.com/pluriza/fba-agent-orchestrator/internal/domain"

// AssessmentCategories defines the categories to probe during account assessment.
// Organized by tier:
//   Tier 1: High-volume, commonly open categories (4 probes each)
//   Tier 2: Commonly gated categories (3 probes each)
//   Tier 3: Niche/specialty categories (2 probes each)
var AssessmentCategories = []struct {
	Category string
	Tier     int // 1, 2, or 3
}{
	// Tier 1 — high-volume, usually open (4 probes each = 40)
	{"Home & Kitchen", 1},
	{"Office Products", 1},
	{"Sports & Outdoors", 1},
	{"Tools & Home Improvement", 1},
	{"Patio, Lawn & Garden", 1},
	{"Industrial & Scientific", 1},
	{"Arts, Crafts & Sewing", 1},
	{"Automotive", 1},
	{"Pet Supplies", 1},
	{"Musical Instruments", 1},

	// Tier 2 — commonly gated (3 probes each = 30)
	{"Grocery & Gourmet Food", 2},
	{"Health & Household", 2},
	{"Beauty & Personal Care", 2},
	{"Toys & Games", 2},
	{"Baby", 2},
	{"Clothing, Shoes & Jewelry", 2},
	{"Electronics", 2},
	{"Cell Phones & Accessories", 2},
	{"Appliances", 2},
	{"Video Games", 2},

	// Tier 3 — niche (2 probes each = 20)
	{"Books", 3},
	{"Kitchen & Dining", 3},
	{"Camera & Photo", 3},
	{"Computers & Accessories", 3},
	{"Software", 3},
	{"Garden & Outdoor", 3},
	{"Collectibles & Fine Art", 3},
	{"Handmade Products", 3},
	{"Amazon Renewed", 3},
	{"Amazon Launchpad", 3},
}

// TopWholesaleBrands are the 25 most common wholesale brands to probe (2 ASINs each = 50).
var TopWholesaleBrands = []string{
	"3M", "Rubbermaid", "Energizer", "Scotch-Brite", "Command",
	"Clorox", "Mrs. Meyer's", "Seventh Generation", "Method", "Burt's Bees",
	"Nature's Bounty", "Centrum", "Band-Aid", "Neosporin", "Aveeno",
	"Crayola", "Play-Doh", "LEGO", "Hasbro", "Mattel",
	"Duracell", "AmazonBasics", "Glad", "Reynolds", "Ziploc",
}

// ProbesPerTier returns how many ASINs to check per category at each tier.
func ProbesPerTier(tier int) int {
	switch tier {
	case 1:
		return 4
	case 2:
		return 3
	case 3:
		return 2
	default:
		return 2
	}
}

// BuildAssessmentProbes generates the list of ASINs to check.
// In production, this queries SP-API to find representative ASINs per category.
// The caller provides a function that finds ASINs for a given category + brand tier.
type ASINFinder func(category string, brandTier string, limit int) []domain.AssessmentProbe

// GenerateProbeList builds the full assessment probe list.
// If finder is nil, returns an empty list (caller must provide real ASINs).
func GenerateProbeList(finder ASINFinder) []domain.AssessmentProbe {
	if finder == nil {
		return nil
	}

	var probes []domain.AssessmentProbe

	// Category probes: stratified by brand tier
	for _, cat := range AssessmentCategories {
		count := ProbesPerTier(cat.Tier)

		// Split probes across brand tiers
		topCount := count / 2
		if topCount == 0 {
			topCount = 1
		}
		midCount := count - topCount
		genericCount := 1
		if count >= 4 {
			genericCount = 1
			midCount = count - topCount - genericCount
		}

		probes = append(probes, finder(cat.Category, "top", topCount)...)
		probes = append(probes, finder(cat.Category, "mid", midCount)...)
		if count >= 3 {
			probes = append(probes, finder(cat.Category, "generic", genericCount)...)
		}
	}

	// Brand probes: top 25 brands × 2 ASINs
	for _, brand := range TopWholesaleBrands {
		probes = append(probes, finder(brand, "brand_probe", 2)...)
	}

	// Calibration probes: 10 known-open ASINs
	probes = append(probes, finder("calibration", "calibration", 10)...)

	return probes
}

// SPAPIASINFinder creates a finder that uses SP-API catalog search to discover ASINs.
// This is the production implementation — searches by category keyword to find real products.
func SPAPIASINFinder(searcher interface {
	SearchProducts(ctx interface{}, keywords []string, marketplace string) ([]interface{}, error)
}) ASINFinder {
	// This will be implemented when we integrate with the actual SP-API
	// For now, return nil to indicate dynamic discovery is needed
	return nil
}

// StaticProbeSet returns a hardcoded set of calibration ASINs that are known to be
// universally accessible. These serve as the baseline for assessment accuracy.
func StaticCalibrationProbes() []domain.AssessmentProbe {
	return []domain.AssessmentProbe{
		// Office supplies — commonly open
		{ASIN: "B00006IE70", Category: "Office Products", Brand: "Scotch", Tier: "calibration", ExpectedGating: "open"},
		{ASIN: "B0000AQODM", Category: "Office Products", Brand: "Post-it", Tier: "calibration", ExpectedGating: "open"},
		// Home & Kitchen — commonly open
		{ASIN: "B002YK46UQ", Category: "Home & Kitchen", Brand: "Rubbermaid", Tier: "mid", ExpectedGating: "open"},
		{ASIN: "B00004OCKR", Category: "Home & Kitchen", Brand: "Rubbermaid", Tier: "mid", ExpectedGating: "open"},
		// Tools
		{ASIN: "B00004SBDR", Category: "Tools & Home Improvement", Brand: "3M", Tier: "top", ExpectedGating: "open"},
		// Sports
		{ASIN: "B014US3FQI", Category: "Sports & Outdoors", Brand: "", Tier: "generic", ExpectedGating: "open"},
		// Beauty — commonly gated
		{ASIN: "B004Y9GZDO", Category: "Beauty & Personal Care", Brand: "CeraVe", Tier: "top", ExpectedGating: "brand_gated"},
		// Grocery — commonly gated
		{ASIN: "B00CQ7ELOM", Category: "Grocery & Gourmet Food", Brand: "KIND", Tier: "top", ExpectedGating: "brand_gated"},
		// Electronics
		{ASIN: "B09V3KXJPB", Category: "Electronics", Brand: "Anker", Tier: "mid", ExpectedGating: "open"},
		// Pet
		{ASIN: "B000084E6V", Category: "Pet Supplies", Brand: "KONG", Tier: "mid", ExpectedGating: "open"},
	}
}
