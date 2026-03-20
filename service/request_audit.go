package service

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

const requestAuditCaptureLimit = 256 * 1024

type requestAuditCaptureWriter struct {
	gin.ResponseWriter
	buffer bytes.Buffer
	limit  int
}

func (w *requestAuditCaptureWriter) Write(data []byte) (int, error) {
	w.capture(data)
	return w.ResponseWriter.Write(data)
}

func (w *requestAuditCaptureWriter) WriteString(s string) (int, error) {
	w.capture(common.StringToByteSlice(s))
	return w.ResponseWriter.WriteString(s)
}

func (w *requestAuditCaptureWriter) capture(data []byte) {
	if w == nil || len(data) == 0 {
		return
	}
	remaining := w.limit - w.buffer.Len()
	if remaining <= 0 {
		return
	}
	if len(data) > remaining {
		data = data[:remaining]
	}
	_, _ = w.buffer.Write(data)
}

type RequestAuditCapture struct {
	writer *requestAuditCaptureWriter
}

type requestAuditTrace struct {
	AuditID         string
	ParentRequestID string
	SessionID       string
	ConversationID  string
}

func (c *RequestAuditCapture) Bytes() []byte {
	if c == nil || c.writer == nil {
		return nil
	}
	return c.writer.buffer.Bytes()
}

func (c *RequestAuditCapture) Status() int {
	if c == nil || c.writer == nil {
		return 0
	}
	return c.writer.Status()
}

func ShouldRecordRequestAudit(request dto.Request, relayFormat types.RelayFormat) bool {
	if request == nil {
		return false
	}
	switch relayFormat {
	case types.RelayFormatOpenAI,
		types.RelayFormatClaude,
		types.RelayFormatGemini,
		types.RelayFormatOpenAIResponses,
		types.RelayFormatOpenAIResponsesCompaction:
	default:
		return false
	}
	switch request.(type) {
	case *dto.GeneralOpenAIRequest,
		*dto.OpenAIResponsesRequest,
		*dto.OpenAIResponsesCompactionRequest,
		*dto.ClaudeRequest,
		*dto.GeminiChatRequest:
		return true
	default:
		return false
	}
}

func InstallRequestAuditCapture(c *gin.Context) *RequestAuditCapture {
	if c == nil || c.Writer == nil {
		return nil
	}
	writer := &requestAuditCaptureWriter{
		ResponseWriter: c.Writer,
		limit:          requestAuditCaptureLimit,
	}
	c.Writer = writer
	return &RequestAuditCapture{writer: writer}
}

func RecordRequestAuditAfterRelay(c *gin.Context, request dto.Request, relayFormat types.RelayFormat, relayInfo *relaycommon.RelayInfo, capture *RequestAuditCapture, relayErr *types.NewAPIError) {
	if c == nil || request == nil || relayErr != nil || !ShouldRecordRequestAudit(request, relayFormat) {
		return
	}
	userId := c.GetInt("id")
	if userId == 0 {
		return
	}

	mode := common.GetContextKeyString(c, constant.ContextKeyRequestAuditMode)
	if mode == dto.UserRequestInterceptModeIgnore {
		return
	}
	if mode == "" {
		mode = "normal"
	}

	matchedKeywords, _ := common.GetContextKeyType[[]string](c, constant.ContextKeyRequestAuditMatchedKeywords)
	originalRequestText := strings.TrimSpace(common.GetContextKeyString(c, constant.ContextKeyRequestAuditOriginalText))
	finalRequestText := strings.TrimSpace(extractRequestText(request))
	if finalRequestText == "" {
		finalRequestText = originalRequestText
	}
	if originalRequestText == finalRequestText {
		originalRequestText = ""
	}

	responseText := ""
	statusCode := 0
	if capture != nil {
		responseText = extractRequestAuditResponseText(relayFormat, capture.Bytes(), request.IsStream(c))
		statusCode = capture.Status()
	}

	modelName := extractRequestModel(request)
	if relayInfo != nil && relayInfo.OriginModelName != "" {
		modelName = relayInfo.OriginModelName
	}

	group := c.GetString("group")
	if relayInfo != nil && relayInfo.UsingGroup != "" {
		group = relayInfo.UsingGroup
	}
	trace := buildRequestAuditTrace(request)

	model.RecordRequestAuditLog(c, userId, model.RecordRequestAuditLogParams{
		ModelName:           modelName,
		TokenName:           c.GetString("token_name"),
		TokenId:             c.GetInt("token_id"),
		Group:               group,
		Mode:                mode,
		Status:              "completed",
		MatchedKeywords:     matchedKeywords,
		OriginalRequestText: originalRequestText,
		FinalRequestText:    finalRequestText,
		ResponseText:        responseText,
		RequestPath:         requestAuditPath(c),
		StatusCode:          statusCode,
		AuditID:             trace.AuditID,
		ParentRequestID:     trace.ParentRequestID,
		SessionID:           trace.SessionID,
		ConversationID:      trace.ConversationID,
	})
}

