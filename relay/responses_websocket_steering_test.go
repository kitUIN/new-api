package relay

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const steeringTestQuota = 10000000

func responsesWebSocketPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	accepted := make(chan *websocket.Conn, 1)
	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		accepted <- conn
		<-done
	}))
	client, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	require.NoError(t, err)
	serverConn := <-accepted
	t.Cleanup(func() {
		_ = client.Close()
		_ = serverConn.Close()
		close(done)
		server.Close()
	})
	return client, serverConn
}

func newSteeringTestSession(t *testing.T) (*responsesWebSocketSession, *websocket.Conn, *websocket.Conn, *gorm.DB) {
	t.Helper()
	oldRatios, err := common.Marshal(ratio_setting.GetModelRatioCopy())
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"gpt-4o":1}`))
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(oldRatios))) })
	oldDB, oldLogDB := model.DB, model.LOG_DB
	oldRedis, oldBatch, oldLog := common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled
	oldSQLite, oldMySQL, oldPG := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL
	oldPath, oldMaster := common.SQLitePath, common.IsMasterNode
	t.Cleanup(func() {
		model.DB, model.LOG_DB = oldDB, oldLogDB
		common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = oldRedis, oldBatch, oldLog
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = oldSQLite, oldMySQL, oldPG
		common.SQLitePath, common.IsMasterNode = oldPath, oldMaster
	})
	// Use the production initializer so reserved-column quoting is initialized
	// too. Only the isolated in-memory database is touched; migrate test tables.
	t.Setenv("SQL_DSN", "")
	t.Setenv("LOG_SQL_DSN", "")
	common.SQLitePath, common.IsMasterNode = ":memory:", false
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL = true, false, false
	common.RedisEnabled, common.BatchUpdateEnabled, common.LogConsumeEnabled = false, false, true
	require.NoError(t, model.InitDB())
	db := model.DB
	model.LOG_DB = db
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Channel{}, &model.Log{}))
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "steering-test", Quota: steeringTestQuota}).Error)
	require.NoError(t, db.Create(&model.Token{Id: 1, UserId: 1, Key: "steering-test", RemainQuota: steeringTestQuota}).Error)
	require.NoError(t, db.Create(&model.Channel{Id: 1, Name: "steering-test"}).Error)
	c, info := newResponsesWebSocketTestContext(t)
	common.SetContextKey(c, constant.ContextKeyUserId, 1)
	common.SetContextKey(c, constant.ContextKeyTokenId, 1)
	common.SetContextKey(c, constant.ContextKeyTokenKey, "steering-test")
	common.SetContextKey(c, constant.ContextKeyUserGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUsingGroup, "default")
	common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{BillingPreference: "wallet_only", QuotaWarningThreshold: -1})
	common.SetContextKey(c, constant.ContextKeyChannelModelMapping, `{"gpt-4o":"upstream-model"}`)
	c.Set(common.RequestIdKey, info.RequestId)
	client, downstream := responsesWebSocketPair(t)
	upstream, provider := responsesWebSocketPair(t)
	info.ClientWs, info.TargetWs = downstream, upstream
	s := &responsesWebSocketSession{
		c: c, connectionInfo: info, client: downstream, upstream: upstream,
		state: responsesWebSocketStateIdle, connectionModel: "gpt-4o",
	}
	return s, client, provider, db
}

func readSteeringFrame(t *testing.T, conn *websocket.Conn) string {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(3*time.Second)))
	_, payload, err := conn.ReadMessage()
	require.NoError(t, err)
	return string(payload)
}

func sendSteeringUpstreamEvent(t *testing.T, s *responsesWebSocketSession, client *websocket.Conn, event string) {
	t.Helper()
	require.NoError(t, s.handleUpstreamFrame(websocket.TextMessage, []byte(event)))
	require.JSONEq(t, event, readSteeringFrame(t, client))
}

func startSteeringTestRound(t *testing.T, s *responsesWebSocketSession, client, provider *websocket.Conn) {
	t.Helper()
	require.Nil(t, s.handleClientFrame(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-4o","input":"Initial request","instructions":"Keep it short","temperature":0,"max_output_tokens":20,"tools":[{"type":"web_search_preview"}]}`)))
	payload := readSteeringFrame(t, provider)
	require.Contains(t, payload, `"model":"upstream-model"`)
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.created","response":{"id":"resp_1"}}`)
}

