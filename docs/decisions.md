# Architecture and technical decisions

## Scope
A CLI imports seller catalog entries from JSON into the existing SQLite catalog. A product is reused when its normalized Name, Brand, and Category match an existing catalog product. Otherwise, a new canonical Product row is created. Each seller listing is associated with that canonical product.

## Matching policy
- Unicode decomposition and removal of combining marks (accent-insensitive).
- Case folding (case-insensitive).
- Punctuation and whitespace are treated as separators; repeated whitespace is collapsed.
- The matching key is the tuple (Name, Brand, Category).
- Null and empty Brand/Category are treated as equivalent empty values.
- No translation, synonym expansion, or fuzzy matching is performed. Potential semantic matches are intentionally left as separate products pending a stronger business identifier or human review.
- If the existing catalog contains two different products with the same normalized key, import fails rather than selecting an arbitrary row.

## External seller identifiers
The input Id is a string UUID, while the supplied SQLite schema declares SellerProduct.SellerProductId as INTEGER. The migration changes it to TEXT. Uniqueness is scoped to (SellerName, SellerProductId), since seller identifiers are not assumed to be globally unique.

If a seller reuses the same external ID for a different canonical product, the import fails and the transaction rolls back.

## Transactions and idempotency
The entire file is imported in one transaction. This gives all-or-nothing behavior and keeps the assessment implementation straightforward. A unique index protects seller external IDs. Re-importing the same file does not create duplicate products or associations.

## Security
All values are passed as SQL parameters. The input includes a deliberate-looking SQL-injection string in a Brand field; it is treated as ordinary data, never interpolated into SQL.

## Known limitations
The matching policy does not detect translations (e.g. Router vs Roteador), synonyms, or category aliases (e.g. Photo vs Photography). It may also keep two semantically identical products separate if their descriptions differ materially. A production system could introduce GTIN/EAN/UPC or manufacturer part number matching, configurable aliases, confidence-scored fuzzy matching, and a manual review workflow.
