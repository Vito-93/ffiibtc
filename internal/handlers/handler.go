package handlers

import "ffiibtc/internal/classifier"

type Server struct {
	Classifier *classifier.BudgetClassifier
}

func NewServer(cls *classifier.BudgetClassifier) *Server {
	return &Server{Classifier: cls}
}
