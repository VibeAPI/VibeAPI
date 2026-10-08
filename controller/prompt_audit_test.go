package controller

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromptAuditSettingsResponseSerializesNilListsAsEmptyArrays(t *testing.T) {
	response := promptAuditSettingsResponse(operation_setting.PromptAuditSetting{})

	payload, err := common.Marshal(response)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"protected_channel_ids":[]`)
	require.Contains(t, string(payload), `"audience_user_ids":[]`)
}

func TestPromptAuditSettingsResponseDataSerializesNilRestrictedUsersAsEmptyArray(t *testing.T) {
	response := promptAuditSettingsResponseData{
		promptAuditSettingsInput: promptAuditSettingsResponse(operation_setting.PromptAuditSetting{}),
		RestrictedUsers:          []model.ChannelBlacklistUser{},
	}

	payload, err := common.Marshal(response)
	require.NoError(t, err)
	require.Contains(t, string(payload), `"restricted_users":[]`)
}

// Use real channel selection and a local audit endpoint. The submit callback
// stops at the upstream boundary so rejection tests cannot create billable tasks.
func TestTaskSubmissionPromptAudit(t *testing.T) {
	for _, tc := range []struct {
		name, mode             string
		flagged, retry, locked bool
		status                 int
	}{
		{"reject", "reject", true, false, false, http.StatusForbidden},
		{"allow", "reject", false, false, false, http.StatusTeapot},
		{"downgrade", "downgrade", true, false, false, http.StatusTeapot},
		{"origin task cannot downgrade", "downgrade", true, false, true, http.StatusServiceUnavailable},
		{"retry onto protected channel", "reject", true, true, false, http.StatusForbidden},
		{"audit unavailable", "reject", false, false, false, http.StatusTeapot},
		{"disabled", "reject", true, false, false, http.StatusTeapot},
		{"outside audience", "reject", true, false, false, http.StatusTeapot},
		{"typed task", "reject", true, false, false, http.StatusForbidden},
		{"incompatible fallback", "downgrade", true, false, false, http.StatusServiceUnavailable},
		{"no fallback", "downgrade", true, false, false, http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dialect := os.Getenv("TEST_TASK_DB_DIALECT")
			if dialect == "" {
				dialect = "sqlite"
			}
			dsn := os.Getenv("TEST_MYSQL_DSN")
			if dialect == "postgres" {
				dsn = os.Getenv("TEST_POSTGRES_DSN")
			}
			db := modelManagementDB(t, dialect, dsn)
			oldSetting := *operation_setting.GetPromptAuditSetting()
			oldRetries := common.RetryTimes
			t.Cleanup(func() { *operation_setting.GetPromptAuditSetting() = oldSetting; common.RetryTimes = oldRetries })
			common.RetryTimes = 1
			var auditCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				auditCalls.Add(1)
				if tc.name == "audit unavailable" {
					w.WriteHeader(http.StatusBadGateway)
					return
				}
				body, err := io.ReadAll(r.Body)
				assert.NoError(t, err)
				assert.Contains(t, string(body), "audit this task")
				_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"content":"{\"flagged\":%t,\"confidence\":1,\"categories\":[],\"reason\":\"test\"}"}}]}`, tc.flagged)
			}))
			defer server.Close()
			key, err := common.EncryptSecret("audit-test-key")
			require.NoError(t, err)
			protected := &model.Channel{Name: "protected", Type: constant.ChannelTypeOpenAI, Key: "test", Status: common.ChannelStatusEnabled, Group: "default", Models: "audit-task", Priority: common.GetPointer(int64(10))}
			fallback := &model.Channel{Name: "fallback", Type: constant.ChannelTypeOpenAI, Key: "test", Status: common.ChannelStatusEnabled, Group: "default", Models: "audit-task", Priority: common.GetPointer(int64(5))}
			if tc.retry {
				fallback.Priority = common.GetPointer(int64(20))
			}
			require.NoError(t, protected.Insert())
			require.NoError(t, fallback.Insert())
			if tc.name == "no fallback" {
				require.NoError(t, db.Model(&model.Ability{}).Where("channel_id = ?", fallback.Id).Update("enabled", false).Error)
			}
			*operation_setting.GetPromptAuditSetting() = operation_setting.PromptAuditSetting{Enabled: true, Mode: tc.mode, AudienceUserIds: []int{7}, ProtectedChannelIds: []int{protected.Id}, MainThreshold: 0.7, AllowPrivateEndpoints: true, Main: operation_setting.PromptAuditEndpoint{URL: server.URL, APIKeyEncrypted: key, Model: "audit"}}
			if tc.name == "disabled" {
				operation_setting.GetPromptAuditSetting().Enabled = false
			}
			if tc.name == "outside audience" {
				operation_setting.GetPromptAuditSetting().AudienceUserIds = []int{8}
			}
			c := taskSubmissionTestContext()
			c.Set("task_request", map[string]any{"prompt": "audit this task"})
			if tc.name == "typed task" {
				c.Set("task_request", relaycommon.TaskSubmitReq{Prompt: "audit this task"})
			}
			c.Set("group", "default")
			c.Set("user_group", "default")
			c.Set("id", 7)
			info := taskSubmissionRelayInfo(nil)
			info.UserId = 7
			info.TokenGroup = "default"
			info.OriginModelName = "audit-task"
			info.LockedChannel = nil
			if tc.name == "incompatible fallback" {
				service.GetChannelConstraints(c).AddFilter(taskdto.ChannelFilter{Kind: taskdto.FilterTaskPluginIdentity, TaskPluginKey: "another-plugin", TaskPluginChannelTypes: []int{constant.ChannelTypeTaskPlugin}})
				info.LockedChannel = protected
			}
			if tc.locked {
				info.LockedChannel = protected
				info.OriginTaskID = "origin"
			}
			submitted := []int{}
			_, taskErr := executeTaskSubmissionWith(c, info, func(c *gin.Context, i *relaycommon.RelayInfo) (*relay.TaskSubmitResult, *taskdto.TaskError) {
				submitted = append(submitted, c.GetInt("channel_id"))
				if tc.retry && len(submitted) == 1 {
					return nil, &taskdto.TaskError{Code: "upstream_error", Message: "retry", StatusCode: http.StatusBadGateway}
				}
				return nil, service.TaskErrorWrapperLocal(errors.New("test submit boundary"), "test_stop", http.StatusTeapot)
			})
			require.NotNil(t, taskErr)
			assert.Equal(t, tc.status, taskErr.StatusCode)
			wantCalls := int32(1)
			if tc.name == "disabled" || tc.name == "outside audience" {
				wantCalls = 0
			}
			assert.Equal(t, wantCalls, auditCalls.Load())
			switch {
			case tc.name == "audit unavailable":
				assert.Equal(t, []int{fallback.Id}, submitted)
			case wantCalls == 0:
				assert.Equal(t, []int{protected.Id}, submitted)
			case tc.retry:
				assert.Equal(t, []int{fallback.Id}, submitted)
			case tc.mode == "downgrade" && tc.status == http.StatusTeapot:
				assert.Equal(t, []int{fallback.Id}, submitted)
			case !tc.flagged:
				assert.Equal(t, []int{protected.Id}, submitted)
			default:
				assert.Empty(t, submitted)
			}
			if tc.flagged && wantCalls > 0 {
				var stored model.Channel
				require.NoError(t, db.First(&stored, protected.Id).Error)
				assert.True(t, stored.GetOtherSettings().IsUserBlacklisted(7))
			}
		})
	}
}

