package importer

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"example.com/catalog-consolidation/internal/catalog"
)

func Decode(r io.Reader) ([]catalog.Entry, error) {
	decoder := json.NewDecoder(r)
	var entries []catalog.Entry
	if err := decoder.Decode(&entries); err != nil {
		return nil, fmt.Errorf("decode input JSON: %w", err)
	}
	if entries == nil {
		return nil, fmt.Errorf("input must be a JSON array")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("input must contain a single JSON value")
		}
		return nil, fmt.Errorf("read trailing JSON data: %w", err)
	}
	for index, entry := range entries {
		if strings.TrimSpace(entry.ID) == "" {
			return nil, fmt.Errorf("entry %d: Id is required", index)
		}
		if strings.TrimSpace(entry.SellerName) == "" {
			return nil, fmt.Errorf("entry %d: SellerName is required", index)
		}
		if strings.TrimSpace(entry.Name) == "" {
			return nil, fmt.Errorf("entry %d: Name is required", index)
		}
	}
	return entries, nil
}
