package handlers

import (
	"encoding/json"
	"ffiibtc/internal/classifier"
	"ffiibtc/internal/firefly"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"time"
)

const ServiceTag = "ffiibtc"

type TransactionUpdater interface {
	UpdateTransaction(groupID int, journalID int, budgetName string, tags []string) error
}

type TransactionFetcher interface {
	FetchTransactions(start, end *time.Time) (classifier.TransactionDataSet, error)
}

type Server struct {
	cls       atomic.Pointer[classifier.BudgetClassifier]
	Updater   TransactionUpdater
	Fetcher   TransactionFetcher
	ModelFile string
}

func NewServer(cls *classifier.BudgetClassifier, updater TransactionUpdater, fetcher TransactionFetcher, modelFile string) *Server {
	s := &Server{Updater: updater, Fetcher: fetcher, ModelFile: modelFile}
	s.cls.Store(cls)
	return s
}

func (s *Server) HandleClassify(w http.ResponseWriter, r *http.Request) {
	var payload firefly.WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(payload.Content.Transactions) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	tx := payload.Content.Transactions[0]

	if slices.Contains(tx.Tags, ServiceTag) {
		w.WriteHeader(http.StatusOK)
		return
	}

	if tx.BudgetName != "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	budget := s.cls.Load().ClassifyTransaction(tx.Description, tx.CategoryName)
	newTags := append(tx.Tags, ServiceTag)

	if err := s.Updater.UpdateTransaction(payload.Content.ID, tx.TransactionJournalID, budget, newTags); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) HandleTrain(w http.ResponseWriter, r *http.Request) {
	var start, end *time.Time

	if v := r.URL.Query().Get("start"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		start = &t
	}

	if v := r.URL.Query().Get("end"); v != "" {
		t, err := time.Parse("2006-01-02", v)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		end = &t
	}

	dataset, err := s.Fetcher.FetchTransactions(start, end)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	bc, err := classifier.NewBudgetClassifierWithTraining(dataset)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := os.MkdirAll(filepath.Dir(s.ModelFile), 0755); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := bc.SaveClassifierToFile(s.ModelFile); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	s.cls.Store(bc)
	w.WriteHeader(http.StatusOK)
}
