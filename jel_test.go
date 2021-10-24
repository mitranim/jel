package jel

import (
	"reflect"
	"testing"
	"time"

	"github.com/mitranim/sqlb"
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

	expr := Expr{Text: src, Type: elemTypeOf((*External)(nil))}
	text, args := sqlb.Reify(expr)

	eq(
		t,
		`(($1 or ("external_name" = $2)) and ($3 and (("internal")."internal_time" < $4)))`,
		text,
	)

	eq(
		t,
		[]interface{}{false, `literal string`, true, timeFrom(`9999-01-01T00:00:00Z`)},
		args,
	)
}

func timeFrom(str string) *time.Time {
	inst, err := time.Parse(time.RFC3339, str)
	try(err)
	return &inst
}

func eq(t testing.TB, exp, act interface{}) {
	t.Helper()
	if !reflect.DeepEqual(exp, act) {
		t.Fatalf(`
expected (detailed):
	%#[1]v
actual (detailed):
	%#[2]v
expected (simple):
	%[1]v
actual (simple):
	%[2]v
`, exp, act)
	}
}
