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
	return Expr{Type: reflect.TypeOf(val)}
}

/*
Shortcut for instantiating a boolean-style `Expr` with the type of the given value.
The input is used only as a type carrier.
*/
func Bool(val interface{}) Expr {
	return Expr{Type: reflect.TypeOf(val), IsBool: true}
}

/*
Tool for transcoding JEL into SQL.

Embeds `sqlb.Query` and implements `sqlb.IQuery`. Can be transparently used as a
sub-query for other `sqlb` queries.
*/
type Expr struct {
	sqlb.Query
	Type   reflect.Type
	IsBool bool
}

/*
Support decoding from text, which must be valid JSON. This is provided for cases
like decoding from URL queries or other sources that don't originate in JSON.
*/
func (self *Expr) UnmarshalText(input []byte) error { return self.decode(input) }

// Support decoding from JSON.
func (self *Expr) UnmarshalJSON(input []byte) error { return self.decode(input) }

var _ = sqlb.IQuery(Expr{})

/*
Implement `sqlb.IQuery`. If `.IsBool = true` and the query is empty, this will
append `true`; this allows the caller to always expect some expression, avoiding
invalid syntax.
*/
func (self Expr) QueryAppend(out *sqlb.Query) {
	if self.IsBool && len(self.Query.Text) == 0 {
		out.Append(`true`)
	} else {
		self.Query.QueryAppend(out)
	}
}

func (self *Expr) decode(input []byte) error {
	if isJsonDict(input) {
		return fmt.Errorf(`[jel] unexpected dict in input: %q`, input)
	}
	if isJsonList(input) {
		return self.decodeList(input)
	}
	if isJsonString(input) {
		return self.decodeString(input)
	}
	return self.decodeAny(input)
}

func (self *Expr) decodeList(input []byte) error {
	var list []json.RawMessage
	err := json.Unmarshal(input, &list)
	if err != nil {
		return fmt.Errorf(`[jel] failed to unmarshal as JSON list: %w`, err)
	}

	if !(len(list) > 0) {
		return fmt.Errorf(`[jel] lists must have at least one element, found empty list`)
	}

	head, args := list[0], list[1:]
	if !isJsonString(head) {
		return fmt.Errorf(`[jel] first list element must be a string, found %q`, head)
	}

	var name string
	err = json.Unmarshal(head, &name)
	if err != nil {
		return fmt.Errorf(`[jel] failed to unmarshal JSON list head as string: %w`, err)
	}

	switch SqlOps[name] {
	case SqlPrefix:
		return self.decodeOpPrefix(name, args)
	case SqlPostfix:
		return self.decodeOpPostfix(name, args)
	case SqlInfix:
		return self.decodeOpInfix(name, args)
	case SqlFunc:
		return self.decodeOpFunc(name, args)
	case SqlAny:
		return self.decodeOpAny(name, args)
	case SqlBetween:
		return self.decodeOpBetween(name, args)
	}

	return self.decodeCast(name, args)
}

func (self *Expr) decodeOpPrefix(name string, args []json.RawMessage) (err error) {
	defer rec(&err)
	if len(args) != 1 {
		return fmt.Errorf(`[jel] prefix operation %q must have exactly 1 argument, found %v`, name, len(args))
	}

	self.openParen()
	self.Append(name)
	must(self.decode(args[0]))
	self.closeParen()
	return nil
}

func (self *Expr) decodeOpPostfix(name string, args []json.RawMessage) (err error) {
	defer rec(&err)
	if len(args) != 1 {
		return fmt.Errorf(`[jel] postfix operation %q must have exactly 1 argument, found %v`, name, len(args))
	}

	self.openParen()
	must(self.decode(args[0]))
	self.Append(name)
	self.closeParen()
	return nil
}

func (self *Expr) decodeOpInfix(name string, args []json.RawMessage) (err error) {
	defer rec(&err)
	if !(len(args) >= 2) {
		return fmt.Errorf(`[jel] infix operation %q must have at least 2 arguments, found %v`, name, len(args))
	}

	self.openParen()
	for i, arg := range args {
		if i > 0 {
			self.Append(name)
		}
		must(self.decode(arg))
	}
	self.closeParen()
	return nil
}

func (self *Expr) decodeOpFunc(name string, args []json.RawMessage) (err error) {
	defer rec(&err)
	self.Append(name)
	self.openParen()
	for i, arg := range args {
		if i > 0 {
			appendStr(&self.Text, `, `)
		}
		must(self.decode(arg))
	}
	self.closeParen()
	return nil
}

func (self *Expr) decodeOpAny(name string, args []json.RawMessage) (err error) {
	defer rec(&err)
	if len(args) != 2 {
		return fmt.Errorf(`[jel] operation %q must have exactly 2 arguments, found %v`, name, len(args))
	}

	self.openParen()
	must(self.decode(args[0]))
	self.Append(`=`)
	self.Append(name)
	self.openParen()
	must(self.decode(args[1]))
	self.closeParen()
	self.closeParen()
	return nil
}

func (self *Expr) decodeOpBetween(name string, args []json.RawMessage) (err error) {
	defer rec(&err)
	if len(args) != 3 {
		return fmt.Errorf(`[jel] operation %q must have exactly 3 arguments, found %v`, name, len(args))
	}

	self.openParen()
	must(self.decode(args[0]))
	self.Append(`between`)
	must(self.decode(args[1]))
	self.Append(`and`)
	must(self.decode(args[2]))
	self.closeParen()
	return nil
}

func (self *Expr) decodeCast(name string, args []json.RawMessage) (err error) {
	defer rec(&err)
	if len(args) != 1 {
		return fmt.Errorf(`[jel] cast into %q must have exactly 1 argument, found %v`, name, len(args))
	}

	sfield, _, err := structFieldByJsonPath(self.Type, name)
	must(err)

	rval := reflect.New(sfield.Type)
	must(json.Unmarshal(args[0], rval.Interface()))

	self.Append("$1", rval.Elem().Interface())
	return nil
}

func (self *Expr) decodeString(input []byte) (err error) {
	defer rec(&err)

	var str string
	must(json.Unmarshal(input, &str))

	_, path, err := structFieldByJsonPath(self.Type, str)
	if err != nil {
		return err
	}

	appendSqlPath(&self.Text, path)
	return nil
}

// Should be used only for numbers, bools, nulls.
// TODO: unmarshal integers into `int64` rather than `float64`.
func (self *Expr) decodeAny(input []byte) error {
	var val interface{}
	err := json.Unmarshal(input, &val)
	if err != nil {
		return err
	}
	self.Append("$1", val)
	return nil
}

func (self *Expr) openParen()  { self.Append(`(`) }
func (self *Expr) closeParen() { self.Append(`)`) }
