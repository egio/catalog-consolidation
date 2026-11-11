package matching

import (
	"fmt"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
	"unicode"
)

// Key identifies a product using the three descriptive fields available in
// both the existing database and the incoming file.
type Key struct {
	Name     string
	Brand    string
	Category string
}

// Normalize applies conservative, deterministic text normalization. It does
// not translate terms or infer semantic equivalence.
func Normalize(value string) string {
	value = norm.NFD.String(value)
	var b strings.Builder
	for _, r := range value {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(cases.Fold().String(b.String())), " ")
}

func NewKey(name, brand, category string) Key {
	return Key{Normalize(name), Normalize(brand), Normalize(category)}
}

// Index maps normalized product attributes to their canonical database ID.
// Ambiguous keys are rejected instead of choosing an arbitrary product.
type Index struct {
	products map[Key]int64
}

func NewIndex() *Index { return &Index{products: make(map[Key]int64)} }

func (i *Index) Add(key Key, productID int64) error {
	if existingID, ok := i.products[key]; ok && existingID != productID {
		return fmt.Errorf("ambiguous normalized product key: maps to product IDs %d and %d", existingID, productID)
	}
	i.products[key] = productID
	return nil
}

func (i *Index) Find(key Key) (int64, bool) {
	id, ok := i.products[key]
	return id, ok
}
