package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
)

// ErrStale means a layout was saved somewhere else in the meantime.
var ErrStale = errors.New("this layout was changed somewhere else")

const planCols = `id, user_id, name, kind, address, currency, notes, status, home_id, home_name, rental_id, rental_name,
	share_token, share_on, pinned, archived, source, created_at, updated_at`

func scanPlan(sc interface{ Scan(...any) error }) (*Plan, error) {
	var p Plan
	var hid, rid sql.NullInt64
	var share, pin, arch int
	err := sc.Scan(&p.ID, &p.UserID, &p.Name, &p.Kind, &p.Address, &p.Currency, &p.Notes, &p.Status, &hid, &p.HomeName, &rid, &p.RentalName,
		&p.ShareToken, &share, &pin, &arch, &p.Source, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if hid.Valid {
		p.HomeID = &hid.Int64
	}
	if rid.Valid {
		p.RentalID = &rid.Int64
	}
	p.ShareOn, p.Pinned, p.Archived = share == 1, pin == 1, arch == 1
	return &p, nil
}

// ListPlans returns a person's plans, pinned first, then the latest changed.
func (s *Store) ListPlans(ctx context.Context, uid int64, archived bool) ([]Plan, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+planCols+` FROM plans WHERE user_id = ? AND archived = ? ORDER BY pinned DESC, updated_at DESC`, uid, boolInt(archived))
	if err != nil {
		return nil, err
	}
	out := []Plan{}
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *p)
	}
	rows.Close()
	return out, rows.Err()
}

// GetPlan returns one of a person's plans.
func (s *Store) GetPlan(ctx context.Context, uid, id int64) (*Plan, error) {
	p, err := scanPlan(s.DB.QueryRowContext(ctx, `SELECT `+planCols+` FROM plans WHERE user_id = ? AND id = ?`, uid, id))
	return p, notFound(err)
}

// PlanByShare finds a plan by its share link (only while sharing is on).
func (s *Store) PlanByShare(ctx context.Context, token string) (*Plan, error) {
	if token == "" {
		return nil, ErrNotFound
	}
	p, err := scanPlan(s.DB.QueryRowContext(ctx, `SELECT `+planCols+` FROM plans WHERE share_token = ? AND share_on = 1 AND archived = 0`, token))
	return p, notFound(err)
}

// CreatePlan adds a plan.
func (s *Store) CreatePlan(ctx context.Context, p *Plan) error {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO plans (user_id, name, kind, address, currency, notes, status, home_id, home_name, rental_id, rental_name, pinned, source)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.UserID, p.Name, p.Kind, p.Address, p.Currency, p.Notes, p.Status, nullI(p.HomeID), p.HomeName, nullI(p.RentalID), p.RentalName, boolInt(p.Pinned), p.Source)
	if err != nil {
		return err
	}
	p.ID, _ = res.LastInsertId()
	return nil
}

// UpdatePlan saves a plan's details.
func (s *Store) UpdatePlan(ctx context.Context, p *Plan) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE plans SET name = ?, kind = ?, address = ?, currency = ?, notes = ?, status = ?, home_id = ?, home_name = ?,
		rental_id = ?, rental_name = ?, share_token = ?, share_on = ?, pinned = ?, archived = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now')
		WHERE user_id = ? AND id = ?`,
		p.Name, p.Kind, p.Address, p.Currency, p.Notes, p.Status, nullI(p.HomeID), p.HomeName, nullI(p.RentalID), p.RentalName,
		p.ShareToken, boolInt(p.ShareOn), boolInt(p.Pinned), boolInt(p.Archived), p.UserID, p.ID)
	return affected(res, err)
}

// TouchPlan marks a plan changed now.
func (s *Store) TouchPlan(ctx context.Context, uid, id int64) {
	_, _ = s.DB.ExecContext(ctx, `UPDATE plans SET updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE user_id = ? AND id = ?`, uid, id)
}

// DeletePlan removes a plan with its floors, layouts, routes, checklist and photos.
func (s *Store) DeletePlan(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM plans WHERE user_id = ? AND id = ?`, uid, id)
	return affected(res, err)
}

