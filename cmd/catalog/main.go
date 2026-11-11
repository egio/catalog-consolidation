package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"example.com/catalog-consolidation/internal/importer"
	"example.com/catalog-consolidation/internal/repository/sqlite"
)

func main() {
	databasePath := flag.String("db", "catalog.db", "path to the SQLite catalog database")
	inputPath := flag.String("input", "ProductEntry.json", "path to the seller products JSON file")
	flag.Parse()

	input, err := os.Open(*inputPath)
	if err != nil {
		log.Fatalf("open input file: %v", err)
	}
	defer input.Close()
	entries, err := importer.Decode(input)
	if err != nil {
		log.Fatalf("read input file: %v", err)
	}

	repository, err := sqlite.Open(*databasePath)
	if err != nil {
		log.Fatal(err)
	}
	defer repository.Close()
	ctx := context.Background()
	if err := repository.Migrate(ctx); err != nil {
		log.Fatalf("database migration: %v", err)
	}
	result, err := repository.Import(ctx, entries)
	if err != nil {
		log.Fatalf("catalog import failed: %v", err)
	}
	fmt.Printf("Import completed successfully\nReceived: %d\nNew products: %d\nNew seller associations: %d\nExisting associations: %d\n", result.Received, result.ProductsCreated, result.AssociationsCreated, result.AssociationsUnchanged)
}
