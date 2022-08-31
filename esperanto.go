package esperanto

import (
	"fmt"
	"strings"
)

//nolint:nonamedreturns
func ToSQL(dialect Dialect, expression any) (sql string, args []any, err error) {
	switch expr := expression.(type) {
	case interface {
		ToSQL(dialect Dialect) (string, []any, error)
	}:
		sql, args, err = expr.ToSQL(dialect)
	case interface {
		ToSQL() (string, []any, error)
	}:
		sql, args, err = expr.ToSQL()
	case interface {
		ToSql() (string, []any, error)
	}:
		sql, args, err = expr.ToSql()
	default:
		return "", nil, Error{Err: ExpressionError{}}
	}

	if err != nil {
		return "", nil, Error{Err: err}
	}

	return sql, args, nil
}

type Error struct {
	Err error
}

func (e Error) Error() string {
	if e.Err != nil {
		//nolint:errorlint
		if err, ok := e.Err.(Error); ok {
			return err.Error()
		}

		return fmt.Sprintf("esperanto.Error: %s", e.Err.Error())
	}

	return "esperanto.Error"
}

func (e Error) Unwrap() error {
	return e.Err
}

func (e Error) ToSQL() (string, []any, error) {
	return "", nil, e
}

func (e Error) Scan(dest ...any) error {
	return e
}

type ExpressionError struct{}

func (e ExpressionError) Error() string {
	return "expression is invalid: Take a look at esperanto.ToSQL"
}

// NumberOfArgumentsError is returned if arguments doesn't match the number of placeholders.
type NumberOfArgumentsError struct {
	SQL                     string
	Placeholders, Arguments int
}

func (e NumberOfArgumentsError) Error() string {
	argument := "argument"

	if e.Arguments > 1 {
		argument += "s"
	}

	placeholder := "placeholder"

	if e.Placeholders > 1 {
		placeholder += "s"
	}

	return fmt.Sprintf("%d %s and %d %s in '%s'",
		e.Placeholders, placeholder, e.Arguments, argument, e.SQL)
}

type Placeholder interface {
	Placeholder(position int) string
}

type StaticPlaceholder string

func (s StaticPlaceholder) Placeholder(position int) string {
	return string(s)
}

type PositionalPlaceholder string

func (p PositionalPlaceholder) Placeholder(position int) string {
	return fmt.Sprintf(string(p), position)
}

const (
	Question StaticPlaceholder     = "?"
	Dollar   PositionalPlaceholder = "$%d"
	Colon    PositionalPlaceholder = ":%d"
	AtP      PositionalPlaceholder = "@p%d"
)

type Dialect string

const (
	MySQL     Dialect = "mysql"
	Sqlite    Dialect = "sqlite"
	Postgres  Dialect = "postgres"
	Oracle    Dialect = "oracle"
	SQLServer Dialect = "sqlserver"
)

func (d Dialect) Finalize(expression any) (string, []any, error) {
	switch d {
	case MySQL, Sqlite:
		return Finalize(Question, d, expression)
	case Postgres:
		return Finalize(Dollar, d, expression)
	case Oracle:
		return Finalize(Colon, d, expression)
	case SQLServer:
		return Finalize(AtP, d, expression)
	default:
		return Finalize(Question, d, expression)
	}
}

func Map[From any, To any](from []From, mapper func(From) To) []To {
	toSlice := make([]To, len(from))

	for i, f := range from {
		toSlice[i] = mapper(f)
	}

	return toSlice
}

type Values []any

func (v Values) ToSQL() (string, []any, error) {
	return fmt.Sprintf("(%s)", strings.Repeat(", ?", len(v))[2:]), v, nil
}

func Compile(template string, expressions ...any) Compiler {
	return Compiler{Template: template, Expressions: expressions}
}

type Compiler struct {
	Template    string
	Expressions []any
}

