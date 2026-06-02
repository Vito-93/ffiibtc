package router

import (
	"bytes"
	"encoding/json"
	"ffiiibc/internal/classifier"
	"ffiiibc/internal/firefly"
	"ffiiibc/internal/handlers"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthEndpoint(t *testing.T) {
	r := NewRouter()
	r.AddRoute("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewServer(r.Mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/health")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

type noopUpdater struct{}

func (n *noopUpdater) UpdateTransaction(_, _ int, _ string, _ []string) error { return nil }

func TestClassifyEndpointWiredCorrectly(t *testing.T) {
	ds := classifier.TransactionDataSet{
		{"Needs", "SUPERMERCATO COOP", "Food"},
		{"Fun", "NETFLIX", "Entertainment"},
	}
	cls, err := classifier.NewBudgetClassifierWithTraining(ds)
	require.NoError(t, err)

	srv := handlers.NewServer(cls, &noopUpdater{}, nil, "", nil)

	r := NewRouter()
	r.AddRoute("POST /classify", srv.HandleClassify)

	server := httptest.NewServer(r.Mux)
	defer server.Close()

	payload := firefly.WebhookPayload{
		Content: firefly.WebhookContent{
			ID: 1,
			Transactions: []firefly.WebhookTransaction{{
				TransactionJournalID: "10",
				Description:          "SUPERMERCATO",
				Tags:                 []string{},
			}},
		},
	}
	data, err := json.Marshal(payload)
	require.NoError(t, err)

	resp, err := http.Post(server.URL+"/classify", "application/json", bytes.NewReader(data))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRouter_AddRoute(t *testing.T) {
	r := NewRouter()
	r.AddRoute("/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	server := httptest.NewServer(r.Mux)
	defer server.Close()

	resp, err := http.Get(server.URL + "/test")
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
