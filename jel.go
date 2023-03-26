/*
Overview

"JSON Expession Language". Expresses a whitelisted subset of SQL with simple
JSON structures. Transcodes JSON queries to SQL.

Example query. See below for more code examples.

	["and",
		["=", "someField", 12345678],
		["or",
			"anotherField",
			["<", "inner.otherField", ["inner.otherField", "9999-01-01T00:00:00Z"]]
		]
	]

Language structure

Expressions are Lisp-style, using nested lists to express "calls". This syntax
is used for all SQL operations. Binary infix operators are considered
variadic.

Lists are used for calls and casts. The first element must be a string. It may
be one of the whitelisted operators or functions, listed in `SqlOps`. If not,
it must be a field name or a dot-separated field path. Calls are arbitrarily
nestable.

	["and", true, ["or", true, ["and", true, false]]]

	["<=", 10, 20]

	["=", "someField", "otherField"]

	["and",
		["=", "someField", "otherField"],
		["<=", "dateField", ["dateField", "9999-01-01T00:00:00Z"]]
	]

Transcoding from JSON to SQL is done by consulting two things: the built-in
whitelist of SQL operations (`SqlOps`, shared), and a struct type provided to
that particular decoder. The struct serves as a whitelist of available
identifiers, and allows to determine value types via casting.

Casting allows to decode arbitrary JSON directly into the corresponding Go type:

	["someDateField", "9999-01-01T00:00:00Z"]

	["someGeoField", {"lng": 10, "lat": 20}]

Such decoded values are substituted with ordinal parameters such as $1, and
appended to the slice of arguments (see below).

A string not in a call position and not inside a cast is interpreted as an
identifier: field name or nested field path, dot-separated. It must be found on
the reference struct, otherwise transcoding fails with an error.

	"someField"

	"outerField.innerField"

Literal numbers, booleans, and nulls that occur outside of casts are decoded
into their Go equivalents. Like casts, they're substituted with ordinal parameters
and appended to the slice of arguments. See `Expr`.

Consulting a struct

JSON queries are transcoded against a struct, by matching fields tagged with
`json` against fields tagged with `db`. Literal values are JSON-decoded into
the types of the corresponding struct fields.

	type Input struct {
		FieldOne string `json:"fieldOne" db:"field_one"`
		FIeldTwo struct {
			FieldThree *time.Time `json:"fieldThree" db:"field_three"`
		} `json:"fieldTwo" db:"field_two"`
	}

	const src = `
		["and",
			["=", "fieldOne", ["fieldOne", "literal string"]],
			["<", "fieldTwo.fieldThree", ["fieldTwo.fieldThree", "9999-01-01T00:00:00Z"]]
		]
	`

	expr := Expr{Text: src, Type: reflect.TypeOf((*Input)(nil)).Elem()}
	text, args := expr.AppendExpr(nil, nil)

The result is roughly equivalent to the following (formatted for clarity):

	text := []byte(`
		"field_one" = 'literal string'
		and
		("field_two")."field_three" < '9999-01-01T00:00:00Z'
	`)
	args := []interface{}{"literal string", time.Time("9999-01-01T00:00:00Z")}
*/
package jel

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/mitranim/sqlb"
)

/*
Whitelist of allowed SQL operations. Also describes how to transform standard
Lisp-style calls into SQL expressions (prefix, infix, etc.).
*/
var SqlOps = map[string]SqlOpSyntax{
	"and":                  SqlInfix,
	"or":                   SqlInfix,
	"not":                  SqlPrefix,
	"is null":              SqlPostfix,
	"is not null":          SqlPostfix,
	"is true":              SqlPostfix,
	"is not true":          SqlPostfix,
	"is false":             SqlPostfix,
	"is not false":         SqlPostfix,
	"is unknown":           SqlPostfix,
	"is not unknown":       SqlPostfix,
	"is distinct from":     SqlInfix,
	"is not distinct from": SqlInfix,
	"=":                    SqlInfix,
	"~":                    SqlInfix,
	"~*":                   SqlInfix,
	"~=":                   SqlInfix,
	"<>":                   SqlInfix,
	"<":                    SqlInfix,
	">":                    SqlInfix,
	">=":                   SqlInfix,
	"<=":                   SqlInfix,
	"@@":                   SqlInfix,
	"any":                  SqlAny,
	"between":              SqlBetween,
}

