package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

type sqlStore struct {
	db     *sql.DB
	driver string
}

func openSQL(ctx context.Context, c Config) (Store, error) {
	d := map[string]string{"sqlite": "sqlite", "postgres": "pgx", "mysql": "mysql", "mariadb": "mysql"}[c.Driver]
	addr := c.Address
	if d == "sqlite" {
		addr += "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	}
	db, e := sql.Open(d, addr)
	if e != nil {
		return nil, e
	}
	db.SetMaxOpenConns(8)
	if d == "sqlite" {
		db.SetMaxOpenConns(1)
	}
	blob := "TEXT"
	idType := "VARCHAR(128)"
	if d == "mysql" {
		blob = "LONGTEXT"
		idType += " CHARACTER SET ascii COLLATE ascii_bin"
	}
	_, e = db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS records (kind "+idType+" NOT NULL, id "+idType+" NOT NULL, rank_value BIGINT NOT NULL, published BOOLEAN NOT NULL, payload "+blob+" NOT NULL, PRIMARY KEY(kind,id))")
	if e == nil {
		for _, idx := range []string{"feed_public ON records(kind,published,rank_value,id)", "feed_all ON records(kind,rank_value,id)"} {
			prefix := "CREATE INDEX IF NOT EXISTS "
			if d == "mysql" {
				prefix = "CREATE INDEX "
			}
			_, e = db.ExecContext(ctx, prefix+idx)
			if e != nil && d == "mysql" && strings.Contains(e.Error(), "1061") {
				e = nil
			}
			if e != nil {
				break
			}
		}
	}
	if e != nil {
		db.Close()
		return nil, e
	}
	return &sqlStore{db, c.Driver}, nil
}
func (s *sqlStore) bind(q string) string {
	if s.driver != "postgres" {
		return q
	}
	i := 0
	return rebind(q, &i)
}
func rebind(q string, i *int) string {
	var b strings.Builder
	for _, r := range q {
		if r == '?' {
			*i++
			fmt.Fprintf(&b, "$%d", *i)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

const fields = "kind,id,rank_value,published,payload"

func scan(row interface{ Scan(...any) error }) (Record, error) {
	var r Record
	var b string
	e := row.Scan(&r.Kind, &r.ID, &r.Rank, &r.Published, &b)
	r.Data = []byte(b)
	if errors.Is(e, sql.ErrNoRows) {
		e = ErrMissing
	}
	return r, e
}
func (s *sqlStore) Get(ctx context.Context, k, id string) (Record, error) {
	return scan(s.db.QueryRowContext(ctx, s.bind("SELECT "+fields+" FROM records WHERE kind=? AND id=?"), k, id))
}
func (s *sqlStore) Put(ctx context.Context, r Record) error {
	q := "INSERT INTO records (" + fields + ") VALUES (?,?,?,?,?) "
	if s.driver == "mysql" || s.driver == "mariadb" {
		q += "ON DUPLICATE KEY UPDATE rank_value=VALUES(rank_value),published=VALUES(published),payload=VALUES(payload)"
	} else {
		q += "ON CONFLICT(kind,id) DO UPDATE SET rank_value=excluded.rank_value,published=excluded.published,payload=excluded.payload"
	}
	_, e := s.db.ExecContext(ctx, s.bind(q), r.Kind, r.ID, r.Rank, r.Published, string(r.Data))
	return e
}
func (s *sqlStore) Delete(ctx context.Context, k, id string) error {
	_, e := s.db.ExecContext(ctx, s.bind("DELETE FROM records WHERE kind=? AND id=?"), k, id)
	return e
}
func (s *sqlStore) List(ctx context.Context, q Query) ([]Record, error) {
	query := "SELECT " + fields + " FROM records WHERE kind=?"
	args := []any{q.Kind}
	if q.Prefix != "" {
		query += " AND id>=? AND id<?"
		args = append(args, q.Prefix, q.Prefix+"~")
	}
	if q.Unpublished {
		query += " AND published=FALSE"
	}
	if q.Public {
		query += " AND published=TRUE"
	}
	if q.Before != nil && (s.driver == "sqlite" || s.driver == "postgres") {
		// A tuple boundary allows a direct seek into the ordering index.
		query += " AND (rank_value,id)<(?,?)"
		args = append(args, q.Before.Rank, q.Before.ID)
	} else if q.Before != nil {
		query += " AND (rank_value < ? OR (rank_value=? AND id<?))"
		args = append(args, q.Before.Rank, q.Before.Rank, q.Before.ID)
	}
	query += " ORDER BY rank_value DESC,id DESC"
	if q.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, q.Limit)
	}
	rs, e := s.db.QueryContext(ctx, s.bind(query), args...)
	if e != nil {
		return nil, e
	}
	defer rs.Close()
	out := []Record{}
	for rs.Next() {
		r, e := scan(rs)
		if e != nil {
			return nil, e
		}
		out = append(out, r)
	}
	return out, rs.Err()
}
func (s *sqlStore) Close() error { return s.db.Close() }
