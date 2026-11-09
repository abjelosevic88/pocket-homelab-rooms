package store

import (
	"context"
	"database/sql"
	"encoding/json"
)

// ---- routes (the way in) ----

func scanRoute(sc interface{ Scan(...any) error }) (*Route, error) {
	var r Route
	var steps string
	if err := sc.Scan(&r.ID, &r.UserID, &r.PlanID, &r.Name, &steps, &r.Sort, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(steps), &r.Steps)
	if r.Steps == nil {
		r.Steps = []RouteStep{}
	}
	return &r, nil
}

const routeCols = `id, user_id, plan_id, name, steps, sort, created_at, updated_at`

// ListRoutes returns a plan's ways in (planID 0 = every plan's).
func (s *Store) ListRoutes(ctx context.Context, uid, planID int64) ([]Route, error) {
	q := `SELECT ` + routeCols + ` FROM routes WHERE user_id = ?`
	args := []any{uid}
	if planID > 0 {
		q += ` AND plan_id = ?`
		args = append(args, planID)
	}
	rows, err := s.DB.QueryContext(ctx, q+` ORDER BY plan_id, sort, id`, args...)
	if err != nil {
		return nil, err
	}
	out := []Route{}
	for rows.Next() {
		r, err := scanRoute(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *r)
	}
	rows.Close()
	return out, rows.Err()
}

// GetRoute returns one of a person's routes.
func (s *Store) GetRoute(ctx context.Context, uid, id int64) (*Route, error) {
	r, err := scanRoute(s.DB.QueryRowContext(ctx, `SELECT `+routeCols+` FROM routes WHERE user_id = ? AND id = ?`, uid, id))
	return r, notFound(err)
}

// SaveRoute adds or updates a route.
func (s *Store) SaveRoute(ctx context.Context, r *Route) error {
	if r.Steps == nil {
		r.Steps = []RouteStep{}
	}
	steps := toJSON(r.Steps)
	if r.ID == 0 {
		res, err := s.DB.ExecContext(ctx, `INSERT INTO routes (user_id, plan_id, name, steps, sort) VALUES (?, ?, ?, ?, ?)`, r.UserID, r.PlanID, r.Name, steps, r.Sort)
		if err != nil {
			return err
		}
		r.ID, _ = res.LastInsertId()
		return nil
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE routes SET name = ?, steps = ?, sort = ?, updated_at = strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE user_id = ? AND id = ?`,
		r.Name, steps, r.Sort, r.UserID, r.ID)
	return affected(res, err)
}

// DeleteRoute removes a route.
func (s *Store) DeleteRoute(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM routes WHERE user_id = ? AND id = ?`, uid, id)
	return affected(res, err)
}

// ---- todos (a plan's checklist) ----

const todoCols = `t.id, t.user_id, t.plan_id, t.title, t.done, t.due, t.task_id, t.task_url, t.sort, t.source, t.done_at, t.created_at, p.name`

func scanTodo(sc interface{ Scan(...any) error }) (*Todo, error) {
	var t Todo
	var done int
	if err := sc.Scan(&t.ID, &t.UserID, &t.PlanID, &t.Title, &done, &t.Due, &t.TaskID, &t.TaskURL, &t.Sort, &t.Source, &t.DoneAt, &t.CreatedAt, &t.PlanName); err != nil {
		return nil, err
	}
	t.Done = done == 1
	return &t, nil
}

// TodoFilter narrows the checklist.
type TodoFilter struct {
	PlanID  int64
	Open    bool
	DueFrom string
	DueTo   string
}

// ListTodos returns a person's checklist items.
func (s *Store) ListTodos(ctx context.Context, uid int64, f TodoFilter) ([]Todo, error) {
	q := `SELECT ` + todoCols + ` FROM todos t JOIN plans p ON p.id = t.plan_id WHERE t.user_id = ?`
	args := []any{uid}
	if f.PlanID > 0 {
		q += ` AND t.plan_id = ?`
		args = append(args, f.PlanID)
	} else {
		q += ` AND p.archived = 0`
	}
	if f.Open {
		q += ` AND t.done = 0`
	}
	if f.DueFrom != "" {
		q += ` AND t.due <> '' AND t.due >= ?`
		args = append(args, f.DueFrom)
	}
	if f.DueTo != "" {
		q += ` AND t.due <> '' AND t.due <= ?`
		args = append(args, f.DueTo)
	}
	rows, err := s.DB.QueryContext(ctx, q+` ORDER BY t.done, CASE WHEN t.due = '' THEN 1 ELSE 0 END, t.due, t.sort, t.id`, args...)
	if err != nil {
		return nil, err
	}
	out := []Todo{}
	for rows.Next() {
		t, err := scanTodo(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *t)
	}
	rows.Close()
	return out, rows.Err()
}

