package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"example.com/catalog-consolidation/internal/catalog"
)

func TestImportIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "catalog.db")
	// Build the minimal fixture in a temporary database.
	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	_, err = r.db.Exec(`CREATE TABLE Product (Id INTEGER PRIMARY KEY AUTOINCREMENT, Name TEXT NOT NULL, Brand TEXT, Category TEXT); CREATE TABLE SellerProduct (Id INTEGER PRIMARY KEY AUTOINCREMENT, SellerName TEXT NOT NULL, ProductId INTEGER NOT NULL REFERENCES Product(Id), SellerProductId INTEGER NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := r.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	brand, category := "Acme", "Tools"
	entries := []catalog.Entry{{ID: "uuid-1", SellerName: "Seller A", Name: "Widget", Brand: &brand, Category: &category}}
	first, err := r.Import(ctx, entries)
	if err != nil {
		t.Fatal(err)
	}
	second, err := r.Import(ctx, entries)
	if err != nil {
		t.Fatal(err)
	}
	if first.ProductsCreated != 1 || first.AssociationsCreated != 1 {
		t.Fatalf("unexpected first result: %+v", first)
	}
	if second.ProductsCreated != 0 || second.AssociationsUnchanged != 1 {
		t.Fatalf("unexpected second result: %+v", second)
	}
}

func TestImportRollsBackOnAssociationError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollback.db")

	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	_, err = r.db.Exec(`
        CREATE TABLE Product (
            Id INTEGER PRIMARY KEY AUTOINCREMENT,
            Name TEXT NOT NULL,
            Brand TEXT,
            Category TEXT
        );

        CREATE TABLE SellerProduct (
            Id INTEGER PRIMARY KEY AUTOINCREMENT,
            SellerName TEXT NOT NULL,
            ProductId INTEGER NOT NULL REFERENCES Product(Id),
            SellerProductId INTEGER NOT NULL
        );
    `)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Primeiro executamos a migração.
	if err := r.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	// O gatilho é criado DEPOIS da migração.
	_, err = r.db.Exec(`
        CREATE TRIGGER force_rollback_test
        BEFORE INSERT ON SellerProduct
        WHEN (SELECT COUNT(*) FROM SellerProduct) >= 5
        BEGIN
            SELECT RAISE(ABORT, 'FORCED_ROLLBACK_TEST');
        END;
    `)
	if err != nil {
		t.Fatal(err)
	}

	// Seis produtos diferentes: a sexta associação deve falhar.
	entries := make([]catalog.Entry, 6)

	for i := range entries {
		entries[i] = catalog.Entry{
			ID:         fmt.Sprintf("uuid-%d", i+1),
			SellerName: "Seller A",
			Name:       fmt.Sprintf("Product %d", i+1),
		}
	}

	_, err = r.Import(ctx, entries)

	if err == nil {
		t.Fatal("expected import to fail, but it succeeded")
	}

	// Todas as alterações precisam ter sido desfeitas.
	var products, associations int

	if err := r.db.QueryRow(
		`SELECT COUNT(*) FROM Product`,
	).Scan(&products); err != nil {
		t.Fatal(err)
	}

	if err := r.db.QueryRow(
		`SELECT COUNT(*) FROM SellerProduct`,
	).Scan(&associations); err != nil {
		t.Fatal(err)
	}

	if products != 0 || associations != 0 {
		t.Fatalf(
			"rollback failed: products=%d, associations=%d",
			products,
			associations,
		)
	}
}

func TestImportFailsWhenSellerReusesExternalIdForDifferentProduct(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reused-external-id.db")

	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	_, err = r.db.Exec(`CREATE TABLE Product (Id INTEGER PRIMARY KEY AUTOINCREMENT, Name TEXT NOT NULL, Brand TEXT, Category TEXT); CREATE TABLE SellerProduct (Id INTEGER PRIMARY KEY AUTOINCREMENT, SellerName TEXT NOT NULL, ProductId INTEGER NOT NULL REFERENCES Product(Id), SellerProductId INTEGER NOT NULL)`)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := r.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	// Same seller, same external SellerProductId, but two entries that
	// resolve to two different canonical products. This is the duplicate
	// scenario the challenge cares about most: a seller must not be able to
	// silently repoint one of their external IDs to a different catalog item.
	entries := []catalog.Entry{
		{ID: "ext-1", SellerName: "Seller A", Name: "Product One"},
		{ID: "ext-1", SellerName: "Seller A", Name: "Product Two"},
	}

	_, err = r.Import(ctx, entries)
	if err == nil {
		t.Fatal("expected import to fail when a seller reuses an external ID for a different product")
	}

	wantMessage := `entry 1: seller "Seller A" reused external product ID "ext-1" for a different catalog product`
	if err.Error() != wantMessage {
		t.Fatalf("unexpected error message: got %q, want %q", err.Error(), wantMessage)
	}

	// The whole import is one transaction: the conflict must roll back the
	// product and association created earlier in the same batch too.
	var products, associations int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM Product`).Scan(&products); err != nil {
		t.Fatal(err)
	}
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM SellerProduct`).Scan(&associations); err != nil {
		t.Fatal(err)
	}
	if products != 0 || associations != 0 {
		t.Fatalf("rollback failed: products=%d, associations=%d", products, associations)
	}
}

func TestForeignKeyPreventsInvalidProductAssociation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "foreign-key.db")

	r, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	// Mantém uma única conexão para que o PRAGMA seja verificado
	// na mesma conexão utilizada pelo INSERT.
	r.db.SetMaxOpenConns(1)

	_, err = r.db.Exec(`
        CREATE TABLE Product (
            Id INTEGER PRIMARY KEY AUTOINCREMENT,
            Name TEXT NOT NULL,
            Brand TEXT,
            Category TEXT
        );

        CREATE TABLE SellerProduct (
            Id INTEGER PRIMARY KEY AUTOINCREMENT,
            SellerName TEXT NOT NULL,
            ProductId INTEGER NOT NULL REFERENCES Product(Id),
            SellerProductId INTEGER NOT NULL
        );
    `)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	if err := r.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	// Verifica se a aplicação habilitou a integridade referencial.
	var foreignKeys int
	if err := r.db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}

	if foreignKeys != 1 {
		t.Fatalf("foreign key enforcement is disabled: PRAGMA foreign_keys = %d", foreignKeys)
	}

	// ProductId 999999 não existe.
	_, err = r.db.Exec(`
        INSERT INTO SellerProduct (
            SellerName,
            ProductId,
            SellerProductId
        )
        VALUES (?, ?, ?)
    `, "TestSeller", 999999, "external-123")

	if err == nil {
		t.Fatal("expected foreign key violation, but insert succeeded")
	}
}
