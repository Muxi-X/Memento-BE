package common

import "errors"

var (
	// 表示未在数据库中找到数据
	ErrNotFound = errors.New("not found")
	// 表示数据冲突，例如违反唯一约束
	ErrConflict = errors.New("conflict")
)
