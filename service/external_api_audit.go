package service

import (
	"fmt"
	"io"
	"log"
	"mapmarker/backend/config"
	"mapmarker/backend/database"
	"mapmarker/backend/database/dbmodel"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	externalAPIErrorClassUpstreamHTTP = "upstream_http"
	externalAPIErrorClassNetwork      = "network"
	externalAPIErrorClassDecode       = "decode"
	externalAPIErrorClassOther        = "other"

	maxExternalAPIAuditErrorDetail = 512
)

type ExternalAPIAuditWriteInput struct {
	Provider    string
	Operation   string
	StatusClass string
	HTTPStatus  *int
	LatencyMS   int64
	RequestTime time.Time
	ErrorClass  string
	ErrorCode   string
	ErrorDetail string
	CompletedAt *time.Time
}

type ExternalAPIUsageRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

type ExternalAPIUsageSummaryRow struct {
	Provider      string  `json:"provider"`
	ProviderLabel string  `json:"provider_label"`
	TotalCalls    int64   `json:"total_calls"`
	SuccessCount  int64   `json:"success_count"`
	ErrorCount    int64   `json:"error_count"`
	AvgLatencyMS  float64 `json:"avg_latency_ms"`
}

type ExternalAPIUsageSummaryResult struct {
	Range     ExternalAPIUsageRange        `json:"range"`
	Provider  string                       `json:"provider"`
	Overall   ExternalAPIUsageSummaryRow   `json:"overall"`
	Providers []ExternalAPIUsageSummaryRow `json:"providers"`
}

type ExternalAPIUsageTrendPoint struct {
	BucketStart  time.Time `json:"bucket_start"`
	TotalCalls   int64     `json:"total_calls"`
	SuccessCount int64     `json:"success_count"`
	ErrorCount   int64     `json:"error_count"`
	AvgLatencyMS float64   `json:"avg_latency_ms"`
}

type ExternalAPIUsageTrendResult struct {
	Range    ExternalAPIUsageRange        `json:"range"`
	Provider string                       `json:"provider"`
	Interval string                       `json:"interval"`
	Points   []ExternalAPIUsageTrendPoint `json:"points"`
}

type ExternalAPIUsageFilter struct {
	From     time.Time
	To       time.Time
	Provider string
}

var externalAPIHTTPClientFactory = func() *http.Client {
	return &http.Client{Timeout: 10 * time.Second}
}

var createExternalAPIAuditEventFn = CreateExternalAPIAuditEvent
var listExternalAPIUsageSummaryFn = ListExternalAPIUsageSummary
var listExternalAPIUsageTrendsFn = ListExternalAPIUsageTrends
var listManagedExternalAPIProvidersFn = ListManagedExternalAPIProviders

func CreateExternalAPIAuditEvent(input ExternalAPIAuditWriteInput) error {
	provider := strings.TrimSpace(input.Provider)
	operation := strings.TrimSpace(input.Operation)
	if provider == "" || operation == "" {
		return fmt.Errorf("provider and operation are required")
	}

	statusClass := strings.TrimSpace(strings.ToLower(input.StatusClass))
	if statusClass != dbmodel.ExternalAPIAuditStatusSuccess && statusClass != dbmodel.ExternalAPIAuditStatusError {
		statusClass = dbmodel.ExternalAPIAuditStatusError
	}

	requestTime := input.RequestTime
	if requestTime.IsZero() {
		requestTime = time.Now().UTC()
	}

	latency := input.LatencyMS
	if latency < 0 {
		latency = 0
	}

	event := dbmodel.ExternalAPIAuditEvent{
		Provider:    provider,
		Operation:   operation,
		StatusClass: statusClass,
		HTTPStatus:  input.HTTPStatus,
		LatencyMS:   latency,
		RequestTime: requestTime,
		ErrorClass:  strings.TrimSpace(input.ErrorClass),
		ErrorCode:   strings.TrimSpace(input.ErrorCode),
		ErrorDetail: truncateAuditErrorDetail(input.ErrorDetail),
		CompletedAt: input.CompletedAt,
	}
	return event.Create(database.Connection)
}

func truncateAuditErrorDetail(value string) string {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= maxExternalAPIAuditErrorDetail {
		return trimmed
	}
	return trimmed[:maxExternalAPIAuditErrorDetail]
}

