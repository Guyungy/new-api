package service

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

func ApplyUserRequestInterception(c *gin.Context, request dto.Request) *types.NewAPIError {
	if c == nil || request == nil {
		return nil
	}
	userSetting, ok := common.GetContextKeyType[dto.UserSetting](c, constant.ContextKeyUserSetting)
	if !ok {
		return nil
	}
	policy := SanitizeUserRequestInterception(userSetting.RequestInterception)
	if !policy.Enabled || policy.Mode == "" {
		return nil
	}
	requestText := strings.TrimSpace(extractRequestText(request))
	matched, matchedKeywords := policyMatched(policy, requestText)
	if !matched {
		return nil
	}
	recordUserRequestInterceptionLog(c, request, policy, requestText, matchedKeywords)

	switch policy.Mode {
	case dto.UserRequestInterceptModeIgnore:
		message := strings.TrimSpace(policy.IgnoreResponse)
		if message == "" {
			message = "request blocked by user interception policy"
		}
		common.SetContextKey(c, constant.ContextKeyAdminRejectReason, "user_request_interception=ignore")
		return types.NewErrorWithStatusCode(
			errors.New(message),
			types.ErrorCodePromptBlocked,
			http.StatusForbidden,
			types.ErrOptionWithSkipRetry(),
		)
	case dto.UserRequestInterceptModeInject:
		applyInjectPrompt(policy, request)
	case dto.UserRequestInterceptModeReplace:
		applyReplaceRules(policy, request)
	}
	return nil
}

func SanitizeUserRequestInterception(policy dto.UserRequestInterception) dto.UserRequestInterception {
	policy.Mode = strings.ToLower(strings.TrimSpace(policy.Mode))
	policy.InjectPrompt = strings.TrimSpace(policy.InjectPrompt)
	policy.IgnoreResponse = strings.TrimSpace(policy.IgnoreResponse)
	policy.MatchKeywords = filterNonEmptyStrings(policy.MatchKeywords)
	filteredRules := make([]dto.UserInterceptionReplaceRule, 0, len(policy.ReplaceRules))
	for _, rule := range policy.ReplaceRules {
		from := strings.TrimSpace(rule.From)
		to := strings.TrimSpace(rule.To)
		if from == "" {
			continue
		}
		filteredRules = append(filteredRules, dto.UserInterceptionReplaceRule{From: from, To: to})
	}
	policy.ReplaceRules = filteredRules
	return policy
}

func ValidateUserRequestInterception(policy dto.UserRequestInterception) error {
	policy = SanitizeUserRequestInterception(policy)
	if policy.Mode == "" {
		if policy.Enabled {
			return errors.New("request interception mode is required when enabled")
		}
		return nil
	}
	switch policy.Mode {
	case dto.UserRequestInterceptModeIgnore:
		return nil
	case dto.UserRequestInterceptModeInject:
		if policy.Enabled && policy.InjectPrompt == "" {
			return errors.New("inject prompt is required when mode=inject")
		}
		return nil
	case dto.UserRequestInterceptModeReplace:
		if policy.Enabled && len(policy.ReplaceRules) == 0 {
			return errors.New("replace rules are required when mode=replace")
		}
		return nil
	default:
		return fmt.Errorf("unsupported request interception mode: %s", policy.Mode)
	}
}

func filterNonEmptyStrings(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	return filtered
}

func policyMatched(policy dto.UserRequestInterception, requestText string) (bool, []string) {
	if len(policy.MatchKeywords) == 0 {
		return true, nil
	}
	text := strings.ToLower(requestText)
	if text == "" {
		return false, nil
	}
	matchedKeywords := make([]string, 0, len(policy.MatchKeywords))
	for _, keyword := range policy.MatchKeywords {
		if strings.Contains(text, strings.ToLower(keyword)) {
			matchedKeywords = append(matchedKeywords, keyword)
		}
	}
	return len(matchedKeywords) > 0, matchedKeywords
}

func extractRequestText(request dto.Request) string {
	switch req := request.(type) {
	case *dto.GeneralOpenAIRequest:
		return req.GetTokenCountMeta().CombineText
	case *dto.OpenAIResponsesRequest:
		return req.GetTokenCountMeta().CombineText
	case *dto.OpenAIResponsesCompactionRequest:
		return req.GetTokenCountMeta().CombineText
	case *dto.ClaudeRequest:
		return req.GetTokenCountMeta().CombineText
	case *dto.GeminiChatRequest:
		return req.GetTokenCountMeta().CombineText
	default:
		return ""
	}
}

