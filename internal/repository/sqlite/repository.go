package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"example.com/catalog-consolidation/internal/catalog"
	"example.com/catalog-consolidation/internal/matching"
	_ "github.com/mattn/go-sqlite3"
)

type Repository struct{ db *sql.DB }

func Open(path string) (*Repository, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	// SQLite is embedded and this assessment is a single-process importer.
	// A single connection also makes PRAGMA foreign_keys deterministic.
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping SQLite database: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	return &Repository{db: db}, nil
}

func (r *Repository) Close() error { return r.db.Close() }

// Migrate changes SellerProductId from INTEGER to TEXT because incoming seller
// identifiers are UUID strings. Existing values are preserved using CAST.
func (r *Repository) Migrate(ctx context.Context) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `PRAGMA table_info(SellerProduct)`)
	if err != nil {
		return fmt.Errorf("inspect SellerProduct schema: %w", err)
	}
	var sellerProductIDType string
	for rows.Next() {
		var cid, notnull, pk int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notnull, &defaultValue, &pk); err != nil {
			rows.Close()
			return err
		}
		if name == "SellerProductId" {
			sellerProductIDType = dataType
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if sellerProductIDType == "" {
		return fmt.Errorf("SellerProduct.SellerProductId column not found")
	}
	if sellerProductIDType != "TEXT" {
		statements := []string{
			`CREATE TABLE SellerProduct_new (Id INTEGER PRIMARY KEY AUTOINCREMENT, SellerName TEXT NOT NULL, ProductId INTEGER NOT NULL REFERENCES Product(Id), SellerProductId TEXT NOT NULL)`,
			`INSERT INTO SellerProduct_new (Id, SellerName, ProductId, SellerProductId) SELECT Id, SellerName, ProductId, CAST(SellerProductId AS TEXT) FROM SellerProduct`,
			`DROP TABLE SellerProduct`,
			`ALTER TABLE SellerProduct_new RENAME TO SellerProduct`,
		}
		for _, statement := range statements {
			if _, err := tx.ExecContext(ctx, statement); err != nil {
				return fmt.Errorf("migrate SellerProduct: %w", err)
			}
		}
	}
	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS ux_seller_external_product ON SellerProduct(SellerName, SellerProductId)`); err != nil {
		return fmt.Errorf("create seller product uniqueness index: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS ix_seller_product_product_id ON SellerProduct(ProductId)`); err != nil {
		return fmt.Errorf("create product association index: %w", err)
	}
	return tx.Commit()
}

func (r *Repository) Import(ctx context.Context, entries []catalog.Entry) (catalog.ImportResult, error) {
	result := catalog.ImportResult{Received: len(entries)}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("begin import transaction: %w", err)
	}
	defer tx.Rollback()

	index := matching.NewIndex()
	rows, err := tx.QueryContext(ctx, `SELECT Id, Name, Brand, Category FROM Product`)
	if err != nil {
		return result, fmt.Errorf("load catalog products: %w", err)
	}
	for rows.Next() {
		var id int64
		var name string
		var brand, category sql.NullString
		if err := rows.Scan(&id, &name, &brand, &category); err != nil {
			rows.Close()
			return result, err
		}
		if err := index.Add(matching.NewKey(name, nullableString(brand), nullableString(category)), id); err != nil {
			rows.Close()
			return result, fmt.Errorf("existing catalog contains ambiguous products: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()

	insertProduct, err := tx.PrepareContext(ctx, `INSERT INTO Product(Name, Brand, Category) VALUES (?, ?, ?)`)
	if err != nil {
		return result, err
	}
	defer insertProduct.Close()
	insertAssociation, err := tx.PrepareContext(ctx, `INSERT INTO SellerProduct(SellerName, ProductId, SellerProductId) VALUES (?, ?, ?) ON CONFLICT(SellerName, SellerProductId) DO NOTHING`)
	if err != nil {
		return result, err
	}
	defer insertAssociation.Close()

	for position, entry := range entries {
		key := matching.NewKey(entry.Name, dereference(entry.Brand), dereference(entry.Category))
		productID, found := index.Find(key)
		if !found {
			res, err := insertProduct.ExecContext(ctx, entry.Name, stringArg(entry.Brand), stringArg(entry.Category))
			if err != nil {
				return result, fmt.Errorf("entry %d: insert product: %w", position, err)
			}
			productID, err = res.LastInsertId()
			if err != nil {
				return result, fmt.Errorf("entry %d: get inserted product ID: %w", position, err)
			}
			if err := index.Add(key, productID); err != nil {
				return result, err
			}
			result.ProductsCreated++
		}

		res, err := insertAssociation.ExecContext(ctx, entry.SellerName, productID, entry.ID)
		if err != nil {
			return result, fmt.Errorf("entry %d: associate seller: %w", position, err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return result, err
		}
		if affected == 1 {
			result.AssociationsCreated++
		} else {
			var associatedProductID int64
			err := tx.QueryRowContext(ctx, `SELECT ProductId FROM SellerProduct WHERE SellerName = ? AND SellerProductId = ?`, entry.SellerName, entry.ID).Scan(&associatedProductID)
			if err != nil {
				return result, fmt.Errorf("entry %d: verify existing seller product: %w", position, err)
			}
			if associatedProductID != productID {
				return result, fmt.Errorf("entry %d: seller %q reused external product ID %q for a different catalog product", position, entry.SellerName, entry.ID)
			}
			result.AssociationsUnchanged++
		}
	}
	if err := tx.Commit(); err != nil {
		return result, fmt.Errorf("commit import: %w", err)
	}
	return result, nil
}

func nullableString(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}
func dereference(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func stringArg(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}