func classifyExternalAPIErr(err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "status code"):
		return externalAPIErrorClassUpstreamHTTP
	case strings.Contains(msg, "unmarshal") || strings.Contains(msg, "decode"):
		return externalAPIErrorClassDecode
	case strings.Contains(msg, "dial") || strings.Contains(msg, "timeout") || strings.Contains(msg, "no such host") || strings.Contains(msg, "connection refused"):
		return externalAPIErrorClassNetwork
	default:
		return externalAPIErrorClassOther
	}
}

func writeExternalAPIAuditEvent(input ExternalAPIAuditWriteInput) {
	if err := createExternalAPIAuditEventFn(input); err != nil {
		time.Sleep(50 * time.Millisecond)
		retryErr := createExternalAPIAuditEventFn(input)
		if retryErr == nil {
			log.Printf(
				"external api audit write recovered after retry provider=%s operation=%s status=%s",
				input.Provider,
				input.Operation,
				input.StatusClass,
			)
			return
		}
		log.Printf(
			"external api audit write failed provider=%s operation=%s status=%s http_status=%v error=%v retry_error=%v",
			input.Provider,
			input.Operation,
			input.StatusClass,
			input.HTTPStatus,
			err,
			retryErr,
		)
	}
}

func GetRequestWithExternalAPIAudit(provider string, operation string, url string, validateBody func([]byte) error) ([]byte, error) {
	startedAt := time.Now().UTC()
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		now := time.Now().UTC()
		writeExternalAPIAuditEvent(ExternalAPIAuditWriteInput{
			Provider:    provider,
			Operation:   operation,
			StatusClass: dbmodel.ExternalAPIAuditStatusError,
			LatencyMS:   now.Sub(startedAt).Milliseconds(),
			RequestTime: startedAt,
			ErrorClass:  classifyExternalAPIErr(err),
			ErrorDetail: err.Error(),
			CompletedAt: &now,
		})
		return nil, err
	}

	client := externalAPIHTTPClientFactory()
	resp, err := client.Do(request)
	if err != nil {
		now := time.Now().UTC()
		writeExternalAPIAuditEvent(ExternalAPIAuditWriteInput{
			Provider:    provider,
			Operation:   operation,
			StatusClass: dbmodel.ExternalAPIAuditStatusError,
			LatencyMS:   now.Sub(startedAt).Milliseconds(),
			RequestTime: startedAt,
			ErrorClass:  classifyExternalAPIErr(err),
			ErrorDetail: err.Error(),
			CompletedAt: &now,
		})
		return nil, err
	}
	defer resp.Body.Close()

	body, readErr := io.ReadAll(resp.Body)
	now := time.Now().UTC()
	statusCode := resp.StatusCode
	statusClass := dbmodel.ExternalAPIAuditStatusSuccess
	errClass := ""
	errDetail := ""
	var resultErr error

	if readErr != nil {
		statusClass = dbmodel.ExternalAPIAuditStatusError
		errClass = classifyExternalAPIErr(readErr)
		errDetail = readErr.Error()
		resultErr = readErr
	} else if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		statusClass = dbmodel.ExternalAPIAuditStatusError
		resultErr = fmt.Errorf("status code %d", resp.StatusCode)
		errClass = classifyExternalAPIErr(resultErr)
		errDetail = resultErr.Error()
	} else if validateBody != nil {
		validateErr := validateBody(body)
		if validateErr != nil {
			statusClass = dbmodel.ExternalAPIAuditStatusError
			resultErr = validateErr
			errClass = classifyExternalAPIErr(validateErr)
			errDetail = validateErr.Error()
		}
	}

	writeExternalAPIAuditEvent(ExternalAPIAuditWriteInput{
		Provider:    provider,
		Operation:   operation,
		StatusClass: statusClass,
		HTTPStatus:  &statusCode,
		LatencyMS:   now.Sub(startedAt).Milliseconds(),
		RequestTime: startedAt,
		ErrorClass:  errClass,
		ErrorDetail: errDetail,
		CompletedAt: &now,
	})

	if resultErr != nil {
		return nil, resultErr
	}
	return body, nil
}

type externalAPIUsageSummaryRowDB struct {
	Provider     string
	TotalCalls   int64
	SuccessCount int64
	ErrorCount   int64
	AvgLatencyMS float64
}