func requestAuditPath(c *gin.Context) string {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return ""
	}
	return c.Request.URL.Path
}

func buildRequestAuditTrace(request dto.Request) requestAuditTrace {
	trace := requestAuditTrace{
		AuditID: "audit_" + common.GetUUID(),
	}
	if request == nil {
		return trace
	}

	switch typed := request.(type) {
	case *dto.GeneralOpenAIRequest:
		mergeRequestAuditTrace(&trace, extractRequestAuditTraceFromRaw(typed.Metadata))
	case *dto.OpenAIResponsesRequest:
		mergeRequestAuditTrace(&trace, extractRequestAuditTraceFromRaw(typed.Metadata))
		trace.ParentRequestID = requestAuditFirstNonEmpty(trace.ParentRequestID, typed.PreviousResponseID)
		trace.ConversationID = requestAuditFirstNonEmpty(
			trace.ConversationID,
			extractRequestAuditTraceIdentifier(typed.Conversation, "conversation_id", "conversationId", "id"),
		)
	case *dto.OpenAIResponsesCompactionRequest:
		trace.ParentRequestID = requestAuditFirstNonEmpty(trace.ParentRequestID, typed.PreviousResponseID)
	case *dto.ClaudeRequest:
		mergeRequestAuditTrace(&trace, extractRequestAuditTraceFromRaw(typed.Metadata))
	}

	return trace
}

func mergeRequestAuditTrace(target *requestAuditTrace, source requestAuditTrace) {
	if target == nil {
		return
	}
	target.AuditID = requestAuditFirstNonEmpty(target.AuditID, source.AuditID)
	target.ParentRequestID = requestAuditFirstNonEmpty(target.ParentRequestID, source.ParentRequestID)
	target.SessionID = requestAuditFirstNonEmpty(target.SessionID, source.SessionID)
	target.ConversationID = requestAuditFirstNonEmpty(target.ConversationID, source.ConversationID)
}

func extractRequestAuditTraceFromRaw(raw []byte) requestAuditTrace {
	if len(bytes.TrimSpace(raw)) == 0 {
		return requestAuditTrace{}
	}

	var payload any
	if err := common.Unmarshal(raw, &payload); err != nil {
		return requestAuditTrace{}
	}

	object, ok := payload.(map[string]any)
	if !ok {
		return requestAuditTrace{}
	}

	return requestAuditTrace{
		AuditID: requestAuditFirstNonEmpty(
			extractRequestAuditTraceString(object["audit_id"]),
			extractRequestAuditTraceString(object["auditId"]),
		),
		ParentRequestID: requestAuditFirstNonEmpty(
			extractRequestAuditTraceString(object["parent_request_id"]),
			extractRequestAuditTraceString(object["parentRequestId"]),
			extractRequestAuditTraceString(object["previous_response_id"]),
			extractRequestAuditTraceString(object["previousResponseId"]),
			extractRequestAuditTraceString(object["parent_request"], "id", "request_id", "requestId"),
			extractRequestAuditTraceString(object["parentRequest"], "id", "request_id", "requestId"),
			extractRequestAuditTraceString(object["parent"], "id", "request_id", "requestId"),
		),
		SessionID: requestAuditFirstNonEmpty(
			extractRequestAuditTraceString(object["session_id"]),
			extractRequestAuditTraceString(object["sessionId"]),
			extractRequestAuditTraceString(object["session"], "id", "session_id", "sessionId"),
		),
		ConversationID: requestAuditFirstNonEmpty(
			extractRequestAuditTraceString(object["conversation_id"]),
			extractRequestAuditTraceString(object["conversationId"]),
			extractRequestAuditTraceString(object["conversation"], "id", "conversation_id", "conversationId"),
		),
	}
}