func sendSteeringSubmission(t *testing.T, s *responsesWebSocketSession, provider *websocket.Conn, responseID, input string) {
	t.Helper()
	payload := fmt.Sprintf(`{"type":"response.steer","previous_response_id":%q,"input":%q}`, responseID, input)
	require.Nil(t, s.handleClientFrame(websocket.TextMessage, []byte(payload)))
	// No model, overrides, stream_id or other create parameters may leak in.
	require.Equal(t, payload, readSteeringFrame(t, provider))
}

func assertSteeringBilling(t *testing.T, db *gorm.DB, prompts, completions []int) {
	t.Helper()
	var logs []model.Log
	require.NoError(t, db.Order("id").Find(&logs).Error)
	require.Len(t, logs, len(prompts))
	total := 0
	for i, log := range logs {
		require.Equal(t, fmt.Sprintf("request-test-%d", i+1), log.RequestId)
		require.Equal(t, prompts[i], log.PromptTokens)
		require.Equal(t, completions[i], log.CompletionTokens)
		require.Positive(t, log.Quota)
		require.Equal(t, "gpt-4o", log.ModelName)
		total += log.Quota
	}
	var user model.User
	var token model.Token
	require.NoError(t, db.First(&user, 1).Error)
	require.NoError(t, db.First(&token, 1).Error)
	require.Equal(t, steeringTestQuota-total, user.Quota)
	require.Equal(t, steeringTestQuota-total, token.RemainQuota)
	require.Equal(t, total, user.UsedQuota)
	require.Equal(t, total, token.UsedQuota)
}

func TestResponsesWebSocketSteeringAutomaticSuccessorsBilledSeparately(t *testing.T) {
	s, client, provider, db := newSteeringTestSession(t)
	startSteeringTestRound(t, s, client, provider)
	originalBilling := s.round.Billing
	sendSteeringSubmission(t, s, provider, "resp_1", "Use Go")
	sendSteeringSubmission(t, s, provider, "resp_1", "Add tests")
	for i := 1; i <= 2; i++ {
		sendSteeringUpstreamEvent(t, s, client, fmt.Sprintf(`{"type":"response.steer.accepted","steer":{"id":"steer_%d","previous_response_id":"resp_1"}}`, i))
	}
	require.Same(t, originalBilling, s.round.Billing)
	require.EqualValues(t, 1, s.roundNumber)
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.incomplete","response":{"id":"resp_1","incomplete_details":{"reason":"steered"},"usage":{"input_tokens":20,"output_tokens":3,"total_tokens":23}}}`)
	// The parent is billed, but no independent create may overtake its successor.
	apiErr := s.handleClientFrame(websocket.TextMessage, []byte(`{"type":"response.create","model":"gpt-4o","input":"Overtake"}`))
	require.NotNil(t, apiErr)
	require.Equal(t, http.StatusConflict, apiErr.StatusCode)
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.created","response":{"id":"resp_2","previous_response_id":"resp_1"}}`)
	require.NotSame(t, originalBilling, s.round.Billing)
	require.Equal(t, "request-test-2", s.round.RequestId)
	request := s.round.Request.(*dto.OpenAIResponsesRequest)
	require.JSONEq(t, `[{"role":"user","content":"Use Go"},{"role":"user","content":"Add tests"}]`, string(request.Input))
	require.JSONEq(t, `"Keep it short"`, string(request.Instructions))
	require.NotNil(t, request.Temperature)
	require.Zero(t, *request.Temperature)
	require.EqualValues(t, 20, *request.MaxOutputTokens)
	require.Empty(t, s.steering.pending)
	require.Zero(t, s.round.ResponsesUsageInfo.BuiltInTools["web_search_preview"].CallCount)
	sendSteeringSubmission(t, s, provider, "resp_2", "Also document it")
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.steer.accepted","steer":{"id":"steer_3","previous_response_id":"resp_2"}}`)
	// Normal completion can also be followed by an automatic continuation.
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.completed","response":{"id":"resp_2","usage":{"input_tokens":30,"output_tokens":4,"total_tokens":34}}}`)
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.created","response":{"id":"resp_3","previous_response_id":"resp_2"}}`)
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.completed","response":{"id":"resp_3","usage":{"input_tokens":40,"output_tokens":5,"total_tokens":45}}}`)
	require.Nil(t, s.round)
	require.Equal(t, responsesWebSocketStateIdle, s.state)
	assertSteeringBilling(t, db, []int{20, 30, 40}, []int{3, 4, 5})
}