func ListExternalAPIUsageSummary(filter ExternalAPIUsageFilter) (*ExternalAPIUsageSummaryResult, error) {
	query := database.Connection.Model(&dbmodel.ExternalAPIAuditEvent{}).Where("request_time >= ? AND request_time <= ?", filter.From, filter.To)
	if strings.TrimSpace(filter.Provider) != "" {
		query = query.Where("provider = ?", strings.TrimSpace(filter.Provider))
	}

	rows := make([]externalAPIUsageSummaryRowDB, 0)
	if err := query.Select(
		"provider",
		"COUNT(*) as total_calls",
		"COUNT(*) FILTER (WHERE status_class = 'success') as success_count",
		"COUNT(*) FILTER (WHERE status_class = 'error') as error_count",
		"COALESCE(AVG(latency_ms), 0) as avg_latency_ms",
	).Group("provider").Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := &ExternalAPIUsageSummaryResult{
		Range: ExternalAPIUsageRange{
			From: filter.From,
			To:   filter.To,
		},
		Provider: strings.TrimSpace(filter.Provider),
		Overall: ExternalAPIUsageSummaryRow{
			Provider:      "all",
			ProviderLabel: "All Providers",
		},
		Providers: make([]ExternalAPIUsageSummaryRow, 0, len(rows)),
	}

	for _, item := range rows {
		row := ExternalAPIUsageSummaryRow{
			Provider:      strings.TrimSpace(item.Provider),
			ProviderLabel: ExternalAPIProviderLabel(item.Provider),
			TotalCalls:    item.TotalCalls,
			SuccessCount:  item.SuccessCount,
			ErrorCount:    item.ErrorCount,
			AvgLatencyMS:  item.AvgLatencyMS,
		}
		result.Providers = append(result.Providers, row)
		result.Overall.TotalCalls += row.TotalCalls
		result.Overall.SuccessCount += row.SuccessCount
		result.Overall.ErrorCount += row.ErrorCount
	}

	if result.Overall.TotalCalls > 0 {
		totalLatency := 0.0
		for _, item := range result.Providers {
			totalLatency += item.AvgLatencyMS * float64(item.TotalCalls)
		}
		result.Overall.AvgLatencyMS = totalLatency / float64(result.Overall.TotalCalls)
	}

	sort.Slice(result.Providers, func(i, j int) bool {
		return result.Providers[i].Provider < result.Providers[j].Provider
	})

	return result, nil
}

type externalAPIAuditEventForTrend struct {
	RequestTime time.Time
	StatusClass string
	LatencyMS   int64
}

func ListExternalAPIUsageTrends(filter ExternalAPIUsageFilter, interval string) (*ExternalAPIUsageTrendResult, error) {
	bucketDuration, err := parseTrendInterval(interval)
	if err != nil {
		return nil, err
	}

	query := database.Connection.Model(&dbmodel.ExternalAPIAuditEvent{}).
		Select("request_time", "status_class", "latency_ms").
		Where("request_time >= ? AND request_time <= ?", filter.From, filter.To).
		Order("request_time asc")
	if strings.TrimSpace(filter.Provider) != "" {
		query = query.Where("provider = ?", strings.TrimSpace(filter.Provider))
	}

	rows := make([]externalAPIAuditEventForTrend, 0)
	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	type bucketAccumulator struct {
		totalCalls   int64
		successCount int64
		errorCount   int64
		latencySum   int64
	}
	acc := map[time.Time]*bucketAccumulator{}

	for _, item := range rows {
		bucketStart := floorTimeToBucket(item.RequestTime.UTC(), bucketDuration)
		slot := acc[bucketStart]
		if slot == nil {
			slot = &bucketAccumulator{}
			acc[bucketStart] = slot
		}
		slot.totalCalls++
		if item.StatusClass == dbmodel.ExternalAPIAuditStatusSuccess {
			slot.successCount++
		}
		if item.StatusClass == dbmodel.ExternalAPIAuditStatusError {
			slot.errorCount++
		}
		slot.latencySum += item.LatencyMS
	}

	keys := make([]time.Time, 0, len(acc))
	for key := range acc {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Before(keys[j])
	})

	points := make([]ExternalAPIUsageTrendPoint, 0, len(keys))
	for _, key := range keys {
		slot := acc[key]
		avgLatency := 0.0
		if slot.totalCalls > 0 {
			avgLatency = float64(slot.latencySum) / float64(slot.totalCalls)
		}
		points = append(points, ExternalAPIUsageTrendPoint{
			BucketStart:  key,
			TotalCalls:   slot.totalCalls,
			SuccessCount: slot.successCount,
			ErrorCount:   slot.errorCount,
			AvgLatencyMS: avgLatency,
		})
	}

	return &ExternalAPIUsageTrendResult{
		Range: ExternalAPIUsageRange{
			From: filter.From,
			To:   filter.To,
		},
		Provider: strings.TrimSpace(filter.Provider),
		Interval: normalizeTrendInterval(interval),
		Points:   points,
	}, nil
}

