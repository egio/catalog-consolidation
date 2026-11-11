# Catalog Consolidation

A Go CLI that consolidates seller product catalogs into the supplied SQLite catalog.

## Requirements
- Go 1.23+
- CGO enabled and a C compiler available (required by `github.com/mattn/go-sqlite3`)
- The supplied `catalog.db`
- The supplied `ProductEntry.json`

## Run

```bash
go mod tidy
go test ./...
go run ./cmd/catalog -db ./testdata/catalog.db -input ./testdata/ProductEntry.json
```

**Important:** the `testdata/catalog.db` file is a working copy of the supplied database. Keep the original `catalog.db` untouched. The application runs a migration that changes `SellerProduct.SellerProductId` from INTEGER to TEXT to support the UUID strings in the input file.

## Matching behavior
Products are matched by normalized `(Name, Brand, Category)`. Normalization ignores case, accents, punctuation differences, and repeated whitespace. It does not translate names or perform fuzzy matching. See [docs/decisions.md](docs/decisions.md).

## Atomicity and idempotency
A whole input file is imported in one SQLite transaction. Re-running the same file does not create duplicate seller associations. SQL values are always parameterized.

## CLI output
The command reports the number of received entries, newly created products, newly created seller associations, and unchanged associations.