func extractRequestAuditTraceIdentifier(raw []byte, keys ...string) string {
	if len(bytes.TrimSpace(raw)) == 0 {
		return ""
	}

	var payload any
	if err := common.Unmarshal(raw, &payload); err != nil {
		return ""
	}

	return extractRequestAuditTraceString(payload, keys...)
}

func extractRequestAuditTraceString(value any, keys ...string) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	case map[string]any:
		for _, key := range keys {
			if candidate := extractRequestAuditTraceString(typed[key]); candidate != "" {
				return candidate
			}
		}
		if candidate := extractRequestAuditTraceString(typed["id"]); candidate != "" {
			return candidate
		}
		if candidate := extractRequestAuditTraceString(typed["value"]); candidate != "" {
			return candidate
		}
	case []any:
		for _, item := range typed {
			if candidate := extractRequestAuditTraceString(item, keys...); candidate != "" {
				return candidate
			}
		}
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, bool:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
	return ""
}

func requestAuditFirstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func extractRequestAuditResponseText(relayFormat types.RelayFormat, body []byte, isStream bool) string {
	payload := strings.TrimSpace(string(body))
	if payload == "" {
		return ""
	}

	if isStream || strings.Contains(payload, "\ndata: ") || strings.HasPrefix(payload, "data: ") {
		if streamText := extractRequestAuditStreamText(relayFormat, payload); streamText != "" {
			return streamText
		}
	}

	switch relayFormat {
	case types.RelayFormatOpenAI:
		var response dto.OpenAITextResponse
		if err := common.Unmarshal(body, &response); err == nil {
			return collectOpenAITextResponse(&response)
		}
	case types.RelayFormatOpenAIResponses, types.RelayFormatOpenAIResponsesCompaction:
		var response dto.OpenAIResponsesResponse
		if err := common.Unmarshal(body, &response); err == nil {
			return collectResponsesOutputText(&response)
		}
	case types.RelayFormatClaude:
		var response dto.ClaudeResponse
		if err := common.Unmarshal(body, &response); err == nil {
			return collectClaudeResponseText(&response)
		}
	case types.RelayFormatGemini:
		var response dto.GeminiChatResponse
		if err := common.Unmarshal(body, &response); err == nil {
			return collectGeminiResponseText(&response)
		}
	}

	var openAIResponse dto.OpenAITextResponse
	if err := common.Unmarshal(body, &openAIResponse); err == nil {
		if text := collectOpenAITextResponse(&openAIResponse); text != "" {
			return text
		}
	}

	return payload
}

