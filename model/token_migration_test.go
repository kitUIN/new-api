package model

import (
	"strings"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestTokenMySQLTextFieldsHaveNoDefault(t *testing.T) {
	db, err := gorm.Open(mysql.New(mysql.Config{
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		DisableAutomaticPing: true,
	})
	if err != nil {
		t.Fatalf("open MySQL dialect: %v", err)
	}

	statement := &gorm.Statement{DB: db}
	if err := statement.Parse(&Token{}); err != nil {
		t.Fatalf("parse Token schema: %v", err)
	}
	for _, fieldName := range []string{
		"ModelLimits",
		"ModelGroupCombinationGroups",
		"SessionFailoverGroups",
	} {
		t.Run(fieldName, func(t *testing.T) {
			field := statement.Schema.LookUpField(fieldName)
			if field == nil {
				t.Fatalf("%s field not found", fieldName)
			}

			columnType := strings.ToUpper(db.Migrator().FullDataTypeOf(field).SQL)
			if !strings.Contains(columnType, "TEXT") {
				t.Fatalf("expected TEXT column type, got %q", columnType)
			}
			if strings.Contains(columnType, "DEFAULT") {
				t.Fatalf("MySQL TEXT column must not have a default, got %q", columnType)
			}
		})
	}
}
