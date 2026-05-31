package handlers

import (
	"encoding/json"
	"ffiibtc/internal/classifier"
	"ffiibtc/internal/firefly"
	"net/http"
	"slices"
)

const ServiceTag = "ffiibtc"

type TransactionUpdater interface {
	UpdateTransaction(groupID int, journalID int, budgetName string, tags []string) error
}

type Server struct {
	Classifier *classifier.BudgetClassifier
	Updater    TransactionUpdater
}

func NewServer(cls *classifier.BudgetClassifier, updater TransactionUpdater) *Server {
	return &Server{Classifier: cls, Updater: updater}
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

	budget := s.Classifier.ClassifyTransaction(tx.Description, tx.CategoryName)
	newTags := append(tx.Tags, ServiceTag)

	if err := s.Updater.UpdateTransaction(payload.Content.ID, tx.TransactionJournalID, budget, newTags); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
