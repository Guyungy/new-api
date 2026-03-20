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
	})
}

func requestAuditPath(c *gin.Context) string {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return ""
	}
	return c.Request.URL.Path
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
