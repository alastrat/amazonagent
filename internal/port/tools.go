package port

import "context"

// ProductSearchResult from Amazon SP-API
type ProductSearchResult struct {
	ASIN        string  `json:"asin"`
	Title       string  `json:"title"`
	Brand       string  `json:"brand"`
	Category    string  `json:"category"`
	AmazonPrice float64 `json:"amazon_price"`
	BSRRank     int     `json:"bsr_rank"`
	BSRCategory string  `json:"bsr_category"`
	SellerCount int     `json:"seller_count"`
	ReviewCount int     `json:"review_count"`
	Rating      float64 `json:"rating"`
	ImageURL    string  `json:"image_url"`
	IsGated     bool    `json:"is_gated"`
	IsFBA       bool    `json:"is_fba"`
}

// ProductFeeEstimate from Amazon SP-API
type ProductFeeEstimate struct {
	ASIN        string  `json:"asin"`
	ReferralFee float64 `json:"referral_fee"`
	FBAFee      float64 `json:"fba_fulfillment_fee"`
	ClosingFee  float64 `json:"closing_fee"`
	TotalFees   float64 `json:"total_fees"`
}

// WebSearchResult from Exa or similar
type WebSearchResult struct {
	Title   string  `json:"title"`
	URL     string  `json:"url"`
	Snippet string  `json:"snippet"`
	Score   float64 `json:"score"`
}

// ScrapedPage from Firecrawl or similar
type ScrapedPage struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// ListingRestriction describes why a product can't be listed
type ListingRestriction struct {
	ASIN    string `json:"asin"`
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// ProductSearcher searches Amazon for products
type ProductSearcher interface {
	SearchProducts(ctx context.Context, keywords []string, marketplace string) ([]ProductSearchResult, error)
	SearchByBrowseNode(ctx context.Context, nodeID string, marketplace string, pageToken string) ([]ProductSearchResult, string, error)
	GetProductDetails(ctx context.Context, asins []string, marketplace string) ([]ProductSearchResult, error)
	EstimateFees(ctx context.Context, asin string, price float64, marketplace string) (*ProductFeeEstimate, error)
	CheckListingEligibility(ctx context.Context, asins []string, marketplace string) ([]ListingRestriction, error)
	LookupByIdentifier(ctx context.Context, identifiers []string, idType string, marketplace string) ([]ProductSearchResult, error)
}

// WebSearcher searches the web for information
type WebSearcher interface {
	Search(ctx context.Context, query string, numResults int) ([]WebSearchResult, error)
}

// WebScraper scrapes web pages
type WebScraper interface {
	Scrape(ctx context.Context, url string) (*ScrapedPage, error)
	ScrapeMultiple(ctx context.Context, urls []string) ([]ScrapedPage, error)
}