// GetTodo returns one checklist item.
func (s *Store) GetTodo(ctx context.Context, uid, id int64) (*Todo, error) {
	t, err := scanTodo(s.DB.QueryRowContext(ctx, `SELECT `+todoCols+` FROM todos t JOIN plans p ON p.id = t.plan_id WHERE t.user_id = ? AND t.id = ?`, uid, id))
	return t, notFound(err)
}

// SaveTodo adds or updates a checklist item.
func (s *Store) SaveTodo(ctx context.Context, t *Todo) error {
	if t.ID == 0 {
		res, err := s.DB.ExecContext(ctx, `INSERT INTO todos (user_id, plan_id, title, done, due, task_id, task_url, sort, source, done_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			t.UserID, t.PlanID, t.Title, boolInt(t.Done), t.Due, t.TaskID, t.TaskURL, t.Sort, t.Source, t.DoneAt)
		if err != nil {
			return err
		}
		t.ID, _ = res.LastInsertId()
		return nil
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE todos SET title = ?, done = ?, due = ?, task_id = ?, task_url = ?, sort = ?, done_at = ? WHERE user_id = ? AND id = ?`,
		t.Title, boolInt(t.Done), t.Due, t.TaskID, t.TaskURL, t.Sort, t.DoneAt, t.UserID, t.ID)
	return affected(res, err)
}

// DeleteTodo removes a checklist item.
func (s *Store) DeleteTodo(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM todos WHERE user_id = ? AND id = ?`, uid, id)
	return affected(res, err)
}

// OpenTodos counts a plan's open checklist items.
func (s *Store) OpenTodos(ctx context.Context, uid, planID int64) int {
	var n int
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM todos WHERE user_id = ? AND plan_id = ? AND done = 0`, uid, planID).Scan(&n)
	return n
}

// ---- plan photos ----

const planPhotoCols = `id, user_id, plan_id, photo_id, room, caption, pocket_id, pocket_url, taken_on, created_at`

// ListPlanPhotos returns a plan's room photos, newest first.
func (s *Store) ListPlanPhotos(ctx context.Context, uid, planID int64) ([]PlanPhoto, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+planPhotoCols+` FROM plan_photos WHERE user_id = ? AND plan_id = ? ORDER BY room, created_at DESC`, uid, planID)
	if err != nil {
		return nil, err
	}
	out := []PlanPhoto{}
	for rows.Next() {
		var p PlanPhoto
		var pid sql.NullInt64
		if err := rows.Scan(&p.ID, &p.UserID, &p.PlanID, &pid, &p.Room, &p.Caption, &p.PocketID, &p.PocketURL, &p.TakenOn, &p.CreatedAt); err != nil {
			rows.Close()
			return nil, err
		}
		if pid.Valid {
			p.PhotoID = &pid.Int64
		}
		out = append(out, p)
	}
	rows.Close()
	return out, rows.Err()
}

// GetPlanPhoto returns one room photo.
func (s *Store) GetPlanPhoto(ctx context.Context, uid, id int64) (*PlanPhoto, error) {
	var p PlanPhoto
	var pid sql.NullInt64
	err := s.DB.QueryRowContext(ctx, `SELECT `+planPhotoCols+` FROM plan_photos WHERE user_id = ? AND id = ?`, uid, id).
		Scan(&p.ID, &p.UserID, &p.PlanID, &pid, &p.Room, &p.Caption, &p.PocketID, &p.PocketURL, &p.TakenOn, &p.CreatedAt)
	if err != nil {
		return nil, notFound(err)
	}
	if pid.Valid {
		p.PhotoID = &pid.Int64
	}
	return &p, nil
}

// SavePlanPhoto adds or updates a room photo.
func (s *Store) SavePlanPhoto(ctx context.Context, p *PlanPhoto) error {
	if p.ID == 0 {
		res, err := s.DB.ExecContext(ctx, `INSERT INTO plan_photos (user_id, plan_id, photo_id, room, caption, pocket_id, pocket_url, taken_on) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			p.UserID, p.PlanID, nullI(p.PhotoID), p.Room, p.Caption, p.PocketID, p.PocketURL, p.TakenOn)
		if err != nil {
			return err
		}
		p.ID, _ = res.LastInsertId()
		return nil
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE plan_photos SET room = ?, caption = ?, taken_on = ? WHERE user_id = ? AND id = ?`, p.Room, p.Caption, p.TakenOn, p.UserID, p.ID)
	return affected(res, err)
}

// DeletePlanPhoto removes a room photo (and its picture).
func (s *Store) DeletePlanPhoto(ctx context.Context, uid, id int64) error {
	p, err := s.GetPlanPhoto(ctx, uid, id)
	if err != nil {
		return err
	}
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM plan_photos WHERE user_id = ? AND id = ?`, uid, id); err != nil {
		return err
	}
	if p.PhotoID != nil {
		_, _ = s.DB.ExecContext(ctx, `DELETE FROM photos WHERE user_id = ? AND id = ?`, uid, *p.PhotoID)
	}
	return nil
}