func TestResponsesWebSocketSteeringFailuresDoNotRefundActiveRound(t *testing.T) {
	s, client, provider, db := newSteeringTestSession(t)
	startSteeringTestRound(t, s, client, provider)
	billing := s.round.Billing
	sendSteeringSubmission(t, s, provider, "resp_1", "Rejected")
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.steer.failed","error":{"code":"steering_not_supported"},"steer":{"previous_response_id":"resp_1","input":"Rejected"}}`)
	require.Empty(t, s.steering.pending)
	sendSteeringSubmission(t, s, provider, "resp_1", "Accepted then failed")
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.steer.accepted","steer":{"id":"steer_1","previous_response_id":"resp_1"}}`)
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.steer.failed","error":{"code":"successor_creation_failed"},"steer":{"id":"steer_1","previous_response_id":"resp_1","input":"Accepted then failed"}}`)
	require.Empty(t, s.steering.pending)
	require.Same(t, billing, s.round.Billing)
	require.True(t, billing.NeedsRefund())
	require.Equal(t, responsesWebSocketStateGenerating, s.state)
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.completed","response":{"id":"resp_1","usage":{"input_tokens":20,"output_tokens":3,"total_tokens":23}}}`)
	assertSteeringBilling(t, db, []int{20}, []int{3})
}

func TestResponsesWebSocketSteeringToolContinuationDoesNotReplayInput(t *testing.T) {
	for _, notifyPending := range []bool{true, false} {
		t.Run(fmt.Sprint(notifyPending), func(t *testing.T) {
			s, client, provider, db := newSteeringTestSession(t)
			startSteeringTestRound(t, s, client, provider)
			sendSteeringSubmission(t, s, provider, "resp_1", "Add tests")
			sendSteeringUpstreamEvent(t, s, client, `{"type":"response.steer.accepted","steer":{"id":"steer_1","previous_response_id":"resp_1"}}`)
			sendSteeringUpstreamEvent(t, s, client, `{"type":"response.completed","response":{"id":"resp_1","output":[{"type":"function_call","call_id":"call_1","name":"shell","arguments":"{}"}],"usage":{"input_tokens":20,"output_tokens":3,"total_tokens":23}}}`)
			if notifyPending {
				sendSteeringUpstreamEvent(t, s, client, `{"type":"response.steer.pending","reason":"waiting_for_required_input","steer":{"id":"steer_1","previous_response_id":"resp_1"},"required_input":[{"type":"function_call_output","call_id":"call_1"}]}`)
			}
			// Tool results may be submitted before response.steer.pending arrives.
			require.Nil(t, s.handleClientFrame(websocket.TextMessage, []byte(`{"type":"response.create","previous_response_id":"resp_1","instructions":"New instructions","input":[{"type":"function_call_output","call_id":"call_1","output":"done"}]}`)))
			payload := readSteeringFrame(t, provider)
			require.NotContains(t, payload, "Add tests")
			require.Contains(t, payload, "New instructions")
			require.Contains(t, string(s.round.Request.(*dto.OpenAIResponsesRequest).Input), "Add tests")
			sendSteeringUpstreamEvent(t, s, client, `{"type":"response.created","response":{"id":"resp_2","previous_response_id":"resp_1"}}`)
			sendSteeringUpstreamEvent(t, s, client, `{"type":"response.completed","response":{"id":"resp_2","usage":{"input_tokens":30,"output_tokens":4,"total_tokens":34}}}`)
			assertSteeringBilling(t, db, []int{20, 30}, []int{3, 4})
		})
	}
}

