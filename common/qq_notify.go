package common

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type qqNotifyPayload struct {
	QQ      string `json:"qq"`
	AdminQQ string `json:"admin_qq"`
	To      string `json:"to"`
	Message string `json:"message"`
	Content string `json:"content"`
}

func qqNotifyServiceURL(path string) string {
	return strings.TrimRight(QQCallbackAddress, "/") + path
}

func setQQNotifyServiceAuthHeader(req *http.Request) {
	if QQCallbackAccessToken != "" {
		req.Header.Set("Authorization", QQCallbackAccessToken)
		req.Header.Set("X-Access-Token", QQCallbackAccessToken)
	}
}

func SendQQNotificationGroupMessage(message string, logPrefix string) {
	message = strings.TrimSpace(message)
	targetGroup := strings.TrimSpace(QQNotificationGroup)
	if message == "" || QQCallbackAddress == "" || QQCallbackAccessToken == "" || targetGroup == "" {
		return
	}

	go func() {
		payload, err := Marshal(qqNotifyPayload{
			QQ:      targetGroup,
			AdminQQ: QQAdminNumber,
			To:      targetGroup,
			Message: message,
			Content: message,
		})
		if err != nil {
			SysLog(fmt.Sprintf("failed to marshal %s payload: %v", logPrefix, err))
			return
		}
		req, err := http.NewRequest(http.MethodPost, qqNotifyServiceURL("/api/nachoai/send_group_message"), bytes.NewReader(payload))
		if err != nil {
			SysLog(fmt.Sprintf("failed to create %s request: %v", logPrefix, err))
			return
		}
		req.Header.Set("Content-Type", "application/json")
		setQQNotifyServiceAuthHeader(req)

		client := http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			SysLog(fmt.Sprintf("failed to send %s: %v", logPrefix, err))
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			SysLog(fmt.Sprintf("failed to send %s: status=%d", logPrefix, resp.StatusCode))
		}
	}()
}

func SendQQAdminMessage(message string, logPrefix string) {
	message = strings.TrimSpace(message)
	targetAdmin := strings.TrimSpace(QQAdminNumber)
	if message == "" || QQCallbackAddress == "" || QQCallbackAccessToken == "" || targetAdmin == "" {
		return
	}

	go func() {
		payload, err := Marshal(qqNotifyPayload{
			QQ:      targetAdmin,
			AdminQQ: targetAdmin,
			To:      targetAdmin,
			Message: message,
			Content: message,
		})
		if err != nil {
			SysLog(fmt.Sprintf("failed to marshal %s payload: %v", logPrefix, err))
			return
		}
		req, err := http.NewRequest(http.MethodPost, qqNotifyServiceURL("/api/nachoai/send_message"), bytes.NewReader(payload))
		if err != nil {
			SysLog(fmt.Sprintf("failed to create %s request: %v", logPrefix, err))
			return
		}
		req.Header.Set("Content-Type", "application/json")
		setQQNotifyServiceAuthHeader(req)

		client := http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			SysLog(fmt.Sprintf("failed to send %s: %v", logPrefix, err))
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			SysLog(fmt.Sprintf("failed to send %s: status=%d", logPrefix, resp.StatusCode))
		}
	}()
}

func SendQQUserMessage(qqId string, message string, logPrefix string) error {
	qqId = strings.TrimSpace(qqId)
	message = strings.TrimSpace(message)
	if qqId == "" {
		return fmt.Errorf("qq id is empty")
	}
	if message == "" {
		return fmt.Errorf("message is empty")
	}
	if QQCallbackAddress == "" || QQCallbackAccessToken == "" || QQAdminNumber == "" {
		return fmt.Errorf("qq notification service is not configured")
	}

	payload, err := Marshal(qqNotifyPayload{
		QQ:      qqId,
		AdminQQ: QQAdminNumber,
		To:      qqId,
		Message: message,
		Content: message,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal %s payload: %w", logPrefix, err)
	}
	req, err := http.NewRequest(http.MethodPost, qqNotifyServiceURL("/api/nachoai/send_message"), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create %s request: %w", logPrefix, err)
	}
	req.Header.Set("Content-Type", "application/json")
	setQQNotifyServiceAuthHeader(req)

	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send %s: %w", logPrefix, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to send %s: status=%d", logPrefix, resp.StatusCode)
	}
	return nil
}
