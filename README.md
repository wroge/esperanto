# esperanto

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/wroge/esperanto)
[![Go Report Card](https://goreportcard.com/badge/github.com/wroge/esperanto)](https://goreportcard.com/report/github.com/wroge/esperanto)
![golangci-lint](https://github.com/wroge/esperanto/workflows/golangci-lint/badge.svg)
[![codecov](https://codecov.io/gh/wroge/esperanto/branch/main/graph/badge.svg?token=D2r0ktepvb)](https://codecov.io/gh/wroge/esperanto)
[![tippin.me](https://badgen.net/badge/%E2%9A%A1%EF%B8%8Ftippin.me/@_wroge/F0918E)](https://tippin.me/@_wroge)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/wroge/esperanto.svg?style=social)](https://github.com/wroge/esperanto/tags)

esperanto makes it easy to create SQL expressions for multiple dialects. 

```esperanto.Compile``` compiles expressions into an SQL template and thus offers an alternative to conventional query builders.

If you only need support for one SQL dialect, take a look at [wroge/superbasic](https://github.com/wroge/superbasic). To scan rows to types, i recommend [wroge/scan](https://github.com/wroge/scan).

```go
package main

import (
	"fmt"

	"github.com/wroge/esperanto"
	"github.com/wroge/superbasic"
)

func main() {
	// 1. CREATE SCHEMA
	// Compile expressions into a template to keep the SQL code as concise as possible.
	// Consider the differences of the individual dialects by esperanto.Switch.

	create := esperanto.Compile("CREATE TABLE presidents (\n\t?\n)",
		esperanto.Join(",\n\t",
			esperanto.Switch{
				esperanto.Postgres:  superbasic.SQL("nr SERIAL PRIMARY KEY"),
				esperanto.Sqlite:    superbasic.SQL("nr INTEGER PRIMARY KEY AUTOINCREMENT"),
				esperanto.SQLServer: superbasic.SQL("nr INT IDENTITY PRIMARY KEY"),
			},
			esperanto.SQL("first TEXT NOT NULL"),
			esperanto.SQL("last TEXT NOT NULL"),
		),
	)

	fmt.Println(create.ToSQL(esperanto.Postgres))
	// CREATE TABLE presidents (
	//	nr SERIAL PRIMARY KEY,
	//	first TEXT NOT NULL,
	//	last TEXT NOT NULL
	// )

	fmt.Println(create.ToSQL(esperanto.Sqlite))
	// CREATE TABLE presidents (
	//	nr INTEGER PRIMARY KEY AUTOINCREMENT,
	//	first TEXT NOT NULL,
	//	last TEXT NOT NULL
	// )

	fmt.Println(create.ToSQL(esperanto.SQLServer))
	// CREATE TABLE presidents (
	//	nr INT IDENTITY PRIMARY KEY,
	//	first TEXT NOT NULL,
	//	last TEXT NOT NULL
	// )

	// 2. INSERT
	// Sometimes the syntax of each dialect is completely different, so some parts have to be skipped
	// and others inserted in a certain place by esperanto.Switch.
	// Map is a generic map function, that will be removed until better alternatives are available.

	insert := esperanto.Join(" ",
		esperanto.SQL("INSERT INTO presidents (first, last)"),
		esperanto.Switch{
			esperanto.SQLServer: superbasic.SQL("OUTPUT INSERTED.nr"),
		},
		esperanto.Compile("VALUES ?",
			esperanto.Join(", ",
				superbasic.Map(presidents,
					func(president President) esperanto.Expression {
						return esperanto.Values{president.First, president.Last}
					})...,
			),
		),
		esperanto.Switch{
			esperanto.Postgres: superbasic.SQL("RETURNING nr"),
			esperanto.Sqlite:   superbasic.SQL("RETURNING nr"),
		},
	)

	fmt.Println(esperanto.Finalize("$%d", esperanto.Postgres, insert))
	// INSERT INTO presidents (first, last) VALUES ($1, $2), ($3, $4) RETURNING nr [George Washington John Adams]

	fmt.Println(esperanto.Finalize("?", esperanto.Sqlite, insert))
	// INSERT INTO presidents (first, last) VALUES (?, ?), (?, ?) RETURNING nr [George Washington John Adams]

	fmt.Println(esperanto.Finalize("@p%d", esperanto.SQLServer, insert))
	// INSERT INTO presidents (first, last) OUTPUT INSERTED.nr VALUES (@p1, @p2), (@p3, @p4) [George Washington John Adams]

	// 3. QUERY
	// In this section, we create a query that returns JSON rows.
	// Note that the JSON_OBJECT function is not yet implemented in SQL Server 2019.

	equals := esperanto.Switch{
		esperanto.Postgres:  superbasic.SQL("last = ?", "Adams"),
		esperanto.Sqlite:    superbasic.SQL("last = ?", "Adams"),
		esperanto.SQLServer: superbasic.SQL("CONVERT(VARCHAR, last) = ? COLLATE Latin1_General_CS_AS", "Adams"),
	}

	query := esperanto.Compile("SELECT ? FROM (?) AS q",
		esperanto.Switch{
			esperanto.Postgres: superbasic.SQL("JSON_BUILD_OBJECT('nr', q.nr, 'first', q.first, 'last', q.last) AS result"),
			esperanto.Sqlite:   superbasic.SQL("JSON_OBJECT('nr', q.nr, 'first', q.first, 'last', q.last) AS result"),
			// https://docs.microsoft.com/en-us/sql/t-sql/functions/json-object-transact-sql
			esperanto.SQLServer: superbasic.SQL("JSON_OBJECT('nr': q.nr, 'first': q.first, 'last': q.last) AS result"),
		},
		esperanto.Join(" ",
			esperanto.SQL("SELECT nr, first, last FROM presidents"),
			esperanto.If(equals != nil, esperanto.Compile("WHERE ?", equals)),
		),
	)

	fmt.Println(esperanto.Finalize("$%d", esperanto.Postgres, query))
	// SELECT JSON_BUILD_OBJECT('nr', q.nr, 'first', q.first, 'last', q.last) AS result
	// FROM (SELECT nr, first, last FROM presidents WHERE last = $1) AS q [Adams]

	fmt.Println(esperanto.Finalize("?", esperanto.Sqlite, query))
	// SELECT JSON_OBJECT('nr', q.nr, 'first', q.first, 'last', q.last) AS result
	// FROM (SELECT nr, first, last FROM presidents WHERE last = ?) AS q [Adams]

	fmt.Println(esperanto.Finalize("@p%d", esperanto.SQLServer, query))
	// SELECT JSON_OBJECT('nr': q.nr, 'first': q.first, 'last': q.last) AS result
	// FROM (SELECT nr, first, last FROM presidents
	// WHERE CONVERT(VARCHAR, last) = @p1 COLLATE Latin1_General_CS_AS) AS q [Adams]
}

type President struct {
	First string
	Last  string
}

var presidents = []President{
	{"George", "Washington"},
	{"John", "Adams"},
	// {"Thomas", "Jefferson"},
	// {"James", "Madison"},
	// {"James", "Monroe"},
	// {"John Quincy", "Adams"},
	// {"Andrew", "Jackson"},
	// {"Martin", "Van Buren"},
	// {"William Henry", "Harrison"},
	// {"John", "Tyler"},
	// {"James K.", "Polk"},
	// {"Zachary", "Taylor"},
	// {"Millard", "Fillmore"},
	// {"Franklin", "Pierce"},
	// {"James", "Buchanan"},
	// {"Abraham", "Lincoln"},
	// {"Andrew", "Johnson"},
	// {"Ulysses S.", "Grant"},
	// {"Rutherford B.", "Hayes"},
	// {"James A.", "Garfield"},
	// {"Chester A.", "Arthur"},
	// {"Grover", "Cleveland"},
	// {"Benjamin", "Harrison"},
	// {"Grover", "Cleveland"},
	// {"William", "McKinley"},
	// {"Theodore", "Roosevelt"},
	// {"William Howard", "Taft"},
	// {"Woodrow", "Wilson"},
	// {"Warren G.", "Harding"},
	// {"Calvin", "Coolidge"},
	// {"Herbert", "Hoover"},
	// {"Franklin D.", "Roosevelt"},
	// {"Harry S.", "Truman"},
	// {"Dwight D.", "Eisenhower"},
	// {"John F.", "Kennedy"},
	// {"Lyndon B.", "Johnson"},
	// {"Richard", "Nixon"},
	// {"Gerald", "Ford"},
	// {"Jimmy", "Carter"},
	// {"Ronald", "Reagan"},
	// {"George H. W.", "Bush"},
	// {"Bill", "Clinton"},
	// {"George W.", "Bush"},
	// {"Barack", "Obama"},
	// {"Donald", "Trump"},
	// {"Joe", "Biden"},
}
```
