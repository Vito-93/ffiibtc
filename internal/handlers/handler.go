package handlers

import (
	"bytes"
	"encoding/json"
	"ffiiibc/internal/classifier"
	"ffiiibc/internal/firefly"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/go-pkgz/lgr"
)

const ServiceTag = "ffiiibc"

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
	Logger    *lgr.Logger
}

func NewServer(cls *classifier.BudgetClassifier, updater TransactionUpdater, fetcher TransactionFetcher, modelFile string, logger *lgr.Logger) *Server {
	s := &Server{Updater: updater, Fetcher: fetcher, ModelFile: modelFile, Logger: logger}
	s.cls.Store(cls)
	return s
}

func (s *Server) logf(format string, args ...interface{}) {
	if s.Logger != nil {
		s.Logger.Logf(format, args...)
	}
}

func (s *Server) HandleClassify(w http.ResponseWriter, r *http.Request) {
	bodyBytes, _ := io.ReadAll(r.Body)

	var payload firefly.WebhookPayload
	if err := json.NewDecoder(bytes.NewReader(bodyBytes)).Decode(&payload); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(payload.Content.Transactions) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	tx := payload.Content.Transactions[0]

	s.logf(
		"INFO hook classify: received (id: %v) (description: %s) (category: %s)",
		payload.Content.ID,
		tx.Description,
		tx.CategoryName,
	)

	if slices.Contains(tx.Tags, ServiceTag) {
		s.logf("INFO hook classify: skipped (id: %v), tag %s already present", payload.Content.ID, ServiceTag)
		w.WriteHeader(http.StatusOK)
		return
	}

	if tx.BudgetName != "" {
		s.logf("INFO hook classify: skipped (id: %v), budget already set to %s", payload.Content.ID, tx.BudgetName)
		w.WriteHeader(http.StatusOK)
		return
	}

	budget := s.cls.Load().ClassifyTransaction(tx.Description, tx.CategoryName)
	s.logf("INFO hook classify: classified (id: %v) (budget: %s)", payload.Content.ID, budget)
	newTags := append(tx.Tags, ServiceTag)

	journalID, err := strconv.Atoi(tx.TransactionJournalID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := s.Updater.UpdateTransaction(payload.Content.ID, journalID, budget, newTags); err != nil {
		s.logf("ERROR hook classify: error updating (id: %v) %v", payload.Content.ID, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	s.logf("INFO hook classify: updated (id: %v)", payload.Content.ID)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) HandleTrain(w http.ResponseWriter, r *http.Request) {
	s.logf("INFO Received request to perform force training")
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

	s.logf("INFO Requesting transactions data from Firefly")
	dataset, err := s.Fetcher.FetchTransactions(start, end)
	if err != nil || len(dataset) == 0 {
		s.logf("ERROR: Error while getting transactions data\n %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	s.logf("DEBUG Got training data with %d entries", len(dataset))
	bc, err := classifier.NewBudgetClassifierWithTraining(dataset)
	if err != nil {
		s.logf("ERROR creating classifier from dataset:\n %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	s.logf("INFO forced training completed...")
	s.logf("INFO saving data to model...")

	if err := os.MkdirAll(filepath.Dir(s.ModelFile), 0755); err != nil {
		s.logf("ERROR creating model directory: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := bc.SaveClassifierToFile(s.ModelFile); err != nil {
		s.logf("ERROR saving model to file:\n %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	s.cls.Store(bc)
	s.logf("INFO forced training completed and model saved. Hot-reloaded.")
	w.WriteHeader(http.StatusOK)
}
