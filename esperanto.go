//nolint:ireturn,wrapcheck,varnamelen,gofumpt
package esperanto

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/wroge/scan"
	"github.com/wroge/superbasic"
)

// Dialect can be any string to distinguish between different syntaxes of databases.
type Dialect string

const (
	MySQL     Dialect = "mysql"
	Sqlite    Dialect = "sqlite"
	Postgres  Dialect = "postgres"
	Oracle    Dialect = "oracle"
	SQLServer Dialect = "sqlserver"
)

type Queryable[MODEL, OPTIONS any] func(dialect Dialect, options OPTIONS) (superbasic.Expression, []scan.Column[MODEL])

type QueryExecutable[MODEL, OPTIONS any] func(dialect Dialect, options OPTIONS, models []MODEL) superbasic.Expression

type QueryOneExecutable[MODEL, OPTIONS any] func(dialect Dialect, options OPTIONS, model MODEL) superbasic.Expression

type Executable func(dialect Dialect) superbasic.Expression

func Exec(ctx context.Context, db DB, dialect Dialect, executables ...Executable) error {
	txn, err := db.Begin(ctx)
	if err != nil {
		return err
	}

	for _, exec := range executables {
		err = txn.Exec(ctx, exec(dialect))
		if err != nil {
			return txn.Rollback(ctx, err)
		}
	}

	return txn.Commit(ctx)
}

func Query[MODEL, OPTIONS any](
	ctx context.Context,
	db DB,
	dialect Dialect,
	queryable Queryable[MODEL, OPTIONS],
	options OPTIONS) ([]MODEL, error) {
	expression, columns := queryable(dialect, options)

	rows, err := db.Query(ctx, expression)
	if err != nil {
		return nil, err
	}

	return scan.All(rows, columns...)
}

func QueryOne[MODEL, OPTIONS any](
	ctx context.Context,
	db DB,
	dialect Dialect,
	queryable Queryable[MODEL, OPTIONS],
	options OPTIONS) (MODEL, error) {
	expression, columns := queryable(dialect, options)

	return scan.One(db.QueryRow(ctx, expression), columns...)
}

func QueryAndExec[MODEL, OPTIONS any](
	ctx context.Context,
	db DB,
	dialect Dialect,
	queryable Queryable[MODEL, OPTIONS],
	options OPTIONS,
	executables ...QueryExecutable[MODEL, OPTIONS]) ([]MODEL, error) {
	txn, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}

	expression, columns := queryable(dialect, options)

	rows, err := txn.Query(ctx, expression)
	if err != nil {
		return nil, txn.Rollback(ctx, err)
	}

	models, err := scan.All(rows, columns...)
	if err != nil {
		return nil, txn.Rollback(ctx, err)
	}

	for _, exec := range executables {
		err = txn.Exec(ctx, exec(dialect, options, models))
		if err != nil {
			return nil, txn.Rollback(ctx, err)
		}
	}

	return models, txn.Commit(ctx)
}

func QueryAndExecOne[MODEL, OPTIONS any](
	ctx context.Context,
	db DB, dialect Dialect,
	queryable Queryable[MODEL, OPTIONS],
	options OPTIONS,
	executables ...QueryOneExecutable[MODEL, OPTIONS]) (MODEL, error) {
	var (
		model MODEL
		err   error
	)

	txn, err := db.Begin(ctx)
	if err != nil {
		return model, err
	}

	expression, columns := queryable(dialect, options)

	row := txn.QueryRow(ctx, expression)

	model, err = scan.One(row, columns...)
	if err != nil {
		return model, txn.Rollback(ctx, err)
	}

	for _, exec := range executables {
		err = txn.Exec(ctx, exec(dialect, options, model))
		if err != nil {
			return model, txn.Rollback(ctx, err)
		}
	}

	return model, txn.Commit(ctx)
}

type Tx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context, err error) error
	Query(ctx context.Context, expression superbasic.Expression) (scan.Rows, error)
	QueryRow(ctx context.Context, expression superbasic.Expression) scan.Row
	Exec(ctx context.Context, expression superbasic.Expression) error
}

type DB interface {
	Close() error
	Begin(ctx context.Context) (Tx, error)
	Query(ctx context.Context, expression superbasic.Expression) (scan.Rows, error)
	QueryRow(ctx context.Context, expression superbasic.Expression) scan.Row
	Exec(ctx context.Context, expression superbasic.Expression) error
}

type StdDB struct {
	Placeholder string
	DB          *sql.DB
}

func (s StdDB) Close() error {
	return s.DB.Close()
}

func (s StdDB) Begin(ctx context.Context) (Tx, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	return StdTx{Placeholder: s.Placeholder, Tx: tx}, nil
}

func (s StdDB) Query(ctx context.Context, expression superbasic.Expression) (scan.Rows, error) {
	sql, args, err := superbasic.Finalize(s.Placeholder, expression)
	if err != nil {
		return nil, err
	}

	return s.DB.QueryContext(ctx, sql, args...)
}

func (s StdDB) QueryRow(ctx context.Context, expression superbasic.Expression) scan.Row {
	sql, args, err := superbasic.Finalize(s.Placeholder, expression)
	if err != nil {
		return RowError{Err: err}
	}

	return s.DB.QueryRowContext(ctx, sql, args...)
}

func (s StdDB) Exec(ctx context.Context, expression superbasic.Expression) error {
	sql, args, err := superbasic.Finalize(s.Placeholder, expression)
	if err != nil {
		return err
	}

	_, err = s.DB.ExecContext(ctx, sql, args...)
	if err != nil {
		return err
	}

	return nil
}

type StdTx struct {
	Placeholder string
	Tx          *sql.Tx
}

func (s StdTx) Commit(ctx context.Context) error {
	return s.Tx.Commit()
}

type RollbackError struct {
	Err  error
	Wrap error
}

func (re RollbackError) Error() string {
	return fmt.Sprintf("wroge/esperanto error: %s", re.Err)
}

func (re RollbackError) Unwrap() error {
	return re.Wrap
}

func (s StdTx) Rollback(ctx context.Context, err error) error {
	if rollbackErr := s.Tx.Rollback(); rollbackErr != nil {
		return RollbackError{
			Err:  rollbackErr,
			Wrap: err,
		}
	}

	return err
}

func (s StdTx) Query(ctx context.Context, expression superbasic.Expression) (scan.Rows, error) {
	sql, args, err := superbasic.Finalize(s.Placeholder, expression)
	if err != nil {
		return nil, err
	}

	return s.Tx.QueryContext(ctx, sql, args...)
}

func (s StdTx) QueryRow(ctx context.Context, expression superbasic.Expression) scan.Row {
	sql, args, err := superbasic.Finalize(s.Placeholder, expression)
	if err != nil {
		return RowError{Err: err}
	}

	return s.Tx.QueryRowContext(ctx, sql, args...)
}

func (s StdTx) Exec(ctx context.Context, expression superbasic.Expression) error {
	sql, args, err := superbasic.Finalize(s.Placeholder, expression)
	if err != nil {
		return err
	}

	_, err = s.Tx.ExecContext(ctx, sql, args...)
	if err != nil {
		return err
	}

	return nil
}

type RowError struct {
	Err error
}

func (re RowError) Scan(dest ...any) error {
	return re.Err
}