func extractRequestAuditStreamText(relayFormat types.RelayFormat, payload string) string {
	lines := strings.Split(strings.ReplaceAll(payload, "\r\n", "\n"), "\n")
	var builder strings.Builder

	appendText := func(text string) {
		text = strings.TrimSpace(text)
		if text == "" {
			return
		}
		builder.WriteString(text)
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		switch relayFormat {
		case types.RelayFormatOpenAI:
			var response dto.ChatCompletionsStreamResponse
			if err := common.UnmarshalJsonStr(data, &response); err == nil {
				appendText(collectOpenAIStreamText(&response))
				continue
			}
		case types.RelayFormatOpenAIResponses, types.RelayFormatOpenAIResponsesCompaction:
			var response dto.ResponsesStreamResponse
			if err := common.UnmarshalJsonStr(data, &response); err == nil {
				appendText(collectResponsesStreamText(&response))
				continue
			}
		case types.RelayFormatClaude:
			var response dto.ClaudeResponse
			if err := common.UnmarshalJsonStr(data, &response); err == nil {
				appendText(collectClaudeStreamText(&response))
				continue
			}
		case types.RelayFormatGemini:
			var response dto.GeminiChatResponse
			if err := common.UnmarshalJsonStr(data, &response); err == nil {
				appendText(collectGeminiResponseText(&response))
				continue
			}
		}

		var openAIChunk dto.ChatCompletionsStreamResponse
		if err := common.UnmarshalJsonStr(data, &openAIChunk); err == nil {
			appendText(collectOpenAIStreamText(&openAIChunk))
			continue
		}

		var claudeChunk dto.ClaudeResponse
		if err := common.UnmarshalJsonStr(data, &claudeChunk); err == nil {
			appendText(collectClaudeStreamText(&claudeChunk))
		}
	}

	return builder.String()
}

func collectOpenAITextResponse(response *dto.OpenAITextResponse) string {
	if response == nil {
		return ""
	}
	var builder strings.Builder
	for _, choice := range response.Choices {
		if choice.Message.ReasoningContent != "" {
			builder.WriteString(choice.Message.ReasoningContent)
		}
		if choice.Message.Reasoning != "" {
			builder.WriteString(choice.Message.Reasoning)
		}
		builder.WriteString(choice.Message.StringContent())
	}
	return builder.String()
}

func collectOpenAIStreamText(response *dto.ChatCompletionsStreamResponse) string {
	if response == nil {
		return ""
	}
	var builder strings.Builder
	for _, choice := range response.Choices {
		builder.WriteString(choice.Delta.GetReasoningContent())
		builder.WriteString(choice.Delta.GetContentString())
	}
	return builder.String()
}

func collectResponsesOutputText(response *dto.OpenAIResponsesResponse) string {
	if response == nil {
		return ""
	}
	var builder strings.Builder
	for _, output := range response.Output {
		for _, content := range output.Content {
			builder.WriteString(content.Text)
		}
		if output.Type == "function_call" && output.Name != "" {
			builder.WriteString(fmt.Sprintf("[tool:%s]%s", output.Name, output.Arguments))
		}
	}
	return builder.String()
}

func collectResponsesStreamText(response *dto.ResponsesStreamResponse) string {
	if response == nil {
		return ""
	}
	var builder strings.Builder
	if response.Delta != "" {
		builder.WriteString(response.Delta)
	}
	if response.Item != nil {
		for _, content := range response.Item.Content {
			builder.WriteString(content.Text)
		}
	}
	return builder.String()
}

func collectClaudeResponseText(response *dto.ClaudeResponse) string {
	if response == nil {
		return ""
	}
	var builder strings.Builder
	for _, content := range response.Content {
		builder.WriteString(content.GetText())
		if content.Thinking != nil {
			builder.WriteString(*content.Thinking)
		}
	}
	return builder.String()
}

func collectClaudeStreamText(response *dto.ClaudeResponse) string {
	if response == nil {
		return ""
	}
	var builder strings.Builder
	if response.ContentBlock != nil {
		builder.WriteString(response.ContentBlock.GetText())
		if response.ContentBlock.Thinking != nil {
			builder.WriteString(*response.ContentBlock.Thinking)
		}
	}
	if response.Delta != nil {
		if response.Delta.Text != nil {
			builder.WriteString(*response.Delta.Text)
		}
		if response.Delta.Thinking != nil {
			builder.WriteString(*response.Delta.Thinking)
		}
	}
	return builder.String()
}

func collectGeminiResponseText(response *dto.GeminiChatResponse) string {
	if response == nil {
		return ""
	}
	var builder strings.Builder
	for _, candidate := range response.Candidates {
		for _, part := range candidate.Content.Parts {
			builder.WriteString(part.Text)
			if part.CodeExecutionResult != nil {
				builder.WriteString(part.CodeExecutionResult.Output)
			}
		}
	}
	return builder.String()
}
