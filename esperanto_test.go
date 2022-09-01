package esperanto_test

import (
	"testing"

	"github.com/wroge/esperanto"
)

func TestCreate(t *testing.T) {
	t.Parallel()

	create := esperanto.Compile("CREATE TABLE presidents (\n\t?\n)",
		esperanto.Join(",\n\t",
			esperanto.Switch{
				esperanto.Postgres:  esperanto.SQL("nr SERIAL PRIMARY KEY"),
				esperanto.Sqlite:    esperanto.SQL("nr INTEGER PRIMARY KEY AUTOINCREMENT"),
				esperanto.SQLServer: esperanto.SQL("nr INT IDENTITY PRIMARY KEY"),
			},
			esperanto.SQL("first TEXT NOT NULL"),
			esperanto.SQL("last TEXT NOT NULL"),
		),
	)

	sql, _, err := esperanto.Finalize("$%d", esperanto.Postgres, create)
	if err != nil {
		t.Error(err)
	}

	if sql != `CREATE TABLE presidents (
	nr SERIAL PRIMARY KEY,
	first TEXT NOT NULL,
	last TEXT NOT NULL
)` {
		t.Fatal(sql)
	}

	sql, _, err = esperanto.Finalize("?", esperanto.Sqlite, create)
	if err != nil {
		t.Error(err)
	}

	if sql != `CREATE TABLE presidents (
	nr INTEGER PRIMARY KEY AUTOINCREMENT,
	first TEXT NOT NULL,
	last TEXT NOT NULL
)` {
		t.Fatal(sql)
	}

	sql, _, err = esperanto.Finalize("@p%d", esperanto.SQLServer, create)
	if err != nil {
		t.Error(err)
	}

	if sql != `CREATE TABLE presidents (
	nr INT IDENTITY PRIMARY KEY,
	first TEXT NOT NULL,
	last TEXT NOT NULL
)` {
		t.Fatal(sql)
	}
}

func TestInsert(t *testing.T) {
	t.Parallel()

	insert := esperanto.Join(" ",
		esperanto.SQL("INSERT INTO presidents (first, last)"),
		esperanto.Switch{
			esperanto.SQLServer: esperanto.SQL("OUTPUT INSERTED.nr"),
		},
		esperanto.Compile("VALUES ?",
			esperanto.Join(", ",
				esperanto.Map([]President{
					{"George", "Washington"},
					{"John", "Adams"},
				},
					func(president President) esperanto.Expression {
						return esperanto.Values{president.First, president.Last}
					})...,
			),
		),
		esperanto.Switch{
			esperanto.Postgres: esperanto.SQL("RETURNING nr"),
			esperanto.Sqlite:   esperanto.SQL("RETURNING nr"),
		},
	)

	sql, _, err := esperanto.Finalize("$%d", esperanto.Postgres, insert)
	if err != nil {
		t.Error(err)
	}

	if sql != "INSERT INTO presidents (first, last) VALUES ($1, $2), ($3, $4) RETURNING nr" {
		t.Fatal(sql)
	}

	sql, _, err = esperanto.Finalize("?", esperanto.Sqlite, insert)
	if err != nil {
		t.Error(err)
	}

	if sql != "INSERT INTO presidents (first, last) VALUES (?, ?), (?, ?) RETURNING nr" {
		t.Fatal(sql)
	}

	sql, _, err = esperanto.Finalize("@p%d", esperanto.SQLServer, insert)
	if err != nil {
		t.Error(err)
	}

	if sql != "INSERT INTO presidents (first, last) OUTPUT INSERTED.nr VALUES (@p1, @p2), (@p3, @p4)" {
		t.Fatal(sql)
	}
}

type President struct {
	First string
	Last  string
}

func TestQuery(t *testing.T) {
	t.Parallel()

	equals := esperanto.Switch{
		esperanto.Postgres:  esperanto.SQL("last = ?", "Adams"),
		esperanto.Sqlite:    esperanto.SQL("last = ?", "Adams"),
		esperanto.SQLServer: esperanto.SQL("CONVERT(VARCHAR, last) = ? COLLATE Latin1_General_CS_AS", "Adams"),
	}

	query := esperanto.Compile("SELECT ? FROM (?) AS q",
		esperanto.Switch{
			esperanto.Postgres: esperanto.SQL("JSON_BUILD_OBJECT('nr', q.nr, 'first', q.first, 'last', q.last) AS result"),
			esperanto.Sqlite:   esperanto.SQL("JSON_OBJECT('nr', q.nr, 'first', q.first, 'last', q.last) AS result"),
			// https://docs.microsoft.com/en-us/sql/t-sql/functions/json-object-transact-sql
			esperanto.SQLServer: esperanto.SQL("JSON_OBJECT('nr': q.nr, 'first': q.first, 'last': q.last) AS result"),
		},
		esperanto.Join(" ",
			esperanto.SQL("SELECT nr, first, last FROM presidents"),
			esperanto.If(equals != nil, esperanto.Compile("WHERE ?", equals)),
		),
	)

	sql, args, err := esperanto.Finalize("$%d", esperanto.Postgres, query)
	if err != nil {
		t.Error(err)
	}

	if sql != "SELECT JSON_BUILD_OBJECT('nr', q.nr, 'first', q.first, 'last', q.last) AS result FROM"+
		" (SELECT nr, first, last FROM presidents WHERE last = $1) AS q" || len(args) != 1 {
		t.Fatal(sql)
	}

	sql, args, err = esperanto.Finalize("?", esperanto.Sqlite, query)
	if err != nil {
		t.Error(err)
	}

	if sql != "SELECT JSON_OBJECT('nr', q.nr, 'first', q.first, 'last', q.last) AS result FROM"+
		" (SELECT nr, first, last FROM presidents WHERE last = ?) AS q" || len(args) != 1 {
		t.Fatal(sql)
	}

	sql, args, err = esperanto.Finalize("@p%d", esperanto.SQLServer, query)
	if err != nil {
		t.Error(err)
	}

	if sql != "SELECT JSON_OBJECT('nr': q.nr, 'first': q.first, 'last': q.last) AS result FROM"+
		" (SELECT nr, first, last FROM presidents WHERE CONVERT(VARCHAR, last) = @p1 COLLATE Latin1_General_CS_AS) AS q" ||
		len(args) != 1 {
		t.Fatal(sql)
	}
}