func normalizeTrendInterval(value string) string {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return "1h"
	}
	return trimmed
}

func parseTrendInterval(value string) (time.Duration, error) {
	switch normalizeTrendInterval(value) {
	case "15m":
		return 15 * time.Minute, nil
	case "1h":
		return time.Hour, nil
	case "6h":
		return 6 * time.Hour, nil
	case "1d":
		return 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("interval must be one of 15m,1h,6h,1d")
	}
}

func floorTimeToBucket(value time.Time, bucket time.Duration) time.Time {
	if bucket <= 0 {
		return value
	}
	epoch := value.Unix()
	size := int64(bucket.Seconds())
	floored := (epoch / size) * size
	return time.Unix(floored, 0).UTC()
}

func CleanupOldExternalAPIAuditData() error {
	now := time.Now().UTC()
	logCutoff := now.AddDate(0, 0, -config.Data.IntegrationAuth.LogRetentionDays)
	if err := database.Connection.Where("request_time < ?", logCutoff).Delete(&dbmodel.ExternalAPIAuditEvent{}).Error; err != nil {
		return err
	}

	if config.Data.IntegrationAuth.MaxAuditLogRows > 0 {
		var count int64
		if err := database.Connection.Model(&dbmodel.ExternalAPIAuditEvent{}).Count(&count).Error; err != nil {
			return err
		}
		if count > int64(config.Data.IntegrationAuth.MaxAuditLogRows) {
			excess := count - int64(config.Data.IntegrationAuth.MaxAuditLogRows)
			ids := make([]uint, 0, excess)
			if err := database.Connection.Model(&dbmodel.ExternalAPIAuditEvent{}).Order("request_time asc").Limit(int(excess)).Pluck("id", &ids).Error; err != nil {
				return err
			}
			if len(ids) > 0 {
				if err := database.Connection.Where("id IN ?", ids).Delete(&dbmodel.ExternalAPIAuditEvent{}).Error; err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func StartExternalAPIAuditCleanupWorker() {
	if !config.Data.IntegrationAuth.EnableCleanup {
		return
	}

	interval := time.Duration(config.Data.IntegrationAuth.CleanupIntervalHours) * time.Hour
	if interval <= 0 {
		interval = 24 * time.Hour
	}

	go func() {
		if err := CleanupOldExternalAPIAuditData(); err != nil {
			fmt.Printf("external api audit cleanup failed at startup: %v\n", err)
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for range ticker.C {
			if err := CleanupOldExternalAPIAuditData(); err != nil {
				fmt.Printf("external api audit cleanup failed: %v\n", err)
			}
		}
	}()
}

func parseExternalAPIUsageFilter(query map[string]string, now time.Time) (ExternalAPIUsageFilter, error) {
	filter := ExternalAPIUsageFilter{}
	to := now.UTC()
	from := to.AddDate(0, 0, -7)

	if raw := strings.TrimSpace(query["from"]); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filter, fmt.Errorf("from must be RFC3339")
		}
		from = parsed.UTC()
	}
	if raw := strings.TrimSpace(query["to"]); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return filter, fmt.Errorf("to must be RFC3339")
		}
		to = parsed.UTC()
	}
	if !from.Before(to) {
		return filter, fmt.Errorf("from must be before to")
	}

	provider := strings.TrimSpace(query["provider"])
	if provider != "" && !IsManagedExternalAPIProvider(provider) {
		return filter, fmt.Errorf("unsupported provider")
	}

	filter.From = from
	filter.To = to
	filter.Provider = provider
	return filter, nil
}
