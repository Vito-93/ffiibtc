package bootstrap

import (
	"ffiibtc/internal/classifier"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-pkgz/lgr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockFetcher struct {
	dataset classifier.TransactionDataSet
	calls   int
}

func (m *mockFetcher) FetchTransactions(_, _ *time.Time) (classifier.TransactionDataSet, error) {
	m.calls++
	return m.dataset, nil
}

var testDataset = classifier.TransactionDataSet{
	{"Needs", "SUPERMERCATO COOP alimentari", "Food"},
	{"Fun", "NETFLIX abbonamento streaming", "Entertainment"},
}

func TestLoadOrTrain_TrainsAndPersistsWhenModelAbsent(t *testing.T) {
	dir := t.TempDir()
	modelFile := filepath.Join(dir, "model.gob")
	fetcher := &mockFetcher{dataset: testDataset}
	l := lgr.New()

	bc, err := LoadOrTrain(modelFile, fetcher, l)

	require.NoError(t, err)
	assert.NotNil(t, bc)
	assert.Equal(t, 1, fetcher.calls)
	_, statErr := os.Stat(modelFile)
	assert.NoError(t, statErr, "model file should have been persisted")
}

func TestLoadOrTrain_LoadsFromFileWithoutFetching(t *testing.T) {
	dir := t.TempDir()
	modelFile := filepath.Join(dir, "model.gob")
	l := lgr.New()

	// pre-create model file
	trained, err := classifier.NewBudgetClassifierWithTraining(testDataset)
	require.NoError(t, err)
	require.NoError(t, trained.SaveClassifierToFile(modelFile))

	fetcher := &mockFetcher{dataset: testDataset}
	bc, err := LoadOrTrain(modelFile, fetcher, l)

	require.NoError(t, err)
	assert.NotNil(t, bc)
	assert.Equal(t, 0, fetcher.calls, "fetcher must not be called when model file exists")
}