func applyInjectPrompt(policy dto.UserRequestInterception, request dto.Request) {
	if policy.InjectPrompt == "" {
		return
	}
	switch req := request.(type) {
	case *dto.GeneralOpenAIRequest:
		if len(req.Messages) > 0 {
			injected := dto.Message{Role: "system", Content: policy.InjectPrompt}
			req.Messages = append([]dto.Message{injected}, req.Messages...)
			return
		}
		if req.Instruction != "" {
			req.Instruction = prependText(policy.InjectPrompt, req.Instruction)
			return
		}
		if req.Prompt != nil {
			req.Prompt = prependTextToValue(req.Prompt, policy.InjectPrompt)
			return
		}
		req.Input = prependTextToValue(req.Input, policy.InjectPrompt)
	case *dto.OpenAIResponsesRequest:
		req.Instructions = prependRawJSONString(req.Instructions, policy.InjectPrompt)
	case *dto.OpenAIResponsesCompactionRequest:
		req.Instructions = prependRawJSONString(req.Instructions, policy.InjectPrompt)
	case *dto.ClaudeRequest:
		if req.System == nil {
			req.System = policy.InjectPrompt
			return
		}
		if existing, ok := req.System.(string); ok {
			req.System = strings.TrimSpace(policy.InjectPrompt + "\n" + existing)
		}
	case *dto.GeminiChatRequest:
		injected := dto.GeminiChatContent{
			Parts: []dto.GeminiPart{{
				Text: policy.InjectPrompt,
			}},
		}
		if req.SystemInstructions == nil {
			req.SystemInstructions = &injected
			return
		}
		parts := append([]dto.GeminiPart{{Text: policy.InjectPrompt}}, req.SystemInstructions.Parts...)
		updated := *req.SystemInstructions
		updated.Parts = parts
		req.SystemInstructions = &updated
	}
}

func applyReplaceRules(policy dto.UserRequestInterception, request dto.Request) {
	if len(policy.ReplaceRules) == 0 {
		return
	}
	switch req := request.(type) {
	case *dto.GeneralOpenAIRequest:
		req.Prompt = applyReplaceToValue(req.Prompt, policy.ReplaceRules)
		req.Input = applyReplaceToValue(req.Input, policy.ReplaceRules)
		req.Prefix = applyReplaceToValue(req.Prefix, policy.ReplaceRules)
		req.Suffix = applyReplaceToValue(req.Suffix, policy.ReplaceRules)
		req.Instruction = applyStringReplace(req.Instruction, policy.ReplaceRules)
		for i := range req.Messages {
			applyReplaceToOpenAIMessage(&req.Messages[i], policy.ReplaceRules)
		}
	case *dto.OpenAIResponsesRequest:
		req.Instructions = replaceRawJSONString(req.Instructions, policy.ReplaceRules)
		req.Input = replaceResponsesInput(req.Input, policy.ReplaceRules)
	case *dto.OpenAIResponsesCompactionRequest:
		req.Instructions = replaceRawJSONString(req.Instructions, policy.ReplaceRules)
		req.Input = replaceRawJSONString(req.Input, policy.ReplaceRules)
	case *dto.ClaudeRequest:
		if system, ok := req.System.(string); ok {
			req.System = applyStringReplace(system, policy.ReplaceRules)
		}
		for i := range req.Messages {
			applyReplaceToClaudeMessage(&req.Messages[i], policy.ReplaceRules)
		}
	case *dto.GeminiChatRequest:
		if req.SystemInstructions != nil {
			applyReplaceToGeminiContent(req.SystemInstructions, policy.ReplaceRules)
		}
		for i := range req.Contents {
			applyReplaceToGeminiContent(&req.Contents[i], policy.ReplaceRules)
		}
	}
}

func applyStringReplace(text string, rules []dto.UserInterceptionReplaceRule) string {
	updated := text
	for _, rule := range rules {
		updated = strings.ReplaceAll(updated, rule.From, rule.To)
	}
	return updated
}

func prependText(prefix, text string) string {
	prefix = strings.TrimSpace(prefix)
	text = strings.TrimSpace(text)
	if prefix == "" {
		return text
	}
	if text == "" {
		return prefix
	}
	return prefix + "\n" + text
}

func prependTextToValue(value any, prefix string) any {
	switch v := value.(type) {
	case string:
		return prependText(prefix, v)
	case []string:
		return append([]string{prefix}, v...)
	case []any:
		return append([]any{prefix}, v...)
	default:
		return value
	}
}

func applyReplaceToValue(value any, rules []dto.UserInterceptionReplaceRule) any {
	switch v := value.(type) {
	case string:
		return applyStringReplace(v, rules)
	case []string:
		updated := make([]string, len(v))
		for i := range v {
			updated[i] = applyStringReplace(v[i], rules)
		}
		return updated
	case []any:
		updated := make([]any, len(v))
		for i, item := range v {
			if text, ok := item.(string); ok {
				updated[i] = applyStringReplace(text, rules)
				continue
			}
			updated[i] = item
		}
		return updated
	default:
		return value
	}
}

func prependRawJSONString(raw []byte, prefix string) []byte {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return raw
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		out, err := common.Marshal(prefix)
		if err != nil {
			return raw
		}
		return out
	}
	if common.GetJsonType(raw) != "string" {
		return raw
	}
	var text string
	if err := common.Unmarshal(raw, &text); err != nil {
		return raw
	}
	out, err := common.Marshal(prependText(prefix, text))
	if err != nil {
		return raw
	}
	return out
}

