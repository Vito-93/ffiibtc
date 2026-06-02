package handlers_test

import (
	"bytes"
	"encoding/json"
	"ffiiibc/internal/classifier"
	"ffiiibc/internal/firefly"
	"ffiiibc/internal/handlers"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- fakes ---

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

type fakeFetcher struct {
	dataset    classifier.TransactionDataSet
	calledWith [2]*time.Time
}

func (f *fakeFetcher) FetchTransactions(start, end *time.Time) (classifier.TransactionDataSet, error) {
	f.calledWith[0] = start
	f.calledWith[1] = end
	return f.dataset, nil
}

// --- helpers ---

func buildTestClassifier(t *testing.T, ds classifier.TransactionDataSet) *classifier.BudgetClassifier {
	t.Helper()
	cls, err := classifier.NewBudgetClassifierWithTraining(ds)
	require.NoError(t, err)
	return cls
}

var needsDataset = classifier.TransactionDataSet{
	{"Needs", "SUPERMERCATO COOP", "Food"},
	{"Needs", "FARMACIA CENTRALE", "Health"},
}

var funDataset = classifier.TransactionDataSet{
	{"Fun", "NETFLIX", "Entertainment"},
	{"Fun", "SPOTIFY", "Entertainment"},
}

var mixedDataset = classifier.TransactionDataSet{
	{"Needs", "SUPERMERCATO COOP", "Food"},
	{"Needs", "FARMACIA CENTRALE", "Health"},
	{"Fun", "NETFLIX", "Entertainment"},
	{"Fun", "SPOTIFY", "Entertainment"},
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

func getTrain(t *testing.T, srv *handlers.Server, query string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/train"+query, nil)
	w := httptest.NewRecorder()
	srv.HandleTrain(w, req)
	return w.Result()
}

// --- classify tests ---

func TestClassifyHandler_SkipsTransactionWithServiceTag(t *testing.T) {
	cls := buildTestClassifier(t, mixedDataset)
	updater := &fakeUpdater{}
	srv := handlers.NewServer(cls, updater, nil, "", nil)

	payload := firefly.WebhookPayload{
		Content: firefly.WebhookContent{
			ID: 1,
			Transactions: []firefly.WebhookTransaction{{
				TransactionJournalID: "10",
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
	cls := buildTestClassifier(t, mixedDataset)
	updater := &fakeUpdater{}
	srv := handlers.NewServer(cls, updater, nil, "", nil)

	payload := firefly.WebhookPayload{
		Content: firefly.WebhookContent{
			ID: 2,
			Transactions: []firefly.WebhookTransaction{{
				TransactionJournalID: "20",
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
	cls := buildTestClassifier(t, mixedDataset)
	updater := &fakeUpdater{}
	srv := handlers.NewServer(cls, updater, nil, "", nil)

	payload := firefly.WebhookPayload{
		Content: firefly.WebhookContent{
			ID: 3,
			Transactions: []firefly.WebhookTransaction{{
				TransactionJournalID: "30",
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

// --- train tests ---

func TestTrainHandler_Returns200(t *testing.T) {
	dir := t.TempDir()
	modelFile := filepath.Join(dir, "model.gob")
	cls := buildTestClassifier(t, mixedDataset)
	fetcher := &fakeFetcher{dataset: mixedDataset}
	srv := handlers.NewServer(cls, &fakeUpdater{}, fetcher, modelFile, nil)

	resp := getTrain(t, srv, "")

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestTrainHandler_PersistsModelToFile(t *testing.T) {
	dir := t.TempDir()
	modelFile := filepath.Join(dir, "model.gob")
	cls := buildTestClassifier(t, mixedDataset)
	fetcher := &fakeFetcher{dataset: mixedDataset}
	srv := handlers.NewServer(cls, &fakeUpdater{}, fetcher, modelFile, nil)

	getTrain(t, srv, "")

	_, err := os.Stat(modelFile)
	assert.NoError(t, err, "model file should be persisted after train")
}

func TestTrainHandler_HotReloadsClassifier(t *testing.T) {
	dir := t.TempDir()
	modelFile := filepath.Join(dir, "model.gob")

	// initial model: NETFLIX labelled as Needs, so it will predict "Needs"
	initialDataset := classifier.TransactionDataSet{
		{"Needs", "NETFLIX", "Entertainment"},
		{"Fun", "FARMACIA", "Health"},
	}
	cls := buildTestClassifier(t, initialDataset)

	// retrain dataset: NETFLIX labelled as Fun, so it will predict "Fun"
	retrainDataset := classifier.TransactionDataSet{
		{"Fun", "NETFLIX", "Entertainment"},
		{"Needs", "SUPERMERCATO", "Food"},
	}
	fetcher := &fakeFetcher{dataset: retrainDataset}
	updater := &fakeUpdater{}
	srv := handlers.NewServer(cls, updater, fetcher, modelFile, nil)

	budgetBefore := classifyDescription(t, srv, updater, "NETFLIX", "Entertainment")
	assert.Equal(t, "Needs", budgetBefore, "initial model should predict Needs for NETFLIX")

	getTrain(t, srv, "")

	budgetAfter := classifyDescription(t, srv, updater, "NETFLIX", "Entertainment")
	assert.Equal(t, "Fun", budgetAfter, "after retrain, model should predict Fun for NETFLIX")
}

func TestTrainHandler_PassesStartDateToFetcher(t *testing.T) {
	dir := t.TempDir()
	modelFile := filepath.Join(dir, "model.gob")
	cls := buildTestClassifier(t, mixedDataset)
	fetcher := &fakeFetcher{dataset: mixedDataset}
	srv := handlers.NewServer(cls, &fakeUpdater{}, fetcher, modelFile, nil)

	getTrain(t, srv, "?start=2024-01-15")

	require.NotNil(t, fetcher.calledWith[0])
	assert.Equal(t, "2024-01-15", fetcher.calledWith[0].Format("2006-01-02"))
	assert.Nil(t, fetcher.calledWith[1])
}

func TestTrainHandler_PassesEndDateToFetcher(t *testing.T) {
	dir := t.TempDir()
	modelFile := filepath.Join(dir, "model.gob")
	cls := buildTestClassifier(t, mixedDataset)
	fetcher := &fakeFetcher{dataset: mixedDataset}
	srv := handlers.NewServer(cls, &fakeUpdater{}, fetcher, modelFile, nil)

	getTrain(t, srv, "?end=2024-12-31")

	assert.Nil(t, fetcher.calledWith[0])
	require.NotNil(t, fetcher.calledWith[1])
	assert.Equal(t, "2024-12-31", fetcher.calledWith[1].Format("2006-01-02"))
}

func TestTrainHandler_InvalidStartDate_Returns400(t *testing.T) {
	dir := t.TempDir()
	modelFile := filepath.Join(dir, "model.gob")
	cls := buildTestClassifier(t, mixedDataset)
	fetcher := &fakeFetcher{dataset: mixedDataset}
	srv := handlers.NewServer(cls, &fakeUpdater{}, fetcher, modelFile, nil)

	resp := getTrain(t, srv, "?start=not-a-date")

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestTrainHandler_InvalidEndDate_Returns400(t *testing.T) {
	dir := t.TempDir()
	modelFile := filepath.Join(dir, "model.gob")
	cls := buildTestClassifier(t, mixedDataset)
	fetcher := &fakeFetcher{dataset: mixedDataset}
	srv := handlers.NewServer(cls, &fakeUpdater{}, fetcher, modelFile, nil)

	resp := getTrain(t, srv, "?end=not-a-date")

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// classifyDescription sends a classify request and returns the budget that the updater received.
func classifyDescription(t *testing.T, srv *handlers.Server, updater *fakeUpdater, description, category string) string {
	t.Helper()
	updater.called = false
	updater.budgetName = ""
	payload := firefly.WebhookPayload{
		Content: firefly.WebhookContent{
			ID: 99,
			Transactions: []firefly.WebhookTransaction{{
				TransactionJournalID: "99",
				Description:          description,
				CategoryName:         category,
				BudgetName:           "",
				Tags:                 []string{},
			}},
		},
	}
	postClassify(t, srv, payload)
	return updater.budgetName
}
