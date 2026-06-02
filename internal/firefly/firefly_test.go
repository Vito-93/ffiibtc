package firefly

import (
	"encoding/json"
	"ffiiibc/internal/classifier"
	"ffiiibc/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func singlePageServer(t *testing.T, transactions []FireFlyTransaction) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := apiResponse{
			Data: []transactionGroup{
				{Attributes: transactionAttributes{Transactions: transactions}},
			},
			Meta: apiMeta{Pagination: apiPagination{CurrentPage: 1, TotalPages: 1}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestFetchTransactions_ForwardsEndDate(t *testing.T) {
	var capturedEnd string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedEnd = r.URL.Query().Get("end")
		resp := apiResponse{Meta: apiMeta{Pagination: apiPagination{CurrentPage: 1, TotalPages: 1}}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := &config.Config{APIKey: "test-key", FFApp: srv.URL}
	end := time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC)
	_, err := NewClient(cfg, srv.Client()).FetchTransactions(nil, &end)

	require.NoError(t, err)
	assert.Equal(t, "2024-12-31", capturedEnd)
}

func TestFetchTransactions_ForwardsStartDate(t *testing.T) {
	var capturedStart string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedStart = r.URL.Query().Get("start")
		resp := apiResponse{Meta: apiMeta{Pagination: apiPagination{CurrentPage: 1, TotalPages: 1}}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := &config.Config{APIKey: "test-key", FFApp: srv.URL}
	start := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	_, err := NewClient(cfg, srv.Client()).FetchTransactions(&start, nil)

	require.NoError(t, err)
	assert.Equal(t, "2024-01-15", capturedStart)
}

func TestFetchTransactions_FetchesAllPages(t *testing.T) {
	pages := map[int][]FireFlyTransaction{
		1: {{Description: "SUPERMERCATO COOP", CategoryName: "Food", BudgetName: "Needs"}},
		2: {{Description: "NETFLIX", CategoryName: "Entertainment", BudgetName: "Fun"}},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := 1
		if r.URL.Query().Get("page") == "2" {
			page = 2
		}
		resp := apiResponse{
			Data: []transactionGroup{
				{Attributes: transactionAttributes{Transactions: pages[page]}},
			},
			Meta: apiMeta{Pagination: apiPagination{CurrentPage: page, TotalPages: 2}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cfg := &config.Config{APIKey: "test-key", FFApp: srv.URL}
	got, err := NewClient(cfg, srv.Client()).FetchTransactions(nil, nil)

	require.NoError(t, err)
	assert.Equal(t, classifier.TransactionDataSet{
		{"Needs", "SUPERMERCATO COOP", "Food"},
		{"Fun", "NETFLIX", "Entertainment"},
	}, got)
}

func TestFetchTransactions_ExcludesTransactionsWithoutBudget(t *testing.T) {
	srv := singlePageServer(t, []FireFlyTransaction{
		{Description: "SUPERMERCATO COOP", CategoryName: "Food", BudgetName: "Needs"},
		{Description: "BONIFICO GENERICO", CategoryName: "", BudgetName: ""},
	})
	defer srv.Close()

	cfg := &config.Config{APIKey: "test-key", FFApp: srv.URL}
	client := NewClient(cfg, srv.Client())

	got, err := client.FetchTransactions(nil, nil)

	require.NoError(t, err)
	assert.Equal(t, classifier.TransactionDataSet{{"Needs", "SUPERMERCATO COOP", "Food"}}, got)
}

func TestFetchTransactions_ReturnsSingleTriple(t *testing.T) {
	srv := singlePageServer(t, []FireFlyTransaction{
		{Description: "SUPERMERCATO COOP", CategoryName: "Food", BudgetName: "Needs"},
	})
	defer srv.Close()

	cfg := &config.Config{APIKey: "test-key", FFApp: srv.URL}
	client := NewClient(cfg, srv.Client())

	got, err := client.FetchTransactions(nil, nil)

	require.NoError(t, err)
	assert.Equal(t, classifier.TransactionDataSet{{"Needs", "SUPERMERCATO COOP", "Food"}}, got)
}