func TestRelayAuditsProtectedChannelSelectedOnRetry(t *testing.T) {
	dialect := os.Getenv("TEST_TASK_DB_DIALECT")
	if dialect == "" {
		dialect = "sqlite"
	}
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dialect == "postgres" {
		dsn = os.Getenv("TEST_POSTGRES_DSN")
	}
	db := modelManagementDB(t, dialect, dsn)
	oldSetting := *operation_setting.GetPromptAuditSetting()
	oldRetries, oldErrorLog, oldCount, oldConsume := common.RetryTimes, constant.ErrorLogEnabled, constant.CountToken, common.LogConsumeEnabled
	t.Cleanup(func() {
		*operation_setting.GetPromptAuditSetting() = oldSetting
		common.RetryTimes = oldRetries
		constant.ErrorLogEnabled = oldErrorLog
		constant.CountToken = oldCount
		common.LogConsumeEnabled = oldConsume
	})
	common.RetryTimes = 1
	constant.ErrorLogEnabled = false
	constant.CountToken = false
	common.LogConsumeEnabled = false
	oldFreePreconsume := operation_setting.GetQuotaSetting().EnableFreeModelPreConsume
	operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = false
	t.Cleanup(func() { operation_setting.GetQuotaSetting().EnableFreeModelPreConsume = oldFreePreconsume })
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"audit-chat":0}`))
	service.InitHttpClient()
	var auditCalls, firstCalls, protectedCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/audit":
			auditCalls.Add(1)
			_, _ = io.WriteString(w, `{"choices":[{"message":{"content":"{\"flagged\":true,\"confidence\":1,\"categories\":[],\"reason\":\"test\"}"}}]}`)
		case "/first/v1/chat/completions":
			firstCalls.Add(1)
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, `{"error":{"message":"retry","type":"server_error"}}`)
		default:
			protectedCalls.Add(1)
			_, _ = io.WriteString(w, `{"id":"chat_1","model":"audit-chat","choices":[{"message":{"role":"assistant","content":"should not be sent"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
		}
	}))
	defer upstream.Close()
	protected := &model.Channel{Name: "protected", Type: constant.ChannelTypeOpenAI, Key: "test", Status: common.ChannelStatusEnabled, Group: "default", Models: "audit-chat", Priority: common.GetPointer(int64(5)), BaseURL: common.GetPointer(upstream.URL + "/protected")}
	first := &model.Channel{Name: "first", Type: constant.ChannelTypeOpenAI, Key: "test", Status: common.ChannelStatusEnabled, Group: "default", Models: "audit-chat", Priority: common.GetPointer(int64(10)), BaseURL: common.GetPointer(upstream.URL + "/first")}
	require.NoError(t, protected.Insert())
	require.NoError(t, first.Insert())
	key, err := common.EncryptSecret("test-key")
	require.NoError(t, err)
	*operation_setting.GetPromptAuditSetting() = operation_setting.PromptAuditSetting{Enabled: true, Mode: "reject", AudienceUserIds: []int{7}, ProtectedChannelIds: []int{protected.Id}, MainThreshold: 0.7, AllowPrivateEndpoints: true, Main: operation_setting.PromptAuditEndpoint{URL: upstream.URL + "/audit", APIKeyEncrypted: key, Model: "audit"}}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"audit-chat","messages":[{"role":"user","content":"audit this chat"}]}`))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("group", "default")
	c.Set("user_group", "default")
	c.Set("id", 7)
	common.SetContextKey(c, constant.ContextKeyUserSetting, dto.UserSetting{BillingPreference: "wallet_only"})
	require.Nil(t, middleware.SetupContextForSelectedChannel(c, first, "audit-chat"))
	Relay(c, types.RelayFormatOpenAI)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Equal(t, int32(1), firstCalls.Load())
	assert.Equal(t, int32(1), auditCalls.Load())
	assert.Zero(t, protectedCalls.Load())
	var stored model.Channel
	require.NoError(t, db.First(&stored, protected.Id).Error)
	assert.True(t, stored.GetOtherSettings().IsUserBlacklisted(7))
}
