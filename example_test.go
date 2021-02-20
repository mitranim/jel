package jel_test

import (
	"encoding/json"
	"fmt"
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

	expr := jel.ExprFor(External{})
	err := json.Unmarshal([]byte(src), &expr)
	if err != nil {
		panic(err)
	}

	fmt.Println(expr.String())

	/**
	Formatted here for readability:

	($1 or "external_name" = $2) and ($3 and ("internal")."internal_time" < $4)
	*/
}
