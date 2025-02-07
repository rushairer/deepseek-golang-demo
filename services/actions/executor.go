package actions

import (
	"database/sql"
	"fmt"
	"log"

	"deepseek_golang_demo/models"
	"deepseek_golang_demo/services/notification"
)

// ExecuteAction 执行建议操作
func ExecuteAction(action models.Action, db *sql.DB) error {
	switch action.Type {
	case "数据库操作":
		return executeDatabaseAction(action, db)
	case "通知":
		return executeNotificationAction(action, db)
	case "标记":
		return executeTaggingAction(action, db)
	default:
		return fmt.Errorf("未知的操作类型: %s", action.Type)
	}
}

// executeDatabaseAction 执行数据库操作
func executeDatabaseAction(action models.Action, db *sql.DB) error {
	switch action.Target {
	case "update_status":
		status, ok := action.Params["status"].(string)
		if !ok {
			return fmt.Errorf("无效的状态参数")
		}
		id, ok := action.Params["id"].(string)
		if !ok {
			return fmt.Errorf("无效的ID参数")
		}
		return models.UpdateStatus(db, id, status)

	case "add_tag":
		tag, ok := action.Params["tag"].(string)
		if !ok {
			return fmt.Errorf("无效的标签参数")
		}
		id, ok := action.Params["id"].(string)
		if !ok {
			return fmt.Errorf("无效的ID参数")
		}
		return models.AddTag(db, id, tag)

	default:
		return fmt.Errorf("未知的数据库操作目标: %s", action.Target)
	}
}

// executeNotificationAction 执行通知操作
func executeNotificationAction(action models.Action, db *sql.DB) error {
	message, ok := action.Params["message"].(string)
	if !ok {
		return fmt.Errorf("无效的消息参数")
	}

	channel, ok := action.Params["channel"].(string)
	if !ok {
		return fmt.Errorf("无效的通知渠道参数")
	}

	// 创建通知记录
	recordID, ok := action.Params["record_id"].(float64)
	if !ok {
		return fmt.Errorf("无效的记录ID参数")
	}

	// 创建通知记录
	if err := models.CreateNotification(db, int64(recordID), channel, message); err != nil {
		log.Printf("创建通知记录失败: %v", err)
		return fmt.Errorf("创建通知记录失败: %v", err)
	}

	// 发送通知
	if err := notification.Send(channel, message, action.Params); err != nil {
		log.Printf("发送通知失败: %v", err)
		// 更新通知状态为失败
		if updateErr := models.UpdateNotificationStatus(db, int64(recordID), "failed"); updateErr != nil {
			log.Printf("更新通知状态失败: %v", updateErr)
		}
		return fmt.Errorf("发送通知失败: %v", err)
	}

	// 更新通知状态为已发送
	if err := models.UpdateNotificationStatus(db, int64(recordID), "sent"); err != nil {
		log.Printf("更新通知状态失败: %v", err)
		return fmt.Errorf("更新通知状态失败: %v", err)
	}

	return nil
}

// executeTaggingAction 执行标记操作
func executeTaggingAction(action models.Action, db *sql.DB) error {
	tag, ok := action.Params["tag"].(string)
	if !ok {
		return fmt.Errorf("无效的标签参数")
	}

	id, ok := action.Params["id"].(string)
	if !ok {
		return fmt.Errorf("无效的ID参数")
	}

	return models.AddTag(db, id, tag)
}