func affected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- floors ----

const floorCols = `id, user_id, plan_id, name, level, ceiling_cm, sort, current_layout_id, underlay_photo_id, underlay, created_at`

func scanFloor(sc interface{ Scan(...any) error }) (*Floor, error) {
	var f Floor
	var cur, up sql.NullInt64
	var u string
	if err := sc.Scan(&f.ID, &f.UserID, &f.PlanID, &f.Name, &f.Level, &f.CeilingCm, &f.Sort, &cur, &up, &u, &f.CreatedAt); err != nil {
		return nil, err
	}
	if cur.Valid {
		f.CurrentLayoutID = &cur.Int64
	}
	if up.Valid {
		f.UnderlayPhotoID = &up.Int64
	}
	if u == "" {
		u = "{}"
	}
	f.Underlay = json.RawMessage(u)
	return &f, nil
}

// ListFloors returns a plan's floors, lowest first.
func (s *Store) ListFloors(ctx context.Context, uid, planID int64) ([]Floor, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+floorCols+` FROM floors WHERE user_id = ? AND plan_id = ? ORDER BY level, sort, id`, uid, planID)
	if err != nil {
		return nil, err
	}
	out := []Floor{}
	for rows.Next() {
		f, err := scanFloor(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *f)
	}
	rows.Close()
	return out, rows.Err()
}

// GetFloor returns one of a person's floors.
func (s *Store) GetFloor(ctx context.Context, uid, id int64) (*Floor, error) {
	f, err := scanFloor(s.DB.QueryRowContext(ctx, `SELECT `+floorCols+` FROM floors WHERE user_id = ? AND id = ?`, uid, id))
	return f, notFound(err)
}

// CreateFloor adds a floor.
func (s *Store) CreateFloor(ctx context.Context, f *Floor) error {
	if len(f.Underlay) == 0 {
		f.Underlay = json.RawMessage("{}")
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO floors (user_id, plan_id, name, level, ceiling_cm, sort, underlay) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		f.UserID, f.PlanID, f.Name, f.Level, f.CeilingCm, f.Sort, string(f.Underlay))
	if err != nil {
		return err
	}
	f.ID, _ = res.LastInsertId()
	return nil
}

// UpdateFloor saves a floor.
func (s *Store) UpdateFloor(ctx context.Context, f *Floor) error {
	if len(f.Underlay) == 0 {
		f.Underlay = json.RawMessage("{}")
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE floors SET name = ?, level = ?, ceiling_cm = ?, sort = ?, current_layout_id = ?, underlay_photo_id = ?, underlay = ?
		WHERE user_id = ? AND id = ?`, f.Name, f.Level, f.CeilingCm, f.Sort, nullI(f.CurrentLayoutID), nullI(f.UnderlayPhotoID), string(f.Underlay), f.UserID, f.ID)
	return affected(res, err)
}

// DeleteFloor removes a floor and its layouts.
func (s *Store) DeleteFloor(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM floors WHERE user_id = ? AND id = ?`, uid, id)
	return affected(res, err)
}

// ---- layouts ----

const layoutCols = `id, user_id, plan_id, floor_id, name, notes, data, area_cm2, rooms_n, items_n, budget, sort, revision, created_at, updated_at`

func scanLayout(sc interface{ Scan(...any) error }, withData bool) (*Layout, error) {
	var l Layout
	var data string
	if err := sc.Scan(&l.ID, &l.UserID, &l.PlanID, &l.FloorID, &l.Name, &l.Notes, &data, &l.AreaCm2, &l.RoomsN, &l.ItemsN, &l.Budget, &l.Sort, &l.Revision, &l.CreatedAt, &l.UpdatedAt); err != nil {
		return nil, err
	}
	if withData {
		if data == "" {
			data = "{}"
		}
		l.Data = json.RawMessage(data)
	}
	return &l, nil
}

