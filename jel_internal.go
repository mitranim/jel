package jel

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unsafe"

	"github.com/mitranim/refut"
	"github.com/mitranim/sqlb"
)

const dottedPath = `(?:\w+\.)*\w+`

var dottedPathReg = regexp.MustCompile(`^` + dottedPath + `$`)

func try(err error) {
	if err != nil {
		panic(err)
	}
}

func isJsonDict(val []byte) bool   { return firstMeaningfulByte(val) == '{' }
func isJsonList(val []byte) bool   { return firstMeaningfulByte(val) == '[' }
func isJsonString(val []byte) bool { return firstMeaningfulByte(val) == '"' }

func firstMeaningfulByte(val []byte) byte {
	val = bytes.TrimSpace(val)
	if len(val) > 0 {
		return val[0]
	}
	return 0
}

var errBreak = errors.New("")

/*
Finds the struct field that has the given JSON field name. The field may be in
an embedded struct, but not in any non-embedded nested structs.
*/
func fieldByJsonName(rtype reflect.Type, name string, out *reflect.StructField) error {
	if rtype == nil {
		return fmt.Errorf(`[jel] can't find field %q: no type provided`, name)
	}

	err := refut.TraverseStructRtype(rtype, func(sfield reflect.StructField, _ []int) error {
		if sqlb.FieldJsonName(sfield) == name {
			*out = sfield
			return errBreak
		}
		return nil
	})
	if errors.Is(err, errBreak) {
		return nil
	}
	if err != nil {
		return err
	}

	return fmt.Errorf(`[jel] no struct field corresponding to JSON field name %q in type %v`, name, rtype)
}

/*
Takes a struct type and a dot-separated path of JSON field names
like "one.two.three". Finds the nested struct field corresponding to that path,
returning an error if a field could not be found.

Note that this can't use `reflect.Value.FieldByName` because it searches by JSON
field name, not by Go field name.
*/
func fieldByJsonPath(rtype reflect.Type, pathStr string) (sfield reflect.StructField, path []string, err error) {
	if !dottedPathReg.MatchString(pathStr) {
		err = fmt.Errorf(`[jel] expected a valid dot-separated identifier, got %q`, pathStr)
		return
	}

	if rtype == nil {
		err = fmt.Errorf(`[jel] can't find field by path %q: no type provided`, pathStr)
		return
	}

	path = strings.Split(pathStr, ".")

	for i, segment := range path {
		err = fieldByJsonName(rtype, segment, &sfield)
		if err != nil {
			return
		}

		colName := sqlb.FieldDbName(sfield)
		if colName == "" {
			err = fmt.Errorf(`[jel] no column name corresponding to %q in type %v for path %q`,
				segment, rtype, pathStr)
			return
		}

		path[i] = colName
		rtype = sfield.Type
	}
	return
}

func typeElem(typ reflect.Type) reflect.Type {
	for typ != nil && (typ.Kind() == reflect.Ptr || typ.Kind() == reflect.Slice) {
		typ = typ.Elem()
	}
	return typ
}

func elemTypeOf(typ interface{}) reflect.Type {
	return typeElem(reflect.TypeOf(typ))
}

/*
Allocation-free conversion. Reinterprets a byte slice as a string. Borrowed from
the standard library. Reasonably safe. Should not be used when the underlying
byte array is volatile, for example when it's part of a scratch buffer during
SQL scanning.
*/
func bytesToMutableString(bytes []byte) string {
	return *(*string)(unsafe.Pointer(&bytes))
}

/*
Allocation-free conversion. Returns a byte slice backed by the provided string.
Mutations are reflected in the source string, unless it's backed by constant
storage, in which case they trigger a segfault. Reslicing is ok. Should be safe
as long as the resulting bytes are not mutated. Sometimes produces unexpected
garbage, possibly because the string was, in turn, backed by mutable storage
which gets modified before we use the result; needs investigation.
*/
func stringToBytesUnsafe(val string) []byte {
	type sliceHeader struct {
		_   uintptr
		len int
		cap int
	}
	slice := *(*sliceHeader)(unsafe.Pointer(&val))
	slice.cap = slice.len
	return *(*[]byte)(unsafe.Pointer(&slice))
}

// Self-reminder about non-free conversions.
func bytesToStringAlloc(bytes []byte) string { return string(bytes) }

func exprAppend(expr sqlb.Expr, text []byte) []byte {
	text, _ = expr.AppendExpr(text, nil)
	return text
}

func exprString(expr sqlb.Expr) string {
	return bytesToMutableString(exprAppend(expr, nil))
}
