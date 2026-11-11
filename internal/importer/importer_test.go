package importer

import (
	"strings"
	"testing"
)

func TestDecode(t *testing.T) {
	input := `[{"Id":"external-1","SellerName":"Seller","Name":"Product","Brand":null,"Category":"Tech"}]`
	entries, err := Decode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ID != "external-1" {
		t.Fatalf("unexpected entries: %#v", entries)
	}
}

func TestDecodeRejectsMissingRequiredFields(t *testing.T) {
	_, err := Decode(strings.NewReader(`[{"Id":"x","SellerName":"Seller","Name":""}]`))
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestDecodeRejectsInvalidInputs(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"malformed JSON", `[{"Id":`},
		{"missing ID", `[{"SellerName":"Seller","Name":"Product"}]`},
		{"missing seller", `[{"Id":"1","Name":"Product"}]`},
		{"empty seller", `[{"Id":"1","SellerName":" ","Name":"Product"}]`},
		{"empty name", `[{"Id":"1","SellerName":"Seller","Name":" "}]`},
		{"null input", `null`},
		{"multiple JSON values", `[] []`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode(strings.NewReader(tt.input))
			if err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestDecodeAcceptsOptionalNullFields(t *testing.T) {
	input := `[{
        "Id":"external-1",
        "SellerName":"Seller",
        "Name":"Product",
        "Brand":null,
        "Category":null
    }]`

	entries, err := Decode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	if entries[0].Brand != nil || entries[0].Category != nil {
		t.Fatal("expected Brand and Category to be nil")
	}
}
