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
	case "database":
		return executeDatabaseAction(action, db)
	case "notification":
		return executeNotificationAction(action, db)
	case "tag":
		return executeTaggingAction(action, db)
	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// executeDatabaseAction 执行数据库操作
func executeDatabaseAction(action models.Action, db *sql.DB) error {
	switch action.Target {
	case "update_status":
		status, ok := action.Params["status"].(string)
		if !ok {
			return fmt.Errorf("invalid status parameter")
		}
		id, ok := action.Params["record_id"].(float64)
		if !ok {
			return fmt.Errorf("invalid record_id parameter")
		}
		return models.UpdateStatus(db, fmt.Sprintf("%d", int64(id)), status)

	case "add_tag":
		tag, ok := action.Params["tag"].(string)
		if !ok {
			return fmt.Errorf("invalid tag parameter")
		}
		id, ok := action.Params["record_id"].(float64)
		if !ok {
			return fmt.Errorf("invalid record_id parameter")
		}
		return models.AddTag(db, fmt.Sprintf("%d", int64(id)), tag)

	default:
		return fmt.Errorf("unknown database action target: %s", action.Target)
	}
}

// executeNotificationAction 执行通知操作
func executeNotificationAction(action models.Action, db *sql.DB) error {
	message, ok := action.Params["message"].(string)
	if !ok {
		return fmt.Errorf("invalid message parameter")
	}

	channel, ok := action.Params["channel"].(string)
	if !ok {
		return fmt.Errorf("invalid channel parameter")
	}

	recordID, ok := action.Params["record_id"].(float64)
	if !ok {
		return fmt.Errorf("invalid record_id parameter")
	}

	// Create notification record
	if err := models.CreateNotification(db, int64(recordID), channel, message); err != nil {
		log.Printf("Failed to create notification: %v", err)
		return fmt.Errorf("failed to create notification: %v", err)
	}

	// Send notification
	if err := notification.Send(channel, message, action.Params); err != nil {
		log.Printf("Failed to send notification: %v", err)
		// Update notification status to failed
		if updateErr := models.UpdateNotificationStatus(db, int64(recordID), "failed"); updateErr != nil {
			log.Printf("Failed to update notification status: %v", updateErr)
		}
		return fmt.Errorf("failed to send notification: %v", err)
	}

	// Update notification status to sent
	if err := models.UpdateNotificationStatus(db, int64(recordID), "sent"); err != nil {
		log.Printf("Failed to update notification status: %v", err)
		return fmt.Errorf("failed to update notification status: %v", err)
	}

	return nil
}

// executeTaggingAction 执行标记操作
func executeTaggingAction(action models.Action, db *sql.DB) error {
	tag, ok := action.Params["tag"].(string)
	if !ok {
		return fmt.Errorf("invalid tag parameter")
	}

	id, ok := action.Params["record_id"].(float64)
	if !ok {
		return fmt.Errorf("invalid record_id parameter")
	}

	return models.AddTag(db, fmt.Sprintf("%d", int64(id)), tag)
}
