package store

import (
	"context"
	"database/sql"
	"strings"
)

const pieceCols = `id, user_id, name, type, category, width, depth, height, color, status, price, currency, url, shop, notes, photo_id,
	inventory_id, inventory_name, wish_id, wish_name, money_tx_id, source, created_at, updated_at`

func scanPiece(sc interface{ Scan(...any) error }) (*Piece, error) {
	var p Piece
	var price, photo, inv, wish sql.NullInt64
	err := sc.Scan(&p.ID, &p.UserID, &p.Name, &p.Type, &p.Category, &p.Width, &p.Depth, &p.Height, &p.Color, &p.Status, &price, &p.Currency, &p.URL, &p.Shop, &p.Notes, &photo,
		&inv, &p.InventoryName, &wish, &p.WishName, &p.MoneyTxID, &p.Source, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	for _, x := range []struct {
		n   sql.NullInt64
		dst **int64
	}{{price, &p.Price}, {photo, &p.PhotoID}, {inv, &p.InventoryID}, {wish, &p.WishID}} {
		if x.n.Valid {
			v := x.n.Int64
			*x.dst = &v
		}
	}
	p.PlacedIn = []string{}
	return &p, nil
}

// PieceFilter narrows the furniture list.
type PieceFilter struct {
	Status string
	Q      string
	Limit  int
}

// ListPieces returns a person's furniture.
func (s *Store) ListPieces(ctx context.Context, uid int64, f PieceFilter) ([]Piece, error) {
	q := `SELECT ` + pieceCols + ` FROM pieces WHERE user_id = ?`
	args := []any{uid}
	if f.Status != "" {
		q += ` AND status = ?`
		args = append(args, f.Status)
	} else {
		q += ` AND status <> 'gone'`
	}
	if t := strings.TrimSpace(f.Q); t != "" {
		q += ` AND (name LIKE ? OR notes LIKE ? OR shop LIKE ? OR type LIKE ?)`
		like := "%" + t + "%"
		args = append(args, like, like, like, like)
	}
	q += ` ORDER BY CASE status WHEN 'own' THEN 0 WHEN 'want' THEN 1 WHEN 'maybe' THEN 2 ELSE 3 END, category, name COLLATE NOCASE`
	if f.Limit > 0 {
		q += ` LIMIT ?`
		args = append(args, f.Limit)
	}
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	out := []Piece{}
	for rows.Next() {
		p, err := scanPiece(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *p)
	}
	rows.Close()
	return out, rows.Err()
}

// GetPiece returns one of a person's pieces.
func (s *Store) GetPiece(ctx context.Context, uid, id int64) (*Piece, error) {
	p, err := scanPiece(s.DB.QueryRowContext(ctx, `SELECT `+pieceCols+` FROM pieces WHERE user_id = ? AND id = ?`, uid, id))
	return p, notFound(err)
}

// CreatePiece adds a piece.
func (s *Store) CreatePiece(ctx context.Context, p *Piece) error {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO pieces (user_id, name, type, category, width, depth, height, color, status, price, currency, url, shop, notes, photo_id,
		inventory_id, inventory_name, wish_id, wish_name, money_tx_id, source) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.UserID, p.Name, p.Type, p.Category, p.Width, p.Depth, p.Height, p.Color, p.Status, nullI(p.Price), p.Currency, p.URL, p.Shop, p.Notes, nullI(p.PhotoID),
		nullI(p.InventoryID), p.InventoryName, nullI(p.WishID), p.WishName, p.MoneyTxID, p.Source)
	if err != nil {
		return err
	}
	p.ID, _ = res.LastInsertId()
	return nil
}

// UpdatePiece saves a piece.
func (s *Store) UpdatePiece(ctx context.Context, p *Piece) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE pieces SET name = ?, type = ?, category = ?, width = ?, depth = ?, height = ?, color = ?, status = ?, price = ?, currency = ?,
		url = ?, shop = ?, notes = ?, photo_id = ?, inventory_id = ?, inventory_name = ?, wish_id = ?, wish_name = ?, money_tx_id = ?,
		updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE user_id = ? AND id = ?`,
		p.Name, p.Type, p.Category, p.Width, p.Depth, p.Height, p.Color, p.Status, nullI(p.Price), p.Currency,
		p.URL, p.Shop, p.Notes, nullI(p.PhotoID), nullI(p.InventoryID), p.InventoryName, nullI(p.WishID), p.WishName, p.MoneyTxID, p.UserID, p.ID)
	return affected(res, err)
}

// DeletePiece removes a piece (placed copies stay on the plans, unlinked).
func (s *Store) DeletePiece(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM pieces WHERE user_id = ? AND id = ?`, uid, id)
	return affected(res, err)
}

// PieceByRef finds a piece already linked to an inventory item or a wish.
func (s *Store) PieceByRef(ctx context.Context, uid int64, col string, id int64) (*Piece, error) {
	if col != "inventory_id" && col != "wish_id" {
		return nil, ErrNotFound
	}
	p, err := scanPiece(s.DB.QueryRowContext(ctx, `SELECT `+pieceCols+` FROM pieces WHERE user_id = ? AND `+col+` = ? LIMIT 1`, uid, id))
	return p, notFound(err)
}
