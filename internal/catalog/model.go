package catalog

// Product is the canonical product stored in the marketplace catalog.
type Product struct {
	ID       int64
	Name     string
	Brand    *string
	Category *string
}

// SellerProduct represents a seller's listing for a canonical catalog product.
type SellerProduct struct {
	SellerName      string
	ProductID       int64
	SellerProductID string
}

// Entry is one product record received from a seller's catalog file.
type Entry struct {
	ID         string  `json:"Id"`
	SellerName string  `json:"SellerName"`
	Name       string  `json:"Name"`
	Brand      *string `json:"Brand"`
	Category   *string `json:"Category"`
}

// ImportResult summarizes the effects of one atomic import.
type ImportResult struct {
	Received              int
	ProductsCreated       int
	AssociationsCreated   int
	AssociationsUnchanged int
}