func (c Compiler) ToSQL(dialect Dialect) (string, []any, error) {
	builder := &strings.Builder{}
	arguments := make([]any, 0, len(c.Expressions))

	exprIndex := -1

	for {
		index := strings.IndexRune(c.Template, '?')
		if index < 0 {
			builder.WriteString(c.Template)

			break
		}

		if index < len(c.Template)-1 && c.Template[index+1] == '?' {
			builder.WriteString(c.Template[:index+2])
			c.Template = c.Template[index+2:]

			continue
		}

		exprIndex++

		if exprIndex >= len(c.Expressions) {
			return "", nil, Error{
				Err: NumberOfArgumentsError{
					SQL:          builder.String(),
					Placeholders: exprIndex,
					Arguments:    len(c.Expressions),
				},
			}
		}

		if c.Expressions[exprIndex] == nil {
			return "", nil, Error{Err: ExpressionError{}}
		}

		builder.WriteString(c.Template[:index])
		c.Template = c.Template[index+1:]

		sql, args, err := ToSQL(dialect, c.Expressions[exprIndex])
		if err != nil {
			return "", nil, Error{Err: err}
		}

		builder.WriteString(sql)

		arguments = append(arguments, args...)
	}

	if exprIndex != len(c.Expressions)-1 {
		return "", nil, Error{
			Err: NumberOfArgumentsError{
				SQL:          builder.String(),
				Placeholders: exprIndex,
				Arguments:    len(c.Expressions),
			},
		}
	}

	return builder.String(), arguments, nil
}

func If(condition bool, then any) Condition {
	return Condition{
		If:   condition,
		Then: then,
		Else: nil,
	}
}

func IfElse(condition bool, then any, els any) Condition {
	return Condition{
		If:   condition,
		Then: then,
		Else: els,
	}
}

type Condition struct {
	If   bool
	Then any
	Else any
}

func (c Condition) ToSQL(dialect Dialect) (string, []any, error) {
	if c.If {
		if c.Then == nil {
			return "", nil, nil
		}

		sql, args, err := ToSQL(dialect, c.Then)
		if err != nil {
			return "", nil, Error{Err: err}
		}

		return sql, args, nil
	}

	if c.Else == nil {
		return "", nil, nil
	}

	sql, args, err := ToSQL(dialect, c.Else)
	if err != nil {
		return "", nil, Error{Err: err}
	}

	return sql, args, nil
}

func Join(sep string, expressions ...any) Joiner {
	return Joiner{Sep: sep, Expressions: expressions}
}

type Joiner struct {
	Sep         string
	Expressions []any
}

func (j Joiner) ToSQL(dialect Dialect) (string, []any, error) {
	builder := &strings.Builder{}
	arguments := make([]any, 0, len(j.Expressions))

	for index, expression := range j.Expressions {
		if expression == nil {
			continue
		}

		sql, args, err := ToSQL(dialect, expression)
		if err != nil {
			return "", nil, Error{Err: err}
		}

		if sql == "" {
			continue
		}

		if index != 0 {
			builder.WriteString(j.Sep)
		}

		builder.WriteString(sql)

		arguments = append(arguments, args...)
	}

	return builder.String(), arguments, nil
}

type Switch map[Dialect]any

func (s Switch) ToSQL(dialect Dialect) (string, []any, error) {
	if s == nil {
		return "", nil, nil
	}

	if expr, ok := s[dialect]; ok {
		sql, args, err := ToSQL(dialect, expr)
		if err != nil {
			return "", nil, Error{Err: err}
		}

		return sql, args, nil
	}

	return "", nil, nil
}

func SQL(sql string, args ...any) Raw {
	return Raw{SQL: sql, Args: args}
}

type Raw struct {
	SQL  string
	Args []any
}

func (r Raw) ToSQL() (string, []any, error) {
	return r.SQL, r.Args, nil
}

func Finalize(placeholder Placeholder, dialect Dialect, expression any) (string, []any, error) {
	sql, args, err := ToSQL(dialect, expression)
	if err != nil {
		return "", nil, Error{Err: err}
	}

	var count int

	sql, count, err = Replace(placeholder, sql)
	if err != nil {
		return "", nil, Error{Err: err}
	}

	if count != len(args) {
		return "", nil, Error{Err: NumberOfArgumentsError{SQL: sql, Placeholders: count, Arguments: len(args)}}
	}

	return sql, args, nil
}

func Replace(placeholder Placeholder, sql string) (string, int, error) {
	build := &strings.Builder{}
	count := 0

	if placeholder == nil {
		placeholder = Question
	}

	question := "?"

	if placeholder.Placeholder(1) == "?" {
		question = "??"
	}

	for {
		index := strings.IndexRune(sql, '?')
		if index < 0 {
			build.WriteString(sql)

			break
		}

		if index < len(sql)-1 && sql[index+1] == '?' {
			build.WriteString(sql[:index] + question)
			sql = sql[index+2:]

			continue
		}

		count++

		build.WriteString(fmt.Sprintf("%s%s", sql[:index], placeholder.Placeholder(count)))
		sql = sql[index+1:]
	}

	return build.String(), count, nil
}
