package soft_delete

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dboxed/dboxed-common/db/querier"
	"github.com/dboxed/dboxed-common/util"
	"github.com/google/uuid"
)

type IsSoftDelete interface {
	SetId(id int64)
	GetId() int64
	GetDeletedAt() *time.Time
	GetFinalizers() []string
	SetFinalizers(finalizers []string)
	HasFinalizer(k string) bool

	setFinalizersRaw(finalizers string)
}

type SoftDeleteFields struct {
	DeletedAt  sql.NullTime `db:"deleted_at" omitCreate:"true"`
	Finalizers string       `db:"finalizers" omitCreate:"true"`
}

func (v *SoftDeleteFields) GetDeletedAt() *time.Time {
	if !v.DeletedAt.Valid {
		return nil
	}
	return &v.DeletedAt.Time
}

func (v *SoftDeleteFields) GetFinalizers() []string {
	if v.Finalizers == "{}" || v.Finalizers == "" {
		return nil
	}
	var m map[string]any
	err := json.Unmarshal([]byte(v.Finalizers), &m)
	if err != nil {
		panic(err)
	}
	ret := make([]string, 0, len(m))
	for k := range m {
		ret = append(ret, k)
	}
	return ret
}

func (v *SoftDeleteFields) SetFinalizers(finalizers []string) {
	m := map[string]bool{}
	for _, x := range finalizers {
		m[x] = true
	}
	v.Finalizers = util.MustJson(m)
}

func (v *SoftDeleteFields) setFinalizersRaw(finalizers string) {
	v.Finalizers = finalizers
}

func (v *SoftDeleteFields) HasFinalizer(k string) bool {
	return slices.Contains(v.GetFinalizers(), k)
}

func SoftDelete[T querier.HasId](q *querier.Querier, byFields map[string]any) error {
	return querier.UpdateOneByFields[T](q, byFields, map[string]any{
		"deleted_at": querier.RawSql("current_timestamp"),
	})
}

func SoftDeleteWithConstraints[T querier.HasId](q *querier.Querier, byFields map[string]any) error {
	savepoint := "s_" + strings.ReplaceAll(uuid.NewString(), "-", "_")
	_, err := q.ExecNamed(fmt.Sprintf("savepoint %s", savepoint), nil)
	if err != nil {
		return err
	}

	err = querier.DeleteOneByFields[T](q, byFields)
	if err != nil {
		return err
	}
	_, err = q.ExecNamed(fmt.Sprintf("rollback to savepoint %s", savepoint), nil)
	if err != nil {
		return err
	}

	err = SoftDelete[T](q, byFields)
	if err != nil {
		return err
	}

	return nil
}

var querySetDBFinalizers = map[string]string{
	"pgx": `update @@table_name
set    finalizers = jsonb_strip_nulls(jsonb_set(to_jsonb(finalizers::::json), '{@@k}', '@@nullOrTrue'))
where id = :id
returning finalizers`,
	"sqlite3": `
update @@table_name
set    finalizers = json_patch(finalizers, '{"@@k":: @@nullOrTrue}')
where id = :id
returning finalizers`,
}

func setDBFinalizers[T any](q *querier.Querier, id int64, k string, v bool) (string, error) {
	nullOrTrue := "null"
	if v {
		nullOrTrue = "true"
	}

	var newFinalizers string
	err := q.GetNamed(&newFinalizers, querySetDBFinalizers, map[string]any{
		"id":           id,
		"@@table_name": querier.GetTableName[T](),
		"@@k":          k,
		"@@nullOrTrue": nullOrTrue,
	})
	if err != nil {
		return "", err
	}

	return newFinalizers, nil
}

func AddFinalizer[T IsSoftDelete](q *querier.Querier, v T, finalizer string) error {
	if v.HasFinalizer(finalizer) {
		return nil
	}

	newFinalizers, err := setDBFinalizers[T](q, v.GetId(), finalizer, true)
	if err != nil {
		return err
	}

	v.setFinalizersRaw(newFinalizers)

	return nil
}

func RemoveFinalizer[T IsSoftDelete](q *querier.Querier, v T, finalizer string) error {
	if !v.HasFinalizer(finalizer) {
		return nil
	}

	newFinalizers, err := setDBFinalizers[T](q, v.GetId(), finalizer, false)
	if err != nil {
		return err
	}

	v.setFinalizersRaw(newFinalizers)

	return nil
}
