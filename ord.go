package jel

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/mitranim/sqlb"
)

/*
Short for "orderings". Structured representation of an SQL ordering such as:

	`order by "some_col" asc`

	`order by "some_col" asc, "nested"."other_col" desc`

When encoding to a string, identifiers are quoted for safety. An ordering with
empty `.Items` represents no ordering: "".

`.Type` is used for parsing external input. It must be a struct type. Every
field name or path must be found in the struct type, possibly in nested
structs. The decoding process will convert every JSON field name into the
corresponding DB column name. Identifiers without the corresponding pair of
`json` and `db` tags cause a parse error.

Usage for parsing:

	input := []byte(`["one asc", "two.three desc"]`)

	ords := OrdsFor(SomeStructType{})

	err := ords.UnmarshalJSON(input)
	panic(err)

The result is equivalent to:

	OrdsFrom(OrdAsc(`one`), OrdDesc(`two`, `three`))

Usage for SQL:

	ords.String()

`Ords` implements `sqlb.IQuery` and can be directly used as a sub-query:

	var query sqlb.Query
	query.Append(`select from where $1`, OrdsFrom(OrdAsc(`some_col`)))
*/
type Ords struct {
	Items []Ord
	Type  reflect.Type
}

// Shortcut for creating `Ords` without a type.
func OrdsFrom(items ...Ord) Ords { return Ords{Items: items} }

/*
Shortcut for empty `Ords` intended for parsing. The input is used only as a type
carrier. The parsing process will consult the provided type; see
`Ords.UnmarshalJSON`.
*/
func OrdsFor(val interface{}) Ords { return Ords{Type: reflect.TypeOf(val)} }

/*
Implement decoding from JSON. Consults `.Type` to determine known field paths,
and converts them to DB column paths, rejecting unknown identifiers.
*/
func (self *Ords) UnmarshalJSON(input []byte) error {
	var vals []string
	err := json.Unmarshal(input, &vals)
	if err != nil {
		return err
	}
	return self.ParseSlice(vals)
}

/*
Convenience method for parsing string slices, which may come from URL queries,
form-encoded data, and so on.
*/
func (self *Ords) ParseSlice(vals []string) error {
	self.Items = make([]Ord, 0, len(vals))

	for _, val := range vals {
		var ord Ord
		err := self.parseOrd(val, &ord)
		if err != nil {
			return err
		}
		self.Items = append(self.Items, ord)
	}

	return nil
}

func (self Ords) parseOrd(str string, ord *Ord) error {
	match := ordReg.FindStringSubmatch(str)
	if match == nil {
		return fmt.Errorf(`[jel] %q is not a valid ordering string; expected format: "<ident> asc|desc"`, str)
	}

	_, path, err := structFieldByJsonPath(self.Type, match[1])
	if err != nil {
		return err
	}

	ord.Path = path
	ord.IsDesc = strings.EqualFold(match[2], `desc`)
	return nil
}

/*
Allows this to be used as a sub-query for `sqlb.Query`. When used as an argument
for `Query.Append()` or `Query.AppendNamed()`, this will be automatically
interpolated.
*/
func (self Ords) QueryAppend(out *sqlb.Query) { self.AppendBytes(&out.Text) }

/*
Generates an SQL string like:

	`order by "some_col" asc, "other_col" desc`

If the sequence is empty, returns "".
*/
func (self Ords) String() string {
	return bytesToMutableString(appendedBy(self.AppendBytes))
}

// Appends an SQL string to the buffer. See `.String()`.
func (self Ords) AppendBytes(buf *[]byte) {
	first := true

	for _, ord := range self.Items {
		if first {
			appendSpaceIfNeeded(buf)
			appendStr(buf, "order by ")
			first = false
		} else {
			appendStr(buf, ", ")
		}
		ord.AppendBytes(buf)
	}
}

// True if the item slice is empty. Doesn't care if `.Type` is set.
func (self Ords) IsEmpty() bool { return len(self.Items) == 0 }

// Convenience method for appending orderings.
func (self *Ords) Append(items ...Ord) {
	self.Items = append(self.Items, items...)
}

// If empty, replaces items with the provided fallback. Otherwise does nothing.
func (self *Ords) Or(items ...Ord) {
	if self.IsEmpty() {
		self.Items = items
	}
}

/*
Shortcut:

	OrdAsc(`one`, `two) ≡ Ord{Path: []string{`one`, `two`}, IsDesc: false}
*/
func OrdAsc(path ...string) Ord { return Ord{Path: path, IsDesc: false} }

/*
Shortcut:

	OrdDesc(`one`, `two) ≡ Ord{Path: []string{`one`, `two`}, IsDesc: true}
*/
func OrdDesc(path ...string) Ord { return Ord{Path: path, IsDesc: true} }

/*
Short for "ordering". Describes an SQL ordering like:

	`"some_col" asc`

	`("nested")."other_col" desc`

but in a structured format. When encoding for SQL, identifiers are quoted for
safety. Identifier case is preserved. Parsing of "asc" and "desc" is
case-insensitive and doesn't preserve case.

Note on `IsDesc`: the default value `false` corresponds to "ascending", which is
the default in SQL.

Also see `Ords` and `Ords`.
*/
type Ord struct {
	Path   []string
	IsDesc bool
}

/*
Returns an SQL string like:

	"some_col" asc

	("some_col")."other_col" asc
*/
func (self Ord) String() string {
	return bytesToMutableString(appendedBy(self.AppendBytes))
}

// Appends an SQL string to the buffer. See `.String()`.
func (self Ord) AppendBytes(buf *[]byte) {
	appendSqlPath(buf, self.Path)
	appendStr(buf, " ")
	if self.IsDesc {
		appendStr(buf, "desc")
	} else {
		appendStr(buf, "asc")
	}
}
