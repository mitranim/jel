package jel

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/mitranim/refut"
)

const dottedPath = `(?:\w+\.)*\w+`

var dottedPathReg = regexp.MustCompile(`^` + dottedPath + `$`)
var ordReg = regexp.MustCompile(`^(` + dottedPath + `)\s+(?i)(asc|desc)$`)

func rec(ptr *error) {
	val := recover()
	if val == nil {
		return
	}

	recErr, ok := val.(error)
	if ok {
		*ptr = recErr
		return
	}

	panic(val)
}

func must(err error) error {
	if err != nil {
		panic(err)
	}
	return nil
}

func appendStr(buf *[]byte, str string) {
	*buf = append(*buf, str...)
}

func appendEnclosed(buf *[]byte, prefix, infix, suffix string) {
	*buf = append(*buf, prefix...)
	*buf = append(*buf, infix...)
	*buf = append(*buf, suffix...)
}

func sfieldJsonFieldName(sfield reflect.StructField) string {
	return refut.TagIdent(sfield.Tag.Get("json"))
}

/*
TODO: consider validating that the column name doesn't contain double quotes. We
might return an error, or panic.
*/
func sfieldColumnName(sfield reflect.StructField) string {
	return refut.TagIdent(sfield.Tag.Get("db"))
}

func isJsonDict(val []byte) bool {
	return firstMeaningfulByte(val) == '{'
}

func isJsonList(val []byte) bool {
	return firstMeaningfulByte(val) == '['
}

func isJsonString(val []byte) bool {
	return firstMeaningfulByte(val) == '"'
}

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
func sfieldByJsonName(rtype reflect.Type, name string, out *reflect.StructField) error {
	if rtype == nil {
		return fmt.Errorf(`[jel] can't find field %q: no type provided`, name)
	}

	err := refut.TraverseStructRtype(rtype, func(sfield reflect.StructField, _ []int) error {
		if sfieldJsonFieldName(sfield) == name {
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
func structFieldByJsonPath(rtype reflect.Type, pathStr string) (sfield reflect.StructField, path []string, err error) {
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
		err = sfieldByJsonName(rtype, segment, &sfield)
		if err != nil {
			return
		}

		colName := sfieldColumnName(sfield)
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

func appendSqlPath(buf *[]byte, path []string) {
	for i, str := range path {
		// Just a sanity check. We probably shouldn't allow to decode such
		// identifiers in the first place.
		if strings.Contains(str, `"`) {
			panic(fmt.Errorf(`[jel] unexpected %q in SQL identifier %q`, `"`, str))
		}

		if i == 0 {
			if len(path) > 1 {
				appendEnclosed(buf, `("`, str, `")`)
			} else {
				appendEnclosed(buf, `"`, str, `"`)
			}
		} else {
			appendStr(buf, `.`)
			appendEnclosed(buf, `"`, str, `"`)
		}
	}
}

func appendedBy(fun func(*[]byte)) []byte {
	var buf []byte
	fun(&buf)
	return buf
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

// Duplicated from `sqlb`.
func appendSpaceIfNeeded(buf *[]byte) {
	if buf != nil && len(*buf) > 0 && !endsWithWhitspace(*buf) {
		*buf = append(*buf, ` `...)
	}
}

func endsWithWhitspace(chunk []byte) bool {
	char, _ := utf8.DecodeLastRune(chunk)
	return isWhitespaceChar(char)
}

func isWhitespaceChar(char rune) bool {
	switch char {
	case ' ', '\n', '\r', '\t', '\v':
		return true
	default:
		return false
	}
}
