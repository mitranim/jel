package jel

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/mitranim/sqlb"
)

func TestOrdAsc(t *testing.T) {
	eq(t,
		Ord{Path: []string{`one`, `two`}, IsDesc: false},
		OrdAsc(`one`, `two`),
	)
}

func TestOrdDesc(t *testing.T) {
	eq(t,
		Ord{Path: []string{`one`, `two`}, IsDesc: true},
		OrdDesc(`one`, `two`),
	)
}

func TestOrdString(t *testing.T) {
	t.Run(`singular`, func(t *testing.T) {
		eq(t, `"one" asc nulls last`, OrdAsc(`one`).String())
		eq(t, `"one" desc nulls last`, OrdDesc(`one`).String())
	})
	t.Run(`binary`, func(t *testing.T) {
		eq(t, `("one")."two" asc nulls last`, OrdAsc(`one`, `two`).String())
		eq(t, `("one")."two" desc nulls last`, OrdDesc(`one`, `two`).String())
	})
	t.Run(`plural`, func(t *testing.T) {
		eq(t, `("one")."two"."three" asc nulls last`, OrdAsc(`one`, `two`, `three`).String())
		eq(t, `("one")."two"."three" desc nulls last`, OrdDesc(`one`, `two`, `three`).String())
	})
}

func TestOrdsString(t *testing.T) {
	t.Run(`empty`, func(t *testing.T) {
		eq(t, ``, Ords{}.String())
	})
	t.Run(`singular`, func(t *testing.T) {
		eq(t, `order by "one" asc nulls last`, OrdsFrom(OrdAsc(`one`)).String())
		eq(t, `order by "one" desc nulls last`, OrdsFrom(OrdDesc(`one`)).String())
	})
	t.Run(`binary`, func(t *testing.T) {
		eq(t, `order by ("one")."two" asc nulls last`, OrdsFrom(OrdAsc(`one`, `two`)).String())
		eq(t, `order by ("one")."two" desc nulls last`, OrdsFrom(OrdDesc(`one`, `two`)).String())
	})
	t.Run(`plural`, func(t *testing.T) {
		eq(t, `order by ("one")."two"."three" asc nulls last`, OrdsFrom(OrdAsc(`one`, `two`, `three`)).String())
		eq(t, `order by ("one")."two"."three" desc nulls last`, OrdsFrom(OrdDesc(`one`, `two`, `three`)).String())
	})
}

func TestOrdsQueryAppend(t *testing.T) {
	t.Run(`direct`, func(t *testing.T) {
		var query sqlb.Query
		query.Append(`select from where`)
		query.AppendQuery(OrdsFrom(OrdAsc(`one`, `two`, `three`)))
		eq(t, `select from where order by ("one")."two"."three" asc nulls last`, query.String())
	})

	t.Run(`parametrized`, func(t *testing.T) {
		var query sqlb.Query
		query.Append(`select from where $1`, OrdsFrom(OrdAsc(`one`, `two`, `three`)))
		eq(t, `select from where order by ("one")."two"."three" asc nulls last`, query.String())
	})
}

func TestOrdsDec(t *testing.T) {
	t.Run(`decode_from_json`, func(t *testing.T) {
		const input = `["externalName asc", "internal.internalTime desc"]`
		ords := OrdsFor(External{})

		err := json.Unmarshal([]byte(input), &ords)
		if err != nil {
			t.Fatalf("failed to decode ord from JSON: %+v", err)
		}

		eq(t, ords.Items, OrdsFrom(OrdAsc(`external_name`), OrdDesc(`internal`, `internal_time`)).Items)
	})

	t.Run(`decode_from_strings`, func(t *testing.T) {
		input := []string{"externalName asc", "internal.internalTime desc"}
		ords := OrdsFor(External{})

		err := ords.ParseSlice(input)
		if err != nil {
			t.Fatalf("failed to decode ord from strings: %+v", err)
		}

		eq(t, ords.Items, OrdsFrom(OrdAsc(`external_name`), OrdDesc(`internal`, `internal_time`)).Items)
	})

	t.Run(`reject_unknown_fields`, func(t *testing.T) {
		input := []string{"external_name asc nulls last"}
		ords := OrdsFor(External{})

		err := ords.ParseSlice(input)
		if err == nil {
			t.Fatalf("expected decoding to fail")
		}
	})

	t.Run(`fail_when_type_is_not_provided`, func(t *testing.T) {
		input := []string{"some_ident asc nulls last"}
		ords := OrdsFor(nil)

		err := ords.ParseSlice(input)
		if err == nil {
			t.Fatalf("expected decoding to fail")
		}
	})
}

func eq(t *testing.T, expected interface{}, actual interface{}) {
	if !reflect.DeepEqual(expected, actual) {
		t.Fatalf("expected:\n%#v\nactual:\n%#v", expected, actual)
	}
}
