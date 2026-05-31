package bootstrap

import (
	"ffiibtc/internal/classifier"
	"os"
	"path/filepath"
	"time"

	"github.com/go-pkgz/lgr"
)

type TransactionFetcher interface {
	FetchTransactions(start, end *time.Time) (classifier.TransactionDataSet, error)
}

func LoadOrTrain(modelFile string, fetcher TransactionFetcher, l *lgr.Logger) (*classifier.BudgetClassifier, error) {
	if _, err := os.Stat(modelFile); err == nil {
		l.Logf("INFO loading classifier from %s", modelFile)
		return classifier.NewBudgetClassifierFromFile(modelFile)
	}

	l.Logf("INFO model file not found, fetching transactions and training classifier")
	dataset, err := fetcher.FetchTransactions(nil, nil)
	if err != nil {
		return nil, err
	}

	bc, err := classifier.NewBudgetClassifierWithTraining(dataset)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(modelFile), 0755); err != nil {
		return nil, err
	}

	if err := bc.SaveClassifierToFile(modelFile); err != nil {
		return nil, err
	}

	l.Logf("INFO classifier trained and saved to %s", modelFile)
	return bc, nil
}
