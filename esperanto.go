package esperanto

import (
	"fmt"
	"strings"
)

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

func (e Error) ToSQL(dialect Dialect) (string, []any, error) {
	return "", nil, e
}

func (e Error) Scan(dest ...any) error {
	return e
}

type ExpressionError struct {
	Location string
}

func (e ExpressionError) Error() string {
	if e.Location != "" {
		return fmt.Sprintf("expression is nil at '%s', ", e.Location)
	}

	return "expression is nil"
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

type DialectError struct {
	Dialect  Dialect
	Location string
}

func (e DialectError) Error() string {
	dialect := "nil"

	if e.Dialect != nil {
		dialect = e.Dialect.String()
	}

	if e.Location != "" {
		return fmt.Sprintf("dialect '%s' not allowed at '%s'", dialect, e.Location)
	}

	return fmt.Sprintf("dialect '%s' not allowed", dialect)
}

type Dialect interface {
	fmt.Stringer
	Placeholder(position int) string
	Question() string
}

type knownDialect string

const (
	MySQL     knownDialect = "mysql"
	Sqlite    knownDialect = "sqlite"
	Postgres  knownDialect = "postgres"
	Oracle    knownDialect = "oracle"
	SQLServer knownDialect = "sqlserver"
)

func (d knownDialect) String() string {
	return string(d)
}

func (d knownDialect) Placeholder(position int) string {
	switch d {
	case MySQL, Sqlite:
		return "?"
	case Postgres:
		return fmt.Sprintf("$%d", position)
	case Oracle:
		return fmt.Sprintf(":%d", position)
	case SQLServer:
		return fmt.Sprintf("@p%d", position)
	}

	panic("unknown known dialect")
}

func (d knownDialect) Question() string {
	switch d {
	case MySQL, Sqlite:
		return "??"
	case Postgres, Oracle, SQLServer:
		return "?"
	}

	panic("unknown known dialect")
}

type Expression interface {
	ToSQL(dialect Dialect) (string, []any, error)
}

func Map[From any, To any](from []From, mapper func(From) To) []To {
	toSlice := make([]To, len(from))

	for i, f := range from {
		toSlice[i] = mapper(f)
	}

	return toSlice
}

func Value(a any) Raw {
	return Raw{SQL: "?", Args: []any{a}}
}

type Values []any

func (v Values) ToSQL(dialect Dialect) (string, []any, error) {
	return fmt.Sprintf("(%s)", strings.Repeat(", ?", len(v))[2:]), v, nil
}

func Compile(template string, expressions ...Expression) Compiler {
	return Compiler{Template: template, Expressions: expressions}
}

type Compiler struct {
	Template    string
	Expressions []Expression
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
			return "", nil, Error{Err: ExpressionError{Location: "esperanto.Compiler"}}
		}

		builder.WriteString(c.Template[:index])
		c.Template = c.Template[index+1:]

		sql, args, err := c.Expressions[exprIndex].ToSQL(dialect)
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

func If(condition bool, then Expression) Condition {
	return Condition{
		If:   condition,
		Then: then,
		Else: nil,
	}
}

func IfElse(condition bool, then Expression, els Expression) Condition {
	return Condition{
		If:   condition,
		Then: then,
		Else: els,
	}
}

type Condition struct {
	If   bool
	Then Expression
	Else Expression
}

func (c Condition) ToSQL(dialect Dialect) (string, []any, error) {
	if c.If {
		if c.Then == nil {
			return "", nil, nil
		}

		sql, args, err := c.Then.ToSQL(dialect)
		if err != nil {
			return "", nil, Error{Err: err}
		}

		return sql, args, nil
	}

	if c.Else == nil {
		return "", nil, nil
	}

	sql, args, err := c.Else.ToSQL(dialect)
	if err != nil {
		return "", nil, Error{Err: err}
	}

	return sql, args, nil
}

func Append(expressions ...Expression) Joiner {
	return Joiner{Sep: "", Expressions: expressions}
}

func Join(sep string, expressions ...Expression) Joiner {
	return Joiner{Sep: sep, Expressions: expressions}
}

type Joiner struct {
	Sep         string
	Expressions []Expression
}

func (j Joiner) ToSQL(dialect Dialect) (string, []any, error) {
	builder := &strings.Builder{}
	arguments := make([]any, 0, len(j.Expressions))

	for index, expression := range j.Expressions {
		if expression == nil {
			continue
		}

		sql, args, err := expression.ToSQL(dialect)
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

type Func func(Dialect) Expression

func (f Func) ToSQL(dialect Dialect) (string, []any, error) {
	sql, args, err := f(dialect).ToSQL(dialect)
	if err != nil {
		return "", nil, Error{Err: err}
	}

	return sql, args, nil
}

type Switch map[Dialect]Expression

func (s Switch) ToSQL(dialect Dialect) (string, []any, error) {
	if s == nil {
		return "", nil, Error{Err: DialectError{Dialect: dialect, Location: "esperanto.Switch"}}
	}

	if expr, ok := s[dialect]; ok {
		sql, args, err := expr.ToSQL(dialect)
		if err != nil {
			return "", nil, Error{Err: err}
		}

		return sql, args, nil
	}

	return "", nil, Error{Err: DialectError{Dialect: dialect, Location: "esperanto.Switch"}}
}

func Skip() Raw {
	return Raw{SQL: "", Args: nil}
}

func SQL(sql string, args ...any) Raw {
	return Raw{SQL: sql, Args: args}
}

type Raw struct {
	SQL  string
	Args []any
}

func (r Raw) ToSQL(dialect Dialect) (string, []any, error) {
	return r.SQL, r.Args, nil
}

func Finalize(dialect Dialect, expression Expression) (string, []any, error) {
	if expression == nil {
		return "", nil, Error{Err: ExpressionError{Location: "esperanto.Finalize"}}
	}

	sql, args, err := expression.ToSQL(dialect)
	if err != nil {
		return "", nil, Error{Err: err}
	}

	var count int

	sql, count, err = replace(dialect, sql)
	if err != nil {
		return "", nil, err
	}

	if count != len(args) {
		return "", nil, NumberOfArgumentsError{SQL: sql, Placeholders: count, Arguments: len(args)}
	}

	return sql, args, nil
}

func replace(dialect Dialect, sql string) (string, int, error) {
	build := &strings.Builder{}
	count := 0

	for {
		index := strings.IndexRune(sql, '?')
		if index < 0 {
			build.WriteString(sql)

			break
		}

		if index < len(sql)-1 && sql[index+1] == '?' {
			build.WriteString(sql[:index] + dialect.Question())
			sql = sql[index+2:]

			continue
		}

		count++

		build.WriteString(fmt.Sprintf("%s%s", sql[:index], dialect.Placeholder(count)))
		sql = sql[index+1:]
	}

	return build.String(), count, nil
}
