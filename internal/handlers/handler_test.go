package handlers_test

import (
	"bytes"
	"encoding/json"
	"ffiibtc/internal/classifier"
	"ffiibtc/internal/firefly"
	"ffiibtc/internal/handlers"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeUpdater struct {
	called     bool
	groupID    int
	journalID  int
	budgetName string
	tags       []string
}

func (f *fakeUpdater) UpdateTransaction(groupID int, journalID int, budgetName string, tags []string) error {
	f.called = true
	f.groupID = groupID
	f.journalID = journalID
	f.budgetName = budgetName
	f.tags = tags
	return nil
}

func buildTestClassifier(t *testing.T) *classifier.BudgetClassifier {
	t.Helper()
	ds := classifier.TransactionDataSet{
		{"Needs", "SUPERMERCATO COOP", "Food"},
		{"Needs", "FARMACIA CENTRALE", "Health"},
		{"Fun", "NETFLIX", "Entertainment"},
		{"Fun", "SPOTIFY", "Entertainment"},
	}
	cls, err := classifier.NewBudgetClassifierWithTraining(ds)
	require.NoError(t, err)
	return cls
}

func postClassify(t *testing.T, srv *handlers.Server, payload firefly.WebhookPayload) *http.Response {
	t.Helper()
	data, err := json.Marshal(payload)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/classify", bytes.NewReader(data))
	w := httptest.NewRecorder()
	srv.HandleClassify(w, req)
	return w.Result()
}

func TestClassifyHandler_SkipsTransactionWithServiceTag(t *testing.T) {
	cls := buildTestClassifier(t)
	updater := &fakeUpdater{}
	srv := handlers.NewServer(cls, updater)

	payload := firefly.WebhookPayload{
		Content: firefly.WebhookContent{
			ID: 1,
			Transactions: []firefly.WebhookTransaction{{
				TransactionJournalID: 10,
				Description:          "SUPERMERCATO COOP",
				CategoryName:         "Food",
				BudgetName:           "",
				Tags:                 []string{handlers.ServiceTag},
			}},
		},
	}

	resp := postClassify(t, srv, payload)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.False(t, updater.called)
}

func TestClassifyHandler_SkipsTransactionWithBudgetAlreadySet(t *testing.T) {
	cls := buildTestClassifier(t)
	updater := &fakeUpdater{}
	srv := handlers.NewServer(cls, updater)

	payload := firefly.WebhookPayload{
		Content: firefly.WebhookContent{
			ID: 2,
			Transactions: []firefly.WebhookTransaction{{
				TransactionJournalID: 20,
				Description:          "SUPERMERCATO COOP",
				CategoryName:         "Food",
				BudgetName:           "Needs",
				Tags:                 []string{},
			}},
		},
	}

	resp := postClassify(t, srv, payload)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.False(t, updater.called)
}

func TestClassifyHandler_ClassifiesAndUpdatesUnbudgetedTransaction(t *testing.T) {
	cls := buildTestClassifier(t)
	updater := &fakeUpdater{}
	srv := handlers.NewServer(cls, updater)

	payload := firefly.WebhookPayload{
		Content: firefly.WebhookContent{
			ID: 3,
			Transactions: []firefly.WebhookTransaction{{
				TransactionJournalID: 30,
				Description:          "SUPERMERCATO COOP",
				CategoryName:         "Food",
				BudgetName:           "",
				Tags:                 []string{},
			}},
		},
	}

	resp := postClassify(t, srv, payload)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	require.True(t, updater.called)
	assert.Equal(t, 3, updater.groupID)
	assert.Equal(t, 30, updater.journalID)
	assert.Equal(t, "Needs", updater.budgetName)
	assert.Contains(t, updater.tags, handlers.ServiceTag)
}
