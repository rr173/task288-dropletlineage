package store

import (
	"database/sql"
	"errors"
	"strings"

	"modernc.org/sqlite"
)

// isUniqueViolation 判断 SQLite 错误是否唯一约束冲突。
func isUniqueViolation(err error) bool {
	var se *sqlite.Error
	if errors.As(err, &se) {
		return strings.Contains(se.Error(), "UNIQUE constraint failed")
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// rowToBool 读取 0/1 整数列为布尔。
func rowToBool(v any) bool {
	switch t := v.(type) {
	case int64:
		return t != 0
	case bool:
		return t
	}
	return false
}

// ensureRowsAffected 校验 UPDATE/DELETE 影响行数，为 0 时返回 sql.ErrNoRows。
func ensureRowsAffected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