func TestResponsesWebSocketSteeringAfterParentCompletionAndQuotaFailure(t *testing.T) {
	s, client, provider, db := newSteeringTestSession(t)
	startSteeringTestRound(t, s, client, provider)
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.completed","response":{"id":"resp_1","usage":{"input_tokens":20,"output_tokens":3,"total_tokens":23}}}`)
	sendSteeringSubmission(t, s, provider, "resp_1", "More work")
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.steer.accepted","steer":{"id":"steer_1","previous_response_id":"resp_1"}}`)
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 1).Update("quota", 0).Error)
	err := s.handleUpstreamFrame(websocket.TextMessage, []byte(`{"type":"response.created","response":{"id":"resp_2","previous_response_id":"resp_1"}}`))
	require.Error(t, err)
	apiErr, ok := err.(*types.NewAPIError)
	require.True(t, ok)
	require.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	require.Nil(t, s.round)
	require.EqualValues(t, 1, s.roundNumber)
	var count int64
	require.NoError(t, db.Model(&model.Log{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestResponsesWebSocketSteeringDisconnectRefundsOnlySuccessor(t *testing.T) {
	s, client, provider, db := newSteeringTestSession(t)
	startSteeringTestRound(t, s, client, provider)
	sendSteeringSubmission(t, s, provider, "resp_1", "More work")
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.steer.accepted","steer":{"id":"steer_1","previous_response_id":"resp_1"}}`)
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.incomplete","response":{"id":"resp_1","incomplete_details":{"reason":"steered"},"usage":{"input_tokens":20,"output_tokens":3,"total_tokens":23}}}`)
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.created","response":{"id":"resp_2","previous_response_id":"resp_1"}}`)
	s.refundRound()
	var log model.Log
	require.NoError(t, db.First(&log).Error)
	require.Eventually(t, func() bool {
		var user model.User
		var token model.Token
		return db.First(&user, 1).Error == nil && db.First(&token, 1).Error == nil &&
			user.Quota == steeringTestQuota-log.Quota && token.RemainQuota == steeringTestQuota-log.Quota
	}, 3*time.Second, 10*time.Millisecond)
	assertSteeringBilling(t, db, []int{20}, []int{3})
}

func TestResponsesWebSocketSteeringRunClosesOnSuccessorQuotaFailure(t *testing.T) {
	s, client, provider, db := newSteeringTestSession(t)
	finished := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		defer close(finished)
		result <- RunResponsesWebSocket(s.c, s.connectionInfo, []byte(`{"type":"response.create","model":"gpt-4o","input":"Initial request"}`))
	}()
	t.Cleanup(func() {
		_ = client.Close()
		_ = provider.Close()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			t.Error("WebSocket relay did not stop")
		}
	})
	readSteeringFrame(t, provider)
	for _, event := range []string{
		`{"type":"response.created","response":{"id":"resp_1"}}`,
		`{"type":"response.completed","response":{"id":"resp_1","usage":{"input_tokens":20,"output_tokens":3,"total_tokens":23}}}`,
	} {
		require.NoError(t, provider.WriteMessage(websocket.TextMessage, []byte(event)))
		require.JSONEq(t, event, readSteeringFrame(t, client))
	}
	// The task may still be running tools after its model response completes.
	steer := `{"type":"response.steer","previous_response_id":"resp_1","input":"More work"}`
	require.NoError(t, client.WriteMessage(websocket.TextMessage, []byte(steer)))
	require.Equal(t, steer, readSteeringFrame(t, provider))
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 1).Update("quota", 0).Error)
	require.NoError(t, provider.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.created","response":{"id":"resp_2","previous_response_id":"resp_1"}}`)))
	var failure dto.OpenAIResponsesWebSocketErrorEvent
	require.NoError(t, common.Unmarshal([]byte(readSteeringFrame(t, client)), &failure))
	require.Equal(t, "error", failure.Type)
	require.Equal(t, http.StatusForbidden, failure.Status)
	require.Equal(t, string(types.ErrorCodeInsufficientUserQuota), failure.Error.Code)
	select {
	case err := <-result:
		require.Error(t, err)
	case <-time.After(3 * time.Second):
		t.Fatal("relay kept generating after quota failure")
	}
	_, _, err := provider.ReadMessage()
	require.Error(t, err)
	var count int64
	require.NoError(t, db.Model(&model.Log{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestResponsesWebSocketSteeringRejectsInvalidTargetsWithoutChangingRound(t *testing.T) {
	s, client, provider, db := newSteeringTestSession(t)
	startSteeringTestRound(t, s, client, provider)
	round := s.round
	for _, raw := range []string{
		`{"type":"response.steer","previous_response_id":"other_connection","input":"Wrong target"}`,
		`{"type":"response.steer","previous_response_id":"resp_1","input":"No stream override","stream_id":"other"}`,
		`{"type":"response.steer","previous_response_id":"resp_1","input":[{"role":"system","content":"Wrong role"}]}`,
	} {
		require.NotNil(t, s.handleClientFrame(websocket.TextMessage, []byte(raw)))
		require.Same(t, round, s.round)
		require.Empty(t, s.steering.pending)
	}
	// Invalid messages did not reach the provider or poison the connection.
	sendSteeringSubmission(t, s, provider, "resp_1", "Valid")
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.steer.failed","steer":{"previous_response_id":"resp_1","input":"Valid"}}`)
	sendSteeringUpstreamEvent(t, s, client, `{"type":"response.completed","response":{"id":"resp_1","usage":{"input_tokens":20,"output_tokens":3,"total_tokens":23}}}`)
	assertSteeringBilling(t, db, []int{20}, []int{3})
}
