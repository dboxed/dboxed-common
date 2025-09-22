package querier

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/dboxed/dboxed-common/util"
)

func joinPrefix(a string, b string) string {
	if a == "" {
		return b
	}
	return a + "." + b
}

type StructDBField struct {
	SelectName  string
	FieldName   string
	StructField reflect.StructField
	Path        []int
}

func GetStructValueByPath(v any, path []int) reflect.Value {
	v2 := reflect.ValueOf(v)
	for {
		for v2.Kind() == reflect.Pointer {
			v2 = v2.Elem()
		}
		v2 = v2.Field(path[0])
		if len(path) == 1 {
			return v2
		}
		path = path[1:]
	}
}

type StructJoin struct {
	Type           string
	LeftTableName  string
	RightTableName string
	LeftIDField    string
	RightIDField   string
}

type structDBFieldsCacheEntry struct {
	sync.Once
	fields map[string]StructDBField
	joins  []StructJoin
}

var structDBFieldsCache sync.Map

func GetStructDBFields[T any]() (map[string]StructDBField, []StructJoin) {
	t := reflect.TypeFor[T]()
	e, ok := structDBFieldsCache.Load(t)
	if !ok {
		e, _ = structDBFieldsCache.LoadOrStore(t, &structDBFieldsCacheEntry{})
	}
	e2 := e.(*structDBFieldsCacheEntry)
	e2.Once.Do(func() {
		e2.fields = map[string]StructDBField{}
		getStructDBFields2(t, GetTableName2(t), nil, "", e2.fields, &e2.joins)
	})
	return e2.fields, e2.joins
}

func getStructJoinInfo(parentType reflect.Type, field reflect.StructField) StructJoin {
	join := StructJoin{
		Type:           "left",
		LeftTableName:  field.Tag.Get("join_left_table"),
		RightTableName: field.Tag.Get("join_right_table"),
		LeftIDField:    field.Tag.Get("join_left_field"),
		RightIDField:   field.Tag.Get("join_right_field"),
	}
	if join.LeftTableName == "" {
		join.LeftTableName = GetTableName2(parentType)
	}
	if join.RightTableName == "" {
		join.RightTableName = GetTableName2(field.Type)
	}
	if join.LeftIDField == "" {
		join.LeftIDField = "id"
	}
	if join.RightIDField == "" {
		join.RightIDField = "id"
	}

	return join
}

func dupPath(p []int, extra int) []int {
	pathCopy := make([]int, len(p)+extra)
	for i, idx := range p {
		pathCopy[i] = idx
	}
	return pathCopy
}

func getStructDBFields2(t reflect.Type, fromTableName string, path []int,
	fieldPrefix string,
	retFields map[string]StructDBField, joins *[]StructJoin) {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	path = dupPath(path, 1)

	for i := range t.NumField() {
		path[len(path)-1] = i
		f := t.Field(i)
		if f.Tag.Get("join") == "true" {
			join := getStructJoinInfo(t, f)
			*joins = append(*joins, join)
			getStructDBFields2(f.Type, join.RightTableName, path,
				joinPrefix(fieldPrefix, util.ToSnakeCase(f.Name)),
				retFields, joins)
			continue
		} else if f.Anonymous {
			getStructDBFields2(f.Type, fromTableName, path, fieldPrefix, retFields, joins)
			continue
		}
		dbFieldName := f.Tag.Get("db")
		if dbFieldName == "" {
			continue
		}

		selectName := fmt.Sprintf(`"%s"."%s"`, fromTableName, dbFieldName)
		fieldName := joinPrefix(fieldPrefix, dbFieldName)
		retFields[fieldName] = StructDBField{
			SelectName:  selectName,
			FieldName:   fieldName,
			StructField: f,
			Path:        dupPath(path, 0),
		}
	}
}
