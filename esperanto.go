//nolint:wrapcheck
package esperanto

import (
	"fmt"
	"strings"

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

type Expression interface {
	ToSQL(dialect Dialect) (string, []any, error)
}

type Values []any

func (v Values) ToSQL(dialect Dialect) (string, []any, error) {
	return fmt.Sprintf("(%s)", strings.Repeat(", ?", len(v))[2:]), v, nil
}

func Compile(template string, expressions ...Expression) Compiler {
	return Compiler{Template: template, Expressions: expressions}
}

// Compiler takes a template and compiles a list of expressions into that template.
// The number of placeholders in the template must be exactly equal to the number of expressions.
// If an expression is nil, an ExpressionError is returned.
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
			return "", nil, superbasic.NumberOfArgumentsError{
				SQL:          builder.String(),
				Placeholders: exprIndex,
				Arguments:    len(c.Expressions),
			}
		}

		if c.Expressions[exprIndex] == nil {
			return "", nil, superbasic.ExpressionError{Position: exprIndex}
		}

		builder.WriteString(c.Template[:index])
		c.Template = c.Template[index+1:]

		sql, args, err := c.Expressions[exprIndex].ToSQL(dialect)
		if err != nil {
			return "", nil, err
		}

		builder.WriteString(sql)

		arguments = append(arguments, args...)
	}

	if exprIndex != len(c.Expressions)-1 {
		return "", nil, superbasic.NumberOfArgumentsError{
			SQL:          builder.String(),
			Placeholders: exprIndex,
			Arguments:    len(c.Expressions),
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

// Condition is a boolean switch between two expressions. If an expression is nil, it is skipped.
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
			return "", nil, err
		}

		return sql, args, nil
	}

	if c.Else == nil {
		return "", nil, nil
	}

	sql, args, err := c.Else.ToSQL(dialect)
	if err != nil {
		return "", nil, err
	}

	return sql, args, nil
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
			return "", nil, err
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

// Switch can be used to distinguish between dialects. If a dialect is not found, it is skipped.
type Switch map[Dialect]superbasic.Expression

func (s Switch) ToSQL(dialect Dialect) (string, []any, error) {
	if s == nil {
		return "", nil, nil
	}

	if expr, ok := s[dialect]; ok {
		sql, args, err := expr.ToSQL()
		if err != nil {
			return "", nil, err
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

func (r Raw) ToSQL(dialect Dialect) (string, []any, error) {
	return r.SQL, r.Args, nil
}

// Finalize takes a static placeholder like '?' or a positional placeholder containing '%d'.
// Escaped placeholders ('??') are replaced to '?' when placeholder argument is not '?'.
func Finalize(placeholder string, dialect Dialect, expression Expression) (string, []any, error) {
	sql, args, err := expression.ToSQL(dialect)
	if err != nil {
		return "", nil, err
	}

	var count int

	sql, count = superbasic.Replace(placeholder, sql)

	if count != len(args) {
		return "", nil, superbasic.NumberOfArgumentsError{SQL: sql, Placeholders: count, Arguments: len(args)}
	}

	return sql, args, nil
}
