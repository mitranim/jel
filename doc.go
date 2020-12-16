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
and appended to the slice of arguments. See `Expr` and `sqlb.Query`.

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

  var expr Expr
  err := json.Unmarshal([]byte(src))
  panic(err)

  text, args := expr.String(), expr.Args

The result is roughly equivalent to the following (formatted for clarity):

  text := `
    "field_one" = 'literal string'
    and
    ("field_two")."field_three" < '9999-01-01T00:00:00Z'
  `
  args := []interface{}{"literal string", time.Time("9999-01-01T00:00:00Z")}

Orderings

This package also includes a structured representation of SQL "order by"
clauses, and a tool for decoding them from arbitrary text, using the same
principles as JEL expressions. Currently this is not integrated with JEL in any
way, and must be decoded separately.

See `Ords` for usage examples.
*/
package jel
