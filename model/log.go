package model

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"

	"github.com/bytedance/gopkg/util/gopool"
	"gorm.io/gorm"
)

type Log struct {
	Id               int    `json:"id" gorm:"index:idx_created_at_id,priority:1;index:idx_user_id_id,priority:2"`
	UserId           int    `json:"user_id" gorm:"index;index:idx_user_id_id,priority:1"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint;index:idx_created_at_id,priority:2;index:idx_created_at_type"`
	Type             int    `json:"type" gorm:"index:idx_created_at_type"`
	Content          string `json:"content"`
	Username         string `json:"username" gorm:"index;index:index_username_model_name,priority:2;default:''"`
	TokenName        string `json:"token_name" gorm:"index;default:''"`
	ModelName        string `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota            int    `json:"quota" gorm:"default:0"`
	PromptTokens     int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens int    `json:"completion_tokens" gorm:"default:0"`
	UseTime          int    `json:"use_time" gorm:"default:0"`
	IsStream         bool   `json:"is_stream"`
	ChannelId        int    `json:"channel" gorm:"index"`
	ChannelName      string `json:"channel_name" gorm:"->"`
	TokenId          int    `json:"token_id" gorm:"default:0;index"`
	Group            string `json:"group" gorm:"index"`
	Ip               string `json:"ip" gorm:"index;default:''"`
	RequestId        string `json:"request_id,omitempty" gorm:"type:varchar(64);index:idx_logs_request_id;default:''"`
	Other            string `json:"other"`
}

// don't use iota, avoid change log type value
const (
	LogTypeUnknown = 0
	LogTypeTopup   = 1
	LogTypeConsume = 2
	LogTypeManage  = 3
	LogTypeSystem  = 4
	LogTypeError   = 5
	LogTypeRefund  = 6
)

const requestInterceptionLogLikePattern = `%"intercept_log":true%`
const requestAuditLogLikePattern = `%"request_audit_log":true%`

func formatUserLogs(logs []*Log, startIdx int) {
	for i := range logs {
		logs[i].ChannelName = ""
		var otherMap map[string]interface{}
		otherMap, _ = common.StrToMap(logs[i].Other)
		if otherMap != nil {
			// Remove admin-only debug fields.
			delete(otherMap, "admin_info")
			delete(otherMap, "reject_reason")
		}
		logs[i].Other = common.MapToJsonStr(otherMap)
		logs[i].Id = startIdx + i + 1
	}
}

func GetLogByTokenId(tokenId int) (logs []*Log, err error) {
	err = LOG_DB.Model(&Log{}).Where("token_id = ?", tokenId).Order("id desc").Limit(common.MaxRecentItems).Find(&logs).Error
	formatUserLogs(logs, 0)
	return logs, err
}

func RecordLog(userId int, logType int, content string) {
	if logType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(userId, false)
	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record log: " + err.Error())
	}
}

func RecordErrorLog(c *gin.Context, userId int, channelId int, modelName string, tokenName string, content string, tokenId int, useTimeSeconds int,
	isStream bool, group string, other map[string]interface{}) {
	logger.LogInfo(c, fmt.Sprintf("record error log: userId=%d, channelId=%d, modelName=%s, tokenName=%s, content=%s", userId, channelId, modelName, tokenName, content))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	otherStr := common.MapToJsonStr(other)
	// 判断是否需要记录 IP
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeError,
		Content:          content,
		PromptTokens:     0,
		CompletionTokens: 0,
		TokenName:        tokenName,
		ModelName:        modelName,
		Quota:            0,
		ChannelId:        channelId,
		TokenId:          tokenId,
		UseTime:          useTimeSeconds,
		IsStream:         isStream,
		Group:            group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId: requestId,
		Other:     otherStr,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
}

type RecordConsumeLogParams struct {
	ChannelId        int                    `json:"channel_id"`
	PromptTokens     int                    `json:"prompt_tokens"`
	CompletionTokens int                    `json:"completion_tokens"`
	ModelName        string                 `json:"model_name"`
	TokenName        string                 `json:"token_name"`
	Quota            int                    `json:"quota"`
	Content          string                 `json:"content"`
	TokenId          int                    `json:"token_id"`
	UseTimeSeconds   int                    `json:"use_time_seconds"`
	IsStream         bool                   `json:"is_stream"`
	Group            string                 `json:"group"`
	Other            map[string]interface{} `json:"other"`
}

func RecordConsumeLog(c *gin.Context, userId int, params RecordConsumeLogParams) {
	if !common.LogConsumeEnabled {
		return
	}
	logger.LogInfo(c, fmt.Sprintf("record consume log: userId=%d, params=%s", userId, common.GetJsonString(params)))
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	otherStr := common.MapToJsonStr(params.Other)
	// 判断是否需要记录 IP
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}
	log := &Log{
		UserId:           userId,
		Username:         username,
		CreatedAt:        common.GetTimestamp(),
		Type:             LogTypeConsume,
		Content:          params.Content,
		PromptTokens:     params.PromptTokens,
		CompletionTokens: params.CompletionTokens,
		TokenName:        params.TokenName,
		ModelName:        params.ModelName,
		Quota:            params.Quota,
		ChannelId:        params.ChannelId,
		TokenId:          params.TokenId,
		UseTime:          params.UseTimeSeconds,
		IsStream:         params.IsStream,
		Group:            params.Group,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		RequestId: requestId,
		Other:     otherStr,
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		logger.LogError(c, "failed to record log: "+err.Error())
	}
	if common.DataExportEnabled {
		gopool.Go(func() {
			LogQuotaData(userId, username, params.ModelName, params.Quota, common.GetTimestamp(), params.PromptTokens+params.CompletionTokens)
		})
	}
}

type RecordTaskBillingLogParams struct {
	UserId    int
	LogType   int
	Content   string
	ChannelId int
	ModelName string
	Quota     int
	TokenId   int
	Group     string
	Other     map[string]interface{}
}

func RecordTaskBillingLog(params RecordTaskBillingLogParams) {
	if params.LogType == LogTypeConsume && !common.LogConsumeEnabled {
		return
	}
	username, _ := GetUsernameById(params.UserId, false)
	tokenName := ""
	if params.TokenId > 0 {
		if token, err := GetTokenById(params.TokenId); err == nil {
			tokenName = token.Name
		}
	}
	log := &Log{
		UserId:    params.UserId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      params.LogType,
		Content:   params.Content,
		TokenName: tokenName,
		ModelName: params.ModelName,
		Quota:     params.Quota,
		ChannelId: params.ChannelId,
		TokenId:   params.TokenId,
		Group:     params.Group,
		Other:     common.MapToJsonStr(params.Other),
	}
	err := LOG_DB.Create(log).Error
	if err != nil {
		common.SysLog("failed to record task billing log: " + err.Error())
	}
}

type RecordRequestInterceptionLogParams struct {
	ModelName           string
	TokenName           string
	TokenId             int
	Group               string
	Mode                string
	Action              string
	MatchedKeywords     []string
	OriginalRequestText string
	FinalRequestText    string
	ResponseText        string
	RequestText         string
	RequestPath         string
}

type RecordRequestAuditLogParams struct {
	ModelName           string
	TokenName           string
	TokenId             int
	Group               string
	Mode                string
	Status              string
	MatchedKeywords     []string
	OriginalRequestText string
	FinalRequestText    string
	ResponseText        string
	RequestPath         string
	StatusCode          int
}

func RecordRequestInterceptionLog(c *gin.Context, userId int, params RecordRequestInterceptionLogParams) {
	RecordRequestAuditLogWithExtra(c, userId, RecordRequestAuditLogParams{
		ModelName:           params.ModelName,
		TokenName:           params.TokenName,
		TokenId:             params.TokenId,
		Group:               params.Group,
		Mode:                params.Mode,
		Status:              "blocked",
		MatchedKeywords:     params.MatchedKeywords,
		OriginalRequestText: params.OriginalRequestText,
		FinalRequestText:    firstNonEmpty(params.FinalRequestText, params.RequestText),
		ResponseText:        params.ResponseText,
		RequestPath:         params.RequestPath,
		StatusCode:          http.StatusForbidden,
	}, map[string]interface{}{
		"intercept_log":    true,
		"intercept_mode":   params.Mode,
		"intercept_action": params.Action,
	})
	return
	if LOG_DB == nil {
		return
	}
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}

	requestText := strings.TrimSpace(params.RequestText)
	requestTextRunes := []rune(requestText)
	if len(requestTextRunes) > 8000 {
		requestText = string(requestTextRunes[:8000]) + "\n...[truncated]"
	}
	requestPreview := strings.ReplaceAll(requestText, "\n", " ")
	requestPreviewRunes := []rune(requestPreview)
	if len(requestPreviewRunes) > 120 {
		requestPreview = string(requestPreviewRunes[:120]) + "..."
	}

	content := fmt.Sprintf("请求拦截（%s）", params.Mode)
	if requestPreview != "" {
		content = fmt.Sprintf("%s：%s", content, requestPreview)
	}

	other := map[string]interface{}{
		"intercept_log":     true,
		"intercept_mode":    params.Mode,
		"intercept_action":  params.Action,
		"matched_keywords":  params.MatchedKeywords,
		"request_text":      requestText,
		"request_path":      params.RequestPath,
		"request_text_size": len([]rune(params.RequestText)),
	}

	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeSystem,
		Content:   content,
		TokenName: params.TokenName,
		ModelName: params.ModelName,
		TokenId:   params.TokenId,
		Group:     params.Group,
		RequestId: requestId,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		Other: common.MapToJsonStr(other),
	}

	if err := LOG_DB.Create(log).Error; err != nil {
		logger.LogError(c, "failed to record request interception log: "+err.Error())
	}
}

func truncateLogText(text string, limit int) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	textRunes := []rune(text)
	if len(textRunes) > limit {
		return string(textRunes[:limit]) + "\n...[truncated]"
	}
	return text
}

func buildLogPreview(text string) string {
	text = strings.ReplaceAll(strings.TrimSpace(text), "\n", " ")
	if text == "" {
		return ""
	}
	textRunes := []rune(text)
	if len(textRunes) > 120 {
		return string(textRunes[:120]) + "..."
	}
	return text
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func RecordRequestAuditLog(c *gin.Context, userId int, params RecordRequestAuditLogParams) {
	RecordRequestAuditLogWithExtra(c, userId, params, nil)
}

func RecordRequestAuditLogWithExtra(c *gin.Context, userId int, params RecordRequestAuditLogParams, extra map[string]interface{}) {
	if LOG_DB == nil {
		return
	}
	username := c.GetString("username")
	requestId := c.GetString(common.RequestIdKey)
	needRecordIp := false
	if settingMap, err := GetUserSetting(userId, false); err == nil {
		if settingMap.RecordIpLog {
			needRecordIp = true
		}
	}

	requestText := truncateLogText(params.FinalRequestText, 20000)
	originalRequestText := truncateLogText(params.OriginalRequestText, 20000)
	responseText := truncateLogText(params.ResponseText, 20000)
	requestPreview := buildLogPreview(firstNonEmpty(requestText, originalRequestText))

	content := fmt.Sprintf("请求明细（%s）", firstNonEmpty(params.Mode, "normal"))
	if requestPreview != "" {
		content = fmt.Sprintf("%s：%s", content, requestPreview)
	}

	other := map[string]interface{}{
		"request_audit_log":     true,
		"request_audit_mode":    firstNonEmpty(params.Mode, "normal"),
		"request_audit_status":  firstNonEmpty(params.Status, "completed"),
		"matched_keywords":      params.MatchedKeywords,
		"original_request_text": originalRequestText,
		"request_text":          requestText,
		"response_text":         responseText,
		"request_path":          params.RequestPath,
		"request_text_size":     len([]rune(strings.TrimSpace(params.FinalRequestText))),
		"response_text_size":    len([]rune(strings.TrimSpace(params.ResponseText))),
		"status_code":           params.StatusCode,
	}
	for key, value := range extra {
		other[key] = value
	}

	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: common.GetTimestamp(),
		Type:      LogTypeSystem,
		Content:   content,
		TokenName: params.TokenName,
		ModelName: params.ModelName,
		TokenId:   params.TokenId,
		Group:     params.Group,
		RequestId: requestId,
		Ip: func() string {
			if needRecordIp {
				return c.ClientIP()
			}
			return ""
		}(),
		Other: common.MapToJsonStr(other),
	}

	if err := LOG_DB.Create(log).Error; err != nil {
		logger.LogError(c, "failed to record request audit log: "+err.Error())
	}
}

func applyRequestInterceptionLogFilter(tx *gorm.DB, interceptOnly bool) *gorm.DB {
	if interceptOnly {
		return tx.Where("(logs.other LIKE ? OR logs.other LIKE ?)", requestAuditLogLikePattern, requestInterceptionLogLikePattern)
	}
	return tx.Where("(logs.other NOT LIKE ? OR logs.other = '' OR logs.other IS NULL)", requestAuditLogLikePattern).
		Where("(logs.other NOT LIKE ? OR logs.other = '' OR logs.other IS NULL)", requestInterceptionLogLikePattern)
}

func applyRequestInterceptionModeFilter(tx *gorm.DB, interceptMode string) *gorm.DB {
	interceptMode = strings.ToLower(strings.TrimSpace(interceptMode))
	if interceptMode == "" {
		return tx
	}
	return tx.Where("(logs.other LIKE ? OR logs.other LIKE ?)", fmt.Sprintf(`%%"request_audit_mode":"%s"%%`, interceptMode), fmt.Sprintf(`%%"intercept_mode":"%s"%%`, interceptMode))
}

func applyRequestInterceptionKeywordFilter(tx *gorm.DB, keyword string) (*gorm.DB, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return tx, nil
	}
	keywordPattern, err := sanitizeLikePattern(keyword)
	if err != nil {
		return nil, err
	}
	tx = tx.Where("(logs.other LIKE ? ESCAPE '!' OR logs.content LIKE ? ESCAPE '!')", keywordPattern, keywordPattern)
	return tx, nil
}

func applyLogCommonFilters(tx *gorm.DB, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, group string, requestId string) (*gorm.DB, error) {
	if modelName != "" {
		modelNamePattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return nil, err
		}
		tx = tx.Where("logs.model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if username != "" {
		tx = tx.Where("logs.username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("logs.token_name = ?", tokenName)
	}
	if requestId != "" {
		tx = tx.Where("logs.request_id = ?", requestId)
	}
	if startTimestamp != 0 {
		tx = tx.Where("logs.created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("logs.created_at <= ?", endTimestamp)
	}
	if channel != 0 {
		tx = tx.Where("logs.channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where("logs."+logGroupCol+" = ?", group)
	}
	return tx, nil
}

func GetAllLogs(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, startIdx int, num int, channel int, group string, requestId string, interceptOnly bool, interceptMode string, interceptKeyword string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB
	} else {
		tx = LOG_DB.Where("logs.type = ?", logType)
	}
	tx = applyRequestInterceptionLogFilter(tx, interceptOnly)
	tx = applyRequestInterceptionModeFilter(tx, interceptMode)
	tx, err = applyRequestInterceptionKeywordFilter(tx, interceptKeyword)
	if err != nil {
		return nil, 0, err
	}
	tx, err = applyLogCommonFilters(tx, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group, requestId)
	if err != nil {
		return nil, 0, err
	}
	err = tx.Model(&Log{}).Count(&total).Error
	if err != nil {
		return nil, 0, err
	}
	err = tx.Order("logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		return nil, 0, err
	}

	channelIds := types.NewSet[int]()
	for _, log := range logs {
		if log.ChannelId != 0 {
			channelIds.Add(log.ChannelId)
		}
	}

	if channelIds.Len() > 0 {
		var channels []struct {
			Id   int    `gorm:"column:id"`
			Name string `gorm:"column:name"`
		}
		if common.MemoryCacheEnabled {
			// Cache get channel
			for _, channelId := range channelIds.Items() {
				if cacheChannel, err := CacheGetChannel(channelId); err == nil {
					channels = append(channels, struct {
						Id   int    `gorm:"column:id"`
						Name string `gorm:"column:name"`
					}{
						Id:   channelId,
						Name: cacheChannel.Name,
					})
				}
			}
		} else {
			// Bulk query channels from DB
			if err = DB.Table("channels").Select("id, name").Where("id IN ?", channelIds.Items()).Find(&channels).Error; err != nil {
				return logs, total, err
			}
		}
		channelMap := make(map[int]string, len(channels))
		for _, channel := range channels {
			channelMap[channel.Id] = channel.Name
		}
		for i := range logs {
			logs[i].ChannelName = channelMap[logs[i].ChannelId]
		}
	}

	return logs, total, err
}

const logSearchCountLimit = 10000

func GetUserLogs(userId int, logType int, startTimestamp int64, endTimestamp int64, modelName string, tokenName string, startIdx int, num int, group string, requestId string, interceptOnly bool, interceptMode string, interceptKeyword string) (logs []*Log, total int64, err error) {
	var tx *gorm.DB
	if logType == LogTypeUnknown {
		tx = LOG_DB.Where("logs.user_id = ?", userId)
	} else {
		tx = LOG_DB.Where("logs.user_id = ? and logs.type = ?", userId, logType)
	}
	tx = applyRequestInterceptionLogFilter(tx, interceptOnly)
	tx = applyRequestInterceptionModeFilter(tx, interceptMode)
	tx, err = applyRequestInterceptionKeywordFilter(tx, interceptKeyword)
	if err != nil {
		return nil, 0, err
	}
	tx, err = applyLogCommonFilters(tx, startTimestamp, endTimestamp, modelName, "", tokenName, 0, group, requestId)
	if err != nil {
		return nil, 0, err
	}
	err = tx.Model(&Log{}).Limit(logSearchCountLimit).Count(&total).Error
	if err != nil {
		common.SysError("failed to count user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}
	err = tx.Order("logs.id desc").Limit(num).Offset(startIdx).Find(&logs).Error
	if err != nil {
		common.SysError("failed to search user logs: " + err.Error())
		return nil, 0, errors.New("查询日志失败")
	}

	formatUserLogs(logs, startIdx)
	return logs, total, err
}

type Stat struct {
	Quota int `json:"quota"`
	Rpm   int `json:"rpm"`
	Tpm   int `json:"tpm"`
}

type UserUsageStat struct {
	UserId       int    `json:"user_id"`
	Username     string `json:"username"`
	Quota        int64  `json:"quota"`
	Tokens       int64  `json:"tokens"`
	RequestCount int64  `json:"request_count"`
}

type RequestInterceptionStat struct {
	Total   int64 `json:"total"`
	Normal  int64 `json:"normal"`
	Ignore  int64 `json:"ignore"`
	Inject  int64 `json:"inject"`
	Replace int64 `json:"replace"`
}

func GetRequestInterceptionStat(startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, group string, requestId string, interceptKeyword string) (stat RequestInterceptionStat, err error) {
	baseQuery := LOG_DB.Table("logs")
	baseQuery = applyRequestInterceptionLogFilter(baseQuery, true)
	baseQuery, err = applyRequestInterceptionKeywordFilter(baseQuery, interceptKeyword)
	if err != nil {
		return stat, err
	}
	baseQuery, err = applyLogCommonFilters(baseQuery, startTimestamp, endTimestamp, modelName, username, tokenName, channel, group, requestId)
	if err != nil {
		return stat, err
	}

	if err = baseQuery.Count(&stat.Total).Error; err != nil {
		return stat, err
	}

	countByMode := func(mode string) (int64, error) {
		var count int64
		query := applyRequestInterceptionModeFilter(baseQuery.Session(&gorm.Session{}), mode)
		err := query.Count(&count).Error
		return count, err
	}

	if stat.Normal, err = countByMode("normal"); err != nil {
		return stat, err
	}
	if stat.Ignore, err = countByMode("ignore"); err != nil {
		return stat, err
	}
	if stat.Inject, err = countByMode("inject"); err != nil {
		return stat, err
	}
	if stat.Replace, err = countByMode("replace"); err != nil {
		return stat, err
	}
	return stat, nil
}

func GetUserUsageRanking(startTimestamp int64, endTimestamp int64, modelName string, tokenName string, channel int, group string, username string, sortBy string, sortOrder string, startIdx int, num int) (items []*UserUsageStat, total int64, err error) {
	tx := LOG_DB.Table("logs").Where("type = ?", LogTypeConsume)

	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		modelNamePattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return nil, 0, err
		}
		tx = tx.Where("model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if channel != 0 {
		tx = tx.Where("channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where(logGroupCol+" = ?", group)
	}

	if err = tx.Distinct("user_id").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	orderBy := "quota"
	switch sortBy {
	case "tokens":
		orderBy = "tokens"
	case "request_count":
		orderBy = "request_count"
	}
	if sortOrder != "asc" {
		sortOrder = "desc"
	}

	err = tx.Select("user_id, username, SUM(quota) AS quota, SUM(prompt_tokens + completion_tokens) AS tokens, COUNT(*) AS request_count").
		Group("user_id, username").
		Order(orderBy + " " + sortOrder).
		Limit(num).
		Offset(startIdx).
		Find(&items).Error
	if err != nil {
		return nil, 0, err
	}

	return items, total, nil
}

func SumUsedQuota(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string, channel int, group string) (stat Stat, err error) {
	tx := LOG_DB.Table("logs").Select("sum(quota) quota")

	// 为rpm和tpm创建单独的查询
	rpmTpmQuery := LOG_DB.Table("logs").Select("count(*) rpm, sum(prompt_tokens) + sum(completion_tokens) tpm")

	if username != "" {
		tx = tx.Where("username = ?", username)
		rpmTpmQuery = rpmTpmQuery.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
		rpmTpmQuery = rpmTpmQuery.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		modelNamePattern, err := sanitizeLikePattern(modelName)
		if err != nil {
			return stat, err
		}
		tx = tx.Where("model_name LIKE ? ESCAPE '!'", modelNamePattern)
		rpmTpmQuery = rpmTpmQuery.Where("model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if channel != 0 {
		tx = tx.Where("channel_id = ?", channel)
		rpmTpmQuery = rpmTpmQuery.Where("channel_id = ?", channel)
	}
	if group != "" {
		tx = tx.Where(logGroupCol+" = ?", group)
		rpmTpmQuery = rpmTpmQuery.Where(logGroupCol+" = ?", group)
	}

	tx = tx.Where("type = ?", LogTypeConsume)
	rpmTpmQuery = rpmTpmQuery.Where("type = ?", LogTypeConsume)

	// 只统计最近60秒的rpm和tpm
	rpmTpmQuery = rpmTpmQuery.Where("created_at >= ?", time.Now().Add(-60*time.Second).Unix())

	// 执行查询
	if err := tx.Scan(&stat).Error; err != nil {
		common.SysError("failed to query log stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}
	if err := rpmTpmQuery.Scan(&stat).Error; err != nil {
		common.SysError("failed to query rpm/tpm stat: " + err.Error())
		return stat, errors.New("查询统计数据失败")
	}

	return stat, nil
}

func SumUsedToken(logType int, startTimestamp int64, endTimestamp int64, modelName string, username string, tokenName string) (token int) {
	tx := LOG_DB.Table("logs").Select("ifnull(sum(prompt_tokens),0) + ifnull(sum(completion_tokens),0)")
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if tokenName != "" {
		tx = tx.Where("token_name = ?", tokenName)
	}
	if startTimestamp != 0 {
		tx = tx.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		tx = tx.Where("created_at <= ?", endTimestamp)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	tx.Where("type = ?", LogTypeConsume).Scan(&token)
	return token
}

func DeleteOldLog(ctx context.Context, targetTimestamp int64, limit int) (int64, error) {
	var total int64 = 0

	for {
		if nil != ctx.Err() {
			return total, ctx.Err()
		}

		result := LOG_DB.Where("created_at < ?", targetTimestamp).Limit(limit).Delete(&Log{})
		if nil != result.Error {
			return total, result.Error
		}

		total += result.RowsAffected

		if result.RowsAffected < int64(limit) {
			break
		}
	}

	return total, nil
}