/*
Describes the syntax used for a specific SQL expression. Allows us to convert
Lisp-style "calls" into SQL-style operations that use prefix, infix, etc.
*/
type SqlOpSyntax byte

const (
	SqlPrefix SqlOpSyntax = iota + 1
	SqlPostfix
	SqlInfix
	SqlFunc
	SqlAny
	SqlBetween
)

/*
Shortcut for instantiating `Expr` with the type of the given value.
The input is used only as a type carrier.
*/
func ExprFor(val interface{}) Expr {
	return Expr{Type: elemTypeOf(val)}
}

/*
Shortcut for instantiating a boolean-style `Expr` with the type of the given value.
The input is used only as a type carrier.
*/
func Bool(val interface{}) Expr {
	return Expr{Type: elemTypeOf(val), IsBool: true}
}

/*
Tool for transcoding JEL into SQL. Implements `sqlb.Expr`. Can be transparently
used as a sub-expression in other `sqlb` expressions.
*/
type Expr struct {
	Text   string
	Type   reflect.Type
	IsBool bool
}

var _ = sqlb.Expr(Expr{})

// Stores the input for future use in `.AppendExpr`. Input must be valid JSON.
func (self *Expr) UnmarshalText(val []byte) error {
	self.Text = bytesToMutableString(val)
	return nil
}

// Stores the input for future use in `.AppendExpr`. Input must be valid JSON.
func (self *Expr) UnmarshalJSON(val []byte) error {
	self.Text = bytesToStringAlloc(val)
	return nil
}

/*
Implement `sqlb.Expr`, allowing this to be used as a sub-expression in queries
built with "github.com/mitranim/sqlb". If `.IsBool == true`, this will generate
a valid boolean expression, falling back on "true" if the expression is empty.
*/
func (self Expr) AppendExpr(text []byte, args []interface{}) ([]byte, []interface{}) {
	bui := sqlb.Bui{Text: text, Args: args}

	if self.IsBool && len(self.Text) == 0 {
		bui.Str(`true`)
	} else {
		self.decode(&bui, stringToBytesUnsafe(self.Text))
	}

	return bui.Get()
}

// Implement a hidden interface supported by some libraries, sometimes allowing
// more efficient text encoding.
func (self Expr) AppendTo(text []byte) []byte { return exprAppend(&self, text) }

// Implement the `fmt.Stringer` interface for debug purposes.
func (self Expr) String() string { return exprString(&self) }

func (self *Expr) decode(bui *sqlb.Bui, input []byte) {
	if isJsonDict(input) {
		panic(fmt.Errorf(`[jel] unexpected dict in input: %q`, input))
	} else if isJsonList(input) {
		self.decodeList(bui, input)
	} else if isJsonString(input) {
		self.decodeString(bui, input)
	} else {
		self.decodeAny(bui, input)
	}
}

func (self *Expr) decodeList(bui *sqlb.Bui, input []byte) {
	var list []json.RawMessage
	err := json.Unmarshal(input, &list)
	if err != nil {
		panic(fmt.Errorf(`[jel] failed to unmarshal as JSON list: %w`, err))
	}

	if !(len(list) > 0) {
		panic(fmt.Errorf(`[jel] lists must have at least one element, found empty list`))
	}

	head, args := list[0], list[1:]
	if !isJsonString(head) {
		panic(fmt.Errorf(`[jel] first list element must be a string, found %q`, head))
	}

	var name string
	err = json.Unmarshal(head, &name)
	if err != nil {
		panic(fmt.Errorf(`[jel] failed to unmarshal JSON list head as string: %w`, err))
	}

	switch SqlOps[name] {
	case SqlPrefix:
		self.decodeOpPrefix(bui, name, args)
	case SqlPostfix:
		self.decodeOpPostfix(bui, name, args)
	case SqlInfix:
		self.decodeOpInfix(bui, name, args)
	case SqlFunc:
		self.decodeOpFunc(bui, name, args)
	case SqlAny:
		self.decodeOpAny(bui, name, args)
	case SqlBetween:
		self.decodeOpBetween(bui, name, args)
	default:
		self.decodeCast(bui, name, args)
	}
}

