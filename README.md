# esperanto

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/wroge/esperanto)
[![Go Report Card](https://goreportcard.com/badge/github.com/wroge/esperanto)](https://goreportcard.com/report/github.com/wroge/esperanto)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/wroge/esperanto.svg?style=social)](https://github.com/wroge/esperanto/tags)

esperanto makes it easy to create SQL expressions for multiple dialects. 

```esperanto.Compile``` compiles expressions into an SQL template and thus offers an alternative to conventional query builders. ```Masterminds/squirrel``` expressions and [others](https://github.com/wroge/esperanto/blob/main/esperanto.go#L9) are supported.

```go
package main

import (
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/wroge/esperanto"
)

func main() {
	// 1. CREATE SCHEMA
	// Compile expressions into a template to keep the SQL code as concise as possible.
	// Consider the differences of the individual dialects by esperanto.Switch.

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

	fmt.Println(esperanto.Postgres.Finalize(create))
	// CREATE TABLE presidents (
	//     nr SERIAL PRIMARY KEY,
	//     first TEXT NOT NULL,
	//     last TEXT NOT NULL
	// )

	fmt.Println(esperanto.Sqlite.Finalize(create))
	// CREATE TABLE presidents (
	// 		nr INTEGER PRIMARY KEY AUTOINCREMENT,
	// 		first TEXT NOT NULL,
	// 		last TEXT NOT NULL
	// )

	fmt.Println(esperanto.SQLServer.Finalize(create))
	// CREATE TABLE presidents (
	//     nr INT IDENTITY PRIMARY KEY,
	//     first TEXT NOT NULL,
	//     last TEXT NOT NULL
	// )

	// 2. INSERT
	// Sometimes the syntax of each dialect is completely different, so some parts have to be skipped
	// and others inserted in a certain place by esperanto.Switch.
	// Map is a generic map function, that will be removed until better alternatives are available.

	insert := esperanto.Join(" ",
		esperanto.SQL("INSERT INTO presidents (first, last)"),
		esperanto.Switch{
			esperanto.SQLServer: esperanto.SQL("OUTPUT INSERTED.nr"),
		},
		esperanto.Compile("VALUES ?",
			esperanto.Join(", ",
				esperanto.Map(presidents,
					func(president President) any {
						return esperanto.Values{president.First, president.Last}
					})...,
			),
		),
		esperanto.Switch{
			esperanto.Postgres: esperanto.SQL("RETURNING nr"),
			esperanto.Sqlite:   esperanto.SQL("RETURNING nr"),
		},
	)

	fmt.Println(esperanto.Postgres.Finalize(insert))
	// INSERT INTO presidents (first, last) VALUES ($1, $2), ($3, $4) RETURNING nr [George Washington John Adams]

	fmt.Println(esperanto.Sqlite.Finalize(insert))
	// INSERT INTO presidents (first, last) VALUES (?, ?), (?, ?) RETURNING nr [George Washington John Adams]

	fmt.Println(esperanto.SQLServer.Finalize(insert))
	// INSERT INTO presidents (first, last) OUTPUT INSERTED.nr VALUES (@p1, @p2), (@p3, @p4) [George Washington John Adams]

	// 3. QUERY
	// This section creates a query that returns JSON rows and is therefore supported by any database driver.
	// Note that the JSON_OBJECT function is not yet implemented in SQL Server 2019.

	equals := esperanto.Switch{
		esperanto.Postgres:  esperanto.SQL("last = ?", "Adams"),
		esperanto.Sqlite:    squirrel.Eq{"last": "Adams"},
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

	fmt.Println(esperanto.Postgres.Finalize(query))
	// SELECT JSON_BUILD_OBJECT('nr', q.nr, 'first', q.first, 'last', q.last) AS result
	// FROM (SELECT nr, first, last FROM presidents WHERE last = $1) AS q [Adams]

	fmt.Println(esperanto.Sqlite.Finalize(query))
	// SELECT JSON_OBJECT('nr', q.nr, 'first', q.first, 'last', q.last) AS result
	// FROM (SELECT nr, first, last FROM presidents WHERE last = ?) AS q [Adams]

	fmt.Println(esperanto.SQLServer.Finalize(query))
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
