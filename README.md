# esperanto

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/wroge/esperanto)
[![Go Report Card](https://goreportcard.com/badge/github.com/wroge/esperanto)](https://goreportcard.com/report/github.com/wroge/esperanto)
![golangci-lint](https://github.com/wroge/esperanto/workflows/golangci-lint/badge.svg)
[![tippin.me](https://badgen.net/badge/%E2%9A%A1%EF%B8%8Ftippin.me/@_wroge/F0918E)](https://tippin.me/@_wroge)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/wroge/esperanto.svg?style=social)](https://github.com/wroge/esperanto/tags)

esperanto is a database access layer. It is based upon ...

- [wroge/superbasic](https://github.com/wroge/superbasic)
- [wroge/scan](https://github.com/wroge/scan)

This module can help you better organize your queries, especially if you need support for multiple dialects.

```go
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/wroge/esperanto"
	"github.com/wroge/scan"
	"github.com/wroge/superbasic"

	_ "modernc.org/sqlite"
)

func main() {
	ctx := context.Background()

	stdDB, err := sql.Open("sqlite", ":memory:?_pragma=foreign_keys(1)")
	if err != nil {
		panic(err)
	}

	db := esperanto.StdDB{
		Placeholder: "?",
		DB:          stdDB,
	}

	err = esperanto.Exec(ctx, db, esperanto.Sqlite, DropPostAuthors, DropPosts, DropAuthors, CreateAuthors, CreatePosts, CreatePostAuthors)
	if err != nil {
		panic(err)
	}

	authorEntities, err := esperanto.Query(ctx, db, esperanto.Sqlite, AuthorInsert, []string{"Jim", "Tim", "Tom"})
	if err != nil {
		panic(err)
	}

	fmt.Println(authorEntities)
	// [{1 Jim} {2 Tim} {3 Tom}]

	postEntities, err := esperanto.QueryAndExec(ctx, db, esperanto.Sqlite, PostInsert, []InsertPostOptions{
		{
			Title:   "Post One",
			Authors: []int64{1, 2},
		},
	}, PostAuthorsInsert)
	if err != nil {
		panic(err)
	}

	fmt.Println(postEntities)
	// [{1 Post One}]

	postTwo, err := esperanto.QueryAndExecOne(ctx, db, esperanto.Sqlite, PostInsertOne, InsertPostOptions{
		Title:   "Post Two",
		Authors: []int64{2, 3},
	}, PostAuthorsInsertOne)
	if err != nil {
		panic(err)
	}

	fmt.Println(postTwo)
	// {2 Post Two []}

	authors, err := esperanto.Query(ctx, db, esperanto.Sqlite, AuthorQuery, QueryAuthorOptions{})
	if err != nil {
		panic(err)
	}

	fmt.Println(authors)
	// [{1 Jim [{1 Post One}]} {2 Tim [{1 Post One} {2 Post Two}]} {3 Tom [{2 Post Two}]}]

	posts, err := esperanto.Query(ctx, db, esperanto.Sqlite, PostQuery, QueryPostOptions{})
	if err != nil {
		panic(err)
	}

	fmt.Println(posts)
	// [{1 Post One [{1 Jim} {2 Tim}]} {2 Post Two [{2 Tim} {3 Tom}]}]
}

type PostEntity struct {
	ID    int64
	Title string
}

type Post struct {
	ID      int64
	Title   string
	Authors []AuthorEntity
}

type AuthorEntity struct {
	ID   int64
	Name string
}

type Author struct {
	ID    int64
	Name  string
	Posts []PostEntity
}

func DropAuthors(dialect esperanto.Dialect) superbasic.Expression {
	return superbasic.SQL("DROP TABLE IF EXISTS authors")
}

func CreateAuthors(dialect esperanto.Dialect) superbasic.Expression {
	return superbasic.Compile("CREATE TABLE IF NOT EXISTS authors (\n\t?\n)",
		superbasic.Join(",\n\t",
			superbasic.Switch(dialect,
				superbasic.Case(esperanto.Postgres, superbasic.SQL("id SERIAL PRIMARY KEY")),
				superbasic.Case(esperanto.Sqlite, superbasic.SQL("id INTEGER PRIMARY KEY AUTOINCREMENT")),
			),
			superbasic.SQL("name TEXT NOT NULL"),
		),
	)
}

type QueryAuthorOptions struct{}

func AuthorQuery(dialect esperanto.Dialect, options QueryAuthorOptions) (superbasic.Expression, []scan.Column[Author]) {
	return superbasic.Compile(`
	SELECT authors.id, authors.name, ? AS posts
	FROM authors
	LEFT JOIN post_authors ON post_authors.author_id = authors.id
	LEFT JOIN posts ON posts.id = post_authors.post_id
	GROUP BY authors.id, authors.name`,
			superbasic.Switch(dialect,
				superbasic.Case(esperanto.Postgres, superbasic.SQL("JSON_AGG(JSON_BUILD_OBJECT('id', posts.id, 'title', posts.title))")),
				superbasic.Case(esperanto.Sqlite, superbasic.SQL(`CASE WHEN posts.id IS NULL THEN '[]' 
				ELSE JSON_GROUP_ARRAY(JSON_OBJECT('id', posts.id, 'title', posts.title)) END`)),
			),
		),
		[]scan.Column[Author]{
			scan.Any(func(author *Author, id int64) { author.ID = id }),
			scan.Any(func(author *Author, name string) { author.Name = name }),
			scan.AnyErr(func(author *Author, posts []byte) error { return json.Unmarshal(posts, &author.Posts) }),
		}
}

func AuthorInsert(dialect esperanto.Dialect, names []string) (superbasic.Expression, []scan.Column[AuthorEntity]) {
	return superbasic.Compile("INSERT INTO authors (name) VALUES ? RETURNING id, name", superbasic.Join(", ", superbasic.Map(names, func(_ int, name string) superbasic.Expression {
			return superbasic.Values{name}
		})...)),
		[]scan.Column[AuthorEntity]{
			scan.Any(func(author *AuthorEntity, id int64) { author.ID = id }),
			scan.Any(func(author *AuthorEntity, name string) { author.Name = name }),
		}
}

func AuthorInsertOne(dialect esperanto.Dialect, name string) (superbasic.Expression, []scan.Column[AuthorEntity]) {
	return superbasic.SQL("INSERT INTO authors (name) VALUES (?) RETURNING id, name", name),
		[]scan.Column[AuthorEntity]{
			scan.Any(func(author *AuthorEntity, id int64) { author.ID = id }),
			scan.Any(func(author *AuthorEntity, name string) { author.Name = name }),
		}
}

func DropPosts(dialect esperanto.Dialect) superbasic.Expression {
	return superbasic.SQL("DROP TABLE IF EXISTS posts")
}

func DropPostAuthors(dialect esperanto.Dialect) superbasic.Expression {
	return superbasic.SQL("DROP TABLE IF EXISTS post_authors")
}

func CreatePosts(dialect esperanto.Dialect) superbasic.Expression {
	return superbasic.Compile("CREATE TABLE IF NOT EXISTS posts (\n\t?\n)",
		superbasic.Join(",\n\t",
			superbasic.Switch(dialect,
				superbasic.Case(esperanto.Postgres, superbasic.SQL("id SERIAL PRIMARY KEY")),
				superbasic.Case(esperanto.Sqlite, superbasic.SQL("id INTEGER PRIMARY KEY AUTOINCREMENT")),
			),
			superbasic.SQL("title TEXT NOT NULL"),
		))
}

func CreatePostAuthors(dialect esperanto.Dialect) superbasic.Expression {
	return superbasic.Compile("CREATE TABLE IF NOT EXISTS post_authors (\n\t?\n)",
		superbasic.Join(",\n\t",
			superbasic.SQL("post_id INTEGER REFERENCES posts (id) ON DELETE CASCADE"),
			superbasic.SQL("author_id INTEGER REFERENCES authors (id) ON DELETE RESTRICT"),
			superbasic.SQL("PRIMARY KEY (post_id, author_id)"),
		),
	)
}

type QueryPostOptions struct{}

func PostQuery(dialect esperanto.Dialect, options QueryPostOptions) (superbasic.Expression, []scan.Column[Post]) {
	return superbasic.Compile(`
	SELECT posts.id, posts.title, ? AS authors
	FROM posts
	LEFT JOIN post_authors ON post_authors.post_id = posts.id
	LEFT JOIN authors ON authors.id = post_authors.author_id
	GROUP BY posts.id, posts.title`,
			superbasic.Switch(dialect,
				superbasic.Case(esperanto.Postgres, superbasic.SQL("JSON_AGG(JSON_BUILD_OBJECT('id', authors.id, 'name', authors.name))")),
				superbasic.Case(esperanto.Sqlite, superbasic.SQL(`CASE WHEN authors.id IS NULL THEN '[]' 
		ELSE JSON_GROUP_ARRAY(JSON_OBJECT('id', authors.id, 'name', authors.name)) END`)),
			)),
		[]scan.Column[Post]{
			scan.Any(func(post *Post, id int64) { post.ID = id }),
			scan.Any(func(post *Post, title string) { post.Title = title }),
			scan.AnyErr(func(post *Post, authors []byte) error { return json.Unmarshal(authors, &post.Authors) }),
		}
}

type InsertPostOptions struct {
	Title   string
	Authors []int64
}

func PostInsert(dialect esperanto.Dialect, options []InsertPostOptions) (superbasic.Expression, []scan.Column[PostEntity]) {
	return superbasic.Compile("INSERT INTO posts (title) VALUES ? RETURNING id, title", superbasic.Join(", ", superbasic.Map(options, func(_ int, option InsertPostOptions) superbasic.Expression {
			return superbasic.Values{option.Title}
		})...)),
		[]scan.Column[PostEntity]{
			scan.Any(func(post *PostEntity, id int64) { post.ID = id }),
			scan.Any(func(post *PostEntity, title string) { post.Title = title }),
		}
}

func PostAuthorsInsert(dialect esperanto.Dialect, options []InsertPostOptions, entities []PostEntity) superbasic.Expression {
	return superbasic.Compile("INSERT INTO post_authors (post_id, author_id) VALUES ?",
		superbasic.Join(", ", superbasic.Map(entities, func(index int, entity PostEntity) superbasic.Expression {
			return superbasic.Join(", ", superbasic.Map(options[index].Authors, func(_ int, author int64) superbasic.Expression {
				return superbasic.Values{entity.ID, author}
			})...)
		})...),
	)
}

func PostInsertOne(dialect esperanto.Dialect, options InsertPostOptions) (superbasic.Expression, []scan.Column[Post]) {
	return superbasic.SQL("INSERT INTO posts (title) VALUES (?) RETURNING id, title", options.Title),
		[]scan.Column[Post]{
			scan.Any(func(post *Post, id int64) { post.ID = id }),
			scan.Any(func(post *Post, title string) { post.Title = title }),
		}
}

func PostAuthorsInsertOne(dialect esperanto.Dialect, insert InsertPostOptions, post Post) superbasic.Expression {
	return superbasic.Compile("INSERT INTO post_authors (post_id, author_id) VALUES ?",
		superbasic.Join(", ", superbasic.Map(insert.Authors, func(_ int, author int64) superbasic.Expression {
			return superbasic.Values{post.ID, author}
		})...),
	)
}
```