// ListLayouts returns the layouts of a person's plan (data only when asked).
func (s *Store) ListLayouts(ctx context.Context, uid, planID int64, withData bool) ([]Layout, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+layoutCols+` FROM layouts WHERE user_id = ? AND plan_id = ? ORDER BY floor_id, sort, id`, uid, planID)
	if err != nil {
		return nil, err
	}
	out := []Layout{}
	for rows.Next() {
		l, err := scanLayout(rows, withData)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *l)
	}
	rows.Close()
	return out, rows.Err()
}

// AllLayouts returns every layout of a person (for export and the furniture list).
func (s *Store) AllLayouts(ctx context.Context, uid int64) ([]Layout, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+layoutCols+` FROM layouts WHERE user_id = ? ORDER BY plan_id, floor_id, sort, id`, uid)
	if err != nil {
		return nil, err
	}
	out := []Layout{}
	for rows.Next() {
		l, err := scanLayout(rows, true)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *l)
	}
	rows.Close()
	return out, rows.Err()
}

// GetLayout returns one of a person's layouts with its drawing.
func (s *Store) GetLayout(ctx context.Context, uid, id int64) (*Layout, error) {
	l, err := scanLayout(s.DB.QueryRowContext(ctx, `SELECT `+layoutCols+` FROM layouts WHERE user_id = ? AND id = ?`, uid, id), true)
	return l, notFound(err)
}

func layoutTotals(l *Layout) {
	d, err := ParseDrawing(l.Data)
	if err != nil {
		return
	}
	l.AreaCm2, l.RoomsN, l.ItemsN, l.Budget = d.Totals()
}

// CreateLayout adds a layout (the totals are worked out from the drawing).
func (s *Store) CreateLayout(ctx context.Context, l *Layout) error {
	if len(l.Data) == 0 {
		l.Data = json.RawMessage(`{}`)
	}
	layoutTotals(l)
	res, err := s.DB.ExecContext(ctx, `INSERT INTO layouts (user_id, plan_id, floor_id, name, notes, data, area_cm2, rooms_n, items_n, budget, sort) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		l.UserID, l.PlanID, l.FloorID, l.Name, l.Notes, string(l.Data), l.AreaCm2, l.RoomsN, l.ItemsN, l.Budget, l.Sort)
	if err != nil {
		return err
	}
	l.ID, _ = res.LastInsertId()
	l.Revision = 1
	return nil
}

// UpdateLayout saves a layout. With rev > 0 the save is refused (ErrStale)
// when the layout has been saved since that revision.
func (s *Store) UpdateLayout(ctx context.Context, l *Layout, rev int) error {
	layoutTotals(l)
	q := `UPDATE layouts SET name = ?, notes = ?, data = ?, area_cm2 = ?, rooms_n = ?, items_n = ?, budget = ?, sort = ?, revision = revision + 1,
		updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE user_id = ? AND id = ?`
	args := []any{l.Name, l.Notes, string(l.Data), l.AreaCm2, l.RoomsN, l.ItemsN, l.Budget, l.Sort, l.UserID, l.ID}
	if rev > 0 {
		q += ` AND revision = ?`
		args = append(args, rev)
	}
	res, err := s.DB.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		if rev > 0 {
			if _, e := s.GetLayout(ctx, l.UserID, l.ID); e == nil {
				return ErrStale
			}
		}
		return ErrNotFound
	}
	_ = s.DB.QueryRowContext(ctx, `SELECT revision, updated_at FROM layouts WHERE id = ?`, l.ID).Scan(&l.Revision, &l.UpdatedAt)
	return nil
}

// DeleteLayout removes a layout.
func (s *Store) DeleteLayout(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM layouts WHERE user_id = ? AND id = ?`, uid, id)
	return affected(res, err)
}
