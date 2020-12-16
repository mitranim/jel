package jel

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

type Internal struct {
	InternalTime *time.Time `json:"internalTime" db:"internal_time"`
}

type External struct {
	ExternalName string   `json:"externalName" db:"external_name"`
	Internal     Internal `json:"internal"     db:"internal"`
}

func TestTranscode(t *testing.T) {
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

	expr := ExprFor(External{})
	err := json.Unmarshal([]byte(src), &expr)
	if err != nil {
		t.Fatalf("unexpected transcoding error: %+v", err)
	}

	// Not as pretty as we'd like. TODO improve.
	textExp := `( ( $1 or ("external_name" = $2 ) ) and ( $3 and (("internal")."internal_time" < $4 ) ) )`
	textGot := expr.String()

	if textExp != textGot {
		t.Fatalf("expected the resulting query to be:\n%v\ngot:\n%v\n", textExp, textGot)
	}

	argsExp := []interface{}{false, "literal string", true, timeFrom("9999-01-01T00:00:00Z")}
	argsGot := expr.Args

	if !reflect.DeepEqual(argsExp, argsGot) {
		t.Fatalf("expected the resulting args to be:\n%v\ngot:\n%v\n", argsExp, argsGot)
	}
}

func timeFrom(str string) *time.Time {
	var inst time.Time
	err := inst.UnmarshalText([]byte(str))
	if err != nil {
		panic(err)
	}
	return &inst
}
