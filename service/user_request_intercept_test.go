package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func buildRequestInterceptionContext(t *testing.T, policy dto.UserRequestInterception) *gin.Context {
	t.Helper()
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	common.SetContextKey(ctx, constant.ContextKeyUserSetting, dto.UserSetting{
		RequestInterception: policy,
	})
	return ctx
}

func mustMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := common.Marshal(value)
	require.NoError(t, err)
	return data
}

func TestApplyUserRequestInterception_IgnoreBlocksRequest(t *testing.T) {
	ctx := buildRequestInterceptionContext(t, dto.UserRequestInterception{
		Enabled:        true,
		Mode:           dto.UserRequestInterceptModeIgnore,
		MatchKeywords:  []string{"blocked"},
		IgnoreResponse: "nope",
	})
	request := &dto.GeneralOpenAIRequest{
		Messages: []dto.Message{{
			Role:    "user",
			Content: "please block this blocked request",
		}},
	}

	err := ApplyUserRequestInterception(ctx, request)

	require.NotNil(t, err)
	require.Equal(t, http.StatusForbidden, err.StatusCode)
	require.Equal(t, types.ErrorCodePromptBlocked, err.GetErrorCode())
	require.Equal(t, "nope", err.Error())
	require.Equal(t, "user_request_interception=ignore", common.GetContextKeyString(ctx, constant.ContextKeyAdminRejectReason))
}

func TestApplyUserRequestInterception_InjectPrependsResponsesInstructions(t *testing.T) {
	ctx := buildRequestInterceptionContext(t, dto.UserRequestInterception{
		Enabled:      true,
		Mode:         dto.UserRequestInterceptModeInject,
		InjectPrompt: "follow policy",
	})
	request := &dto.OpenAIResponsesRequest{
		Model:        "gpt-5",
		Input:        mustMarshalJSON(t, "hello"),
		Instructions: mustMarshalJSON(t, "answer briefly"),
	}

	err := ApplyUserRequestInterception(ctx, request)

	require.Nil(t, err)
	var instructions string
	require.NoError(t, common.Unmarshal(request.Instructions, &instructions))
	require.Equal(t, "follow policy\nanswer briefly", instructions)
}

func TestApplyUserRequestInterception_ReplaceUpdatesResponsesInputText(t *testing.T) {
	ctx := buildRequestInterceptionContext(t, dto.UserRequestInterception{
		Enabled: true,
		Mode:    dto.UserRequestInterceptModeReplace,
		ReplaceRules: []dto.UserInterceptionReplaceRule{{
			From: "foo",
			To:   "bar",
		}},
	})
	request := &dto.OpenAIResponsesRequest{
		Model: "gpt-5",
		Input: mustMarshalJSON(t, []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "input_text",
						"text": "foo request",
					},
				},
			},
		}),
	}

	err := ApplyUserRequestInterception(ctx, request)

	require.Nil(t, err)
	inputs := request.ParseInput()
	require.Len(t, inputs, 1)
	require.Equal(t, "bar request", inputs[0].Text)
}

func TestApplyUserRequestInterception_InjectGeminiCountsSystemInstruction(t *testing.T) {
	ctx := buildRequestInterceptionContext(t, dto.UserRequestInterception{
		Enabled:      true,
		Mode:         dto.UserRequestInterceptModeInject,
		InjectPrompt: "safety first",
	})
	request := &dto.GeminiChatRequest{
		Contents: []dto.GeminiChatContent{{
			Parts: []dto.GeminiPart{{
				Text: "hello world",
			}},
		}},
	}

	err := ApplyUserRequestInterception(ctx, request)

	require.Nil(t, err)
	require.NotNil(t, request.SystemInstructions)
	require.Equal(t, "safety first", request.SystemInstructions.Parts[0].Text)
	require.Contains(t, request.GetTokenCountMeta().CombineText, "safety first")
}

func TestValidateUserRequestInterception_RejectsEmptyEnabledInject(t *testing.T) {
	err := ValidateUserRequestInterception(dto.UserRequestInterception{
		Enabled: true,
		Mode:    dto.UserRequestInterceptModeInject,
	})

	require.EqualError(t, err, "inject prompt is required when mode=inject")
}