func replaceRawJSONString(raw []byte, rules []dto.UserInterceptionReplaceRule) []byte {
	if len(bytes.TrimSpace(raw)) == 0 || common.GetJsonType(raw) != "string" {
		return raw
	}
	var text string
	if err := common.Unmarshal(raw, &text); err != nil {
		return raw
	}
	out, err := common.Marshal(applyStringReplace(text, rules))
	if err != nil {
		return raw
	}
	return out
}

func replaceResponsesInput(raw []byte, rules []dto.UserInterceptionReplaceRule) []byte {
	if len(bytes.TrimSpace(raw)) == 0 {
		return raw
	}
	switch common.GetJsonType(raw) {
	case "string":
		return replaceRawJSONString(raw, rules)
	case "array":
		var inputs []dto.Input
		if err := common.Unmarshal(raw, &inputs); err != nil {
			return raw
		}
		modified := false
		for i := range inputs {
			updatedContent, contentChanged := replaceResponsesInputContent(inputs[i].Content, rules)
			if contentChanged {
				inputs[i].Content = updatedContent
				modified = true
			}
		}
		if !modified {
			return raw
		}
		out, err := common.Marshal(inputs)
		if err != nil {
			return raw
		}
		return out
	default:
		return raw
	}
}

func replaceResponsesInputContent(raw []byte, rules []dto.UserInterceptionReplaceRule) ([]byte, bool) {
	if len(bytes.TrimSpace(raw)) == 0 {
		return raw, false
	}
	switch common.GetJsonType(raw) {
	case "string":
		updated := replaceRawJSONString(raw, rules)
		return updated, !bytes.Equal(updated, raw)
	case "array":
		var items []map[string]any
		if err := common.Unmarshal(raw, &items); err != nil {
			return raw, false
		}
		modified := false
		for i := range items {
			typeVal, _ := items[i]["type"].(string)
			if typeVal != "input_text" {
				continue
			}
			text, ok := items[i]["text"].(string)
			if !ok {
				continue
			}
			replaced := applyStringReplace(text, rules)
			if replaced == text {
				continue
			}
			items[i]["text"] = replaced
			modified = true
		}
		if !modified {
			return raw, false
		}
		out, err := common.Marshal(items)
		if err != nil {
			return raw, false
		}
		return out, true
	default:
		return raw, false
	}
}

func applyReplaceToOpenAIMessage(message *dto.Message, rules []dto.UserInterceptionReplaceRule) {
	if message == nil || message.Content == nil {
		return
	}
	if message.IsStringContent() {
		message.SetStringContent(applyStringReplace(message.StringContent(), rules))
		return
	}
	parts := message.ParseContent()
	modified := false
	for i := range parts {
		if parts[i].Type == dto.ContentTypeText {
			parts[i].Text = applyStringReplace(parts[i].Text, rules)
			modified = true
		}
	}
	if modified {
		message.SetMediaContent(parts)
	}
}

func applyReplaceToClaudeMessage(message *dto.ClaudeMessage, rules []dto.UserInterceptionReplaceRule) {
	if message == nil || message.Content == nil {
		return
	}
	if message.IsStringContent() {
		message.SetStringContent(applyStringReplace(message.GetStringContent(), rules))
		return
	}
	content, err := message.ParseContent()
	if err != nil {
		return
	}
	modified := false
	for i := range content {
		if content[i].Type == dto.ContentTypeText {
			content[i].SetText(applyStringReplace(content[i].GetText(), rules))
			modified = true
		}
	}
	if modified {
		message.SetContent(content)
	}
}

func applyReplaceToGeminiContent(content *dto.GeminiChatContent, rules []dto.UserInterceptionReplaceRule) {
	if content == nil {
		return
	}
	for i := range content.Parts {
		content.Parts[i].Text = applyStringReplace(content.Parts[i].Text, rules)
	}
}

func recordUserRequestInterceptionLog(c *gin.Context, request dto.Request, policy dto.UserRequestInterception, requestText string, matchedKeywords []string) {
	if c == nil {
		return
	}
	userId := c.GetInt("id")
	if userId == 0 {
		return
	}

	model.RecordRequestInterceptionLog(c, userId, model.RecordRequestInterceptionLogParams{
		ModelName:       extractRequestModel(request),
		TokenName:       c.GetString("token_name"),
		TokenId:         c.GetInt("token_id"),
		Group:           c.GetString("group"),
		Mode:            policy.Mode,
		Action:          policy.Mode,
		MatchedKeywords: matchedKeywords,
		RequestText:     requestText,
		RequestPath: func() string {
			if c.Request != nil && c.Request.URL != nil {
				return c.Request.URL.Path
			}
			return ""
		}(),
	})
}

func extractRequestModel(request dto.Request) string {
	switch req := request.(type) {
	case *dto.GeneralOpenAIRequest:
		return req.Model
	case *dto.OpenAIResponsesRequest:
		return req.Model
	case *dto.OpenAIResponsesCompactionRequest:
		return req.Model
	case *dto.ClaudeRequest:
		return req.Model
	default:
		return ""
	}
}
