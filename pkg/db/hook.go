package db

// import (
// 	"fmt"
// 	"reflect"
// 	"time"

// 	"smallbiznis-controlplane/internal/audit/domain"
// 	"go.uber.org/zap"
// 	"gorm.io/gorm"
// )

// func BeforeCreate(tx *gorm.DB) error {

// 	if tx.Statement.Schema == nil {
// 		return nil
// 	}

// 	resource := tx.Statement.Table
// 	resourceID := tx.Statement.ReflectValue.FieldByName("ID").String()
// 	action := "CREATE"

// 	auditLog := domain.AuditLog{
// 		ResourceID:   resourceID,
// 		ResourceName: resource,
// 		Action:       action,
// 		CreatedAt:    time.Now(),
// 	}

// 	if userID, ok := tx.Get("user_id"); ok {
// 		auditLog.UserID = userID.(string)
// 	}

// 	if orgID, ok := tx.Get("organization_id"); ok {
// 		auditLog.OrganizationID = orgID.(string)
// 	}

// 	if err := tx.Session(&gorm.Session{SkipHooks: true}).Create(&auditLog).Error; err != nil {
// 		zap.L().Error("failed to create audit field value", zap.Error(err))
// 		return err
// 	}

// 	for _, column := range tx.Statement.Schema.Fields {
// 		if column.Name != "id" {
// 			val := tx.Statement.ReflectValue.FieldByName(column.Name)
// 			newValue := fmt.Sprintf("%v", val.Interface())

// 			auditField := domain.AuditLogFieldValue{
// 				AuditLogID:    auditLog.ID,
// 				Field:         column.Name,
// 				PreviousValue: "",
// 				NewValue:      newValue,
// 			}

// 			if err := tx.Session(&gorm.Session{SkipHooks: true}).Create(&auditField).Error; err != nil {
// 				zap.L().Error("failed to create audit field value", zap.Error(err))
// 				return err
// 			}
// 		}
// 	}
// }

// func BeforeSave(tx *gorm.DB) error {

// 	if tx.Statement.Schema == nil {
// 		return nil
// 	}

// 	resource := tx.Statement.Table
// 	resourceID := tx.Statement.ReflectValue.FieldByName("ID").String()
// 	action := "UPDATE"

// 	auditLog := domain.AuditLog{
// 		ResourceID:   resourceID,
// 		ResourceName: resource,
// 		Action:       action,
// 		CreatedAt:    time.Now(),
// 	}

// 	userID, ok := tx.Get("user_id")
// 	if ok {
// 		auditLog.UserID = userID.(string)
// 	}

// 	orgID, ok := tx.Get("organization_id")
// 	if ok {
// 		auditLog.OrganizationID = orgID.(string)
// 	}

// 	if err := tx.Create(&auditLog).Error; err != nil {
// 		zap.L().Error("failed to create audit field value", zap.Error(err))
// 		return err
// 	}

// 	old := reflect.New(tx.Statement.Schema.ModelType).Interface()
// 	if err := tx.Session(&gorm.Session{NewDB: true}).Where("id = ?", resourceID).First(&old).Error; err != nil {
// 		zap.L().Error("failed to fetch previous data", zap.Error(err))
// 		return err
// 	}

// 	oldValue := reflect.ValueOf(old).Elem()
// 	for _, column := range tx.Statement.Schema.Fields {
// 		if column.Name != "id" {

// 			previousValue := oldValue.FieldByName(column.Name).Interface()
// 			newValue := tx.Statement.ReflectValue.FieldByName(column.Name).Interface()

// 			auditField := domain.AuditLogFieldValue{
// 				AuditLogID:    auditLog.ID,
// 				Field:         column.Name,
// 				PreviousValue: fmt.Sprintf("%s", previousValue),
// 				NewValue:      fmt.Sprintf("%s", newValue),
// 			}

// 			if err := tx.Session(&gorm.Session{SkipHooks: true}).Create(&auditField).Error; err != nil {
// 				zap.L().Error("failed to create audit field value", zap.Error(err))
// 				return err
// 			}
// 		}
// 	}

// 	return nil
// }