func (self *Expr) decodeOpPrefix(bui *sqlb.Bui, name string, args []json.RawMessage) {
	if len(args) != 1 {
		panic(fmt.Errorf(`[jel] prefix operation %q must have exactly 1 argument, found %v`, name, len(args)))
	}

	openParen(bui)
	bui.Str(name)
	self.decode(bui, args[0])
	closeParen(bui)
}

func (self *Expr) decodeOpPostfix(bui *sqlb.Bui, name string, args []json.RawMessage) {
	if len(args) != 1 {
		panic(fmt.Errorf(`[jel] postfix operation %q must have exactly 1 argument, found %v`, name, len(args)))
	}

	openParen(bui)
	self.decode(bui, args[0])
	bui.Str(name)
	closeParen(bui)
}

func (self *Expr) decodeOpInfix(bui *sqlb.Bui, name string, args []json.RawMessage) {
	if !(len(args) >= 2) {
		panic(fmt.Errorf(`[jel] infix operation %q must have at least 2 arguments, found %v`, name, len(args)))
	}

	openParen(bui)
	for i, arg := range args {
		if i > 0 {
			bui.Str(name)
		}
		self.decode(bui, arg)
	}
	closeParen(bui)
}

func (self *Expr) decodeOpFunc(bui *sqlb.Bui, name string, args []json.RawMessage) {
	bui.Str(name)
	openParen(bui)
	for i, arg := range args {
		if i > 0 {
			bui.Str(`,`)
		}
		self.decode(bui, arg)
	}
	closeParen(bui)
}

func (self *Expr) decodeOpAny(bui *sqlb.Bui, name string, args []json.RawMessage) {
	if len(args) != 2 {
		panic(fmt.Errorf(`[jel] operation %q must have exactly 2 arguments, found %v`, name, len(args)))
	}

	openParen(bui)
	self.decode(bui, args[0])
	bui.Str(`=`)
	bui.Str(name)
	openParen(bui)
	self.decode(bui, args[1])
	closeParen(bui)
	closeParen(bui)
}

func (self *Expr) decodeOpBetween(bui *sqlb.Bui, name string, args []json.RawMessage) {
	if len(args) != 3 {
		panic(fmt.Errorf(`[jel] operation %q must have exactly 3 arguments, found %v`, name, len(args)))
	}

	openParen(bui)
	self.decode(bui, args[0])
	bui.Str(`between`)
	self.decode(bui, args[1])
	bui.Str(`and`)
	self.decode(bui, args[2])
	closeParen(bui)
}

func (self *Expr) decodeCast(bui *sqlb.Bui, name string, args []json.RawMessage) {
	if len(args) != 1 {
		panic(fmt.Errorf(`[jel] cast into %q must have exactly 1 argument, found %v`, name, len(args)))
	}

	sfield, _, err := fieldByJsonPath(self.Type, name)
	try(err)

	rval := reflect.New(sfield.Type)
	try(json.Unmarshal(args[0], rval.Interface()))

	bui.Param(bui.Arg(rval.Elem().Interface()))
}

func (self *Expr) decodeString(bui *sqlb.Bui, input []byte) {
	var str string
	try(json.Unmarshal(input, &str))

	_, path, err := fieldByJsonPath(self.Type, str)
	try(err)

	bui.Set(sqlb.Path(path).AppendExpr(bui.Get()))
}

// Should be used only for numbers, bools, nulls.
// TODO: unmarshal integers into `int64` rather than `float64`.
func (self *Expr) decodeAny(bui *sqlb.Bui, input []byte) {
	var val interface{}
	try(json.Unmarshal(input, &val))
	bui.Param(bui.Arg(val))
}

func openParen(bui *sqlb.Bui)  { bui.Str(`(`) }
func closeParen(bui *sqlb.Bui) { bui.Str(`)`) }
