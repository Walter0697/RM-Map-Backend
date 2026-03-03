package service

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mapmarker/backend/database/dbmodel"
)

func TestAdminExternalAPIUsageHandlersRequireAdmin(t *testing.T) {
	apiUsageRequireAdminFn = func(w http.ResponseWriter, r *http.Request) *dbmodel.User {
		http.Error(w, "permission denied", http.StatusUnauthorized)
		return nil
	}
	t.Cleanup(func() {
		apiUsageRequireAdminFn = requireAdmin
	})

	handlers := []http.HandlerFunc{
		AdminExternalAPIUsageProvidersHandler,
		AdminExternalAPIUsageSummaryHandler,
		AdminExternalAPIUsageTrendsHandler,
	}
	for _, handler := range handlers {
		req := httptest.NewRequest(http.MethodGet, "/admin/api-usage/test", nil)
		rec := httptest.NewRecorder()
		handler(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected unauthorized for %T, got %d", handler, rec.Code)
		}
	}
}

func TestAdminExternalAPIUsageSummaryValidationAndSuccess(t *testing.T) {
	apiUsageRequireAdminFn = func(w http.ResponseWriter, r *http.Request) *dbmodel.User {
		return &dbmodel.User{Role: "admin"}
	}
	listExternalAPIUsageSummaryFn = func(filter ExternalAPIUsageFilter) (*ExternalAPIUsageSummaryResult, error) {
		return &ExternalAPIUsageSummaryResult{
			Range: ExternalAPIUsageRange{
				From: filter.From,
				To:   filter.To,
			},
			Provider: filter.Provider,
			Overall: ExternalAPIUsageSummaryRow{
				Provider:      "all",
				ProviderLabel: "All Providers",
				TotalCalls:    4,
				SuccessCount:  3,
				ErrorCount:    1,
				AvgLatencyMS:  123.4,
			},
			Providers: []ExternalAPIUsageSummaryRow{},
		}, nil
	}
	t.Cleanup(func() {
		apiUsageRequireAdminFn = requireAdmin
		listExternalAPIUsageSummaryFn = ListExternalAPIUsageSummary
	})

	badReq := httptest.NewRequest(http.MethodGet, "/admin/api-usage/summary?from=bad", nil)
	badRec := httptest.NewRecorder()
	AdminExternalAPIUsageSummaryHandler(badRec, badReq)
	if badRec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for invalid range, got %d", badRec.Code)
	}

	from := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	to := time.Now().UTC().Format(time.RFC3339)
	okReq := httptest.NewRequest(http.MethodGet, "/admin/api-usage/summary?provider="+ExternalAPIProviderTomTomMap+"&from="+from+"&to="+to, nil)
	okRec := httptest.NewRecorder()
	AdminExternalAPIUsageSummaryHandler(okRec, okReq)
	if okRec.Code != http.StatusOK {
		t.Fatalf("expected summary status 200, got %d", okRec.Code)
	}
}

func TestAdminExternalAPIUsageTrendsValidation(t *testing.T) {
	apiUsageRequireAdminFn = func(w http.ResponseWriter, r *http.Request) *dbmodel.User {
		return &dbmodel.User{Role: "admin"}
	}
	listExternalAPIUsageTrendsFn = func(filter ExternalAPIUsageFilter, interval string) (*ExternalAPIUsageTrendResult, error) {
		if interval == "bad" {
			return nil, parseIntervalErrorForTest()
		}
		return &ExternalAPIUsageTrendResult{
			Range: ExternalAPIUsageRange{
				From: filter.From,
				To:   filter.To,
			},
			Provider: filter.Provider,
			Interval: "1h",
			Points:   []ExternalAPIUsageTrendPoint{},
		}, nil
	}
	t.Cleanup(func() {
		apiUsageRequireAdminFn = requireAdmin
		listExternalAPIUsageTrendsFn = ListExternalAPIUsageTrends
	})

	req := httptest.NewRequest(http.MethodGet, "/admin/api-usage/trends?interval=bad", nil)
	rec := httptest.NewRecorder()
	AdminExternalAPIUsageTrendsHandler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for invalid interval, got %d", rec.Code)
	}
}

func parseIntervalErrorForTest() error {
	_, err := parseTrendInterval("bad")
	return err
}
