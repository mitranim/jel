package jel_test

import (
	"fmt"
	"reflect"
	"time"

	"github.com/mitranim/jel"
)

func ExampleExpr() {
	type Internal struct {
		InternalTime *time.Time `json:"internalTime" db:"internal_time"`
	}

	type External struct {
		ExternalName string   `json:"externalName" db:"external_name"`
		Internal     Internal `json:"internal"     db:"internal"`
	}

	const src = `
		["and",
			["or",
				false,
				["=", "externalName", ["externalName", "literal string"]]
			],
			["and",
				true,
				["<", "internal.internalTime", ["internal.internalTime", "9999-01-01T00:00:00Z"]]
			]
		]
	`

	expr := jel.Expr{
		Text: src,
		Type: reflect.TypeOf((*External)(nil)).Elem(),
	}

	text, args := expr.AppendExpr(nil, nil)

	fmt.Println(string(text))
	fmt.Printf("%#v\n", args)
	// Output:
	// (($1 or ("external_name" = $2)) and ($3 and (("internal")."internal_time" < $4)))
	// []interface {}{false, "literal string", true, time.Date(9999, time.January, 1, 0, 0, 0, 0, time.UTC)}
}
