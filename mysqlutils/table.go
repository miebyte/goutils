package mysqlutils

import "gorm.io/gorm/schema"

type Table interface {
	schema.Tabler
}
