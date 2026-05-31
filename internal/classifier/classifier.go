package classifier

import (
	"regexp"
	"slices"
	"strings"

	"github.com/navossoc/bayesian"
)

type BudgetClassifier struct {
	classifier *bayesian.Classifier
}

type TransactionDataSet [][]string

func NewBudgetClassifierFromFile(modelFile string) (*BudgetClassifier, error) {
	cls, err := bayesian.NewClassifierFromFile(modelFile)
	if err != nil {
		return nil, err
	}
	return &BudgetClassifier{classifier: cls}, nil
}

func NewBudgetClassifierWithTraining(dataSet TransactionDataSet) (*BudgetClassifier, error) {
	trainingMap := buildTrainingMap(dataSet)
	classes := classesFromMap(trainingMap)
	cls := bayesian.NewClassifier(classes...)
	for _, class := range classes {
		cls.Learn(trainingMap[string(class)], class)
	}
	return &BudgetClassifier{classifier: cls}, nil
}

func (bc *BudgetClassifier) ClassifyTransaction(description, category string) string {
	features := extractFeatures(description, category)
	_, likely, _ := bc.classifier.LogScores(features)
	return string(bc.classifier.Classes[likely])
}

func (bc *BudgetClassifier) SaveClassifierToFile(modelFile string) error {
	return bc.classifier.WriteToFile(modelFile)
}

func extractFeatures(description, category string) []string {
	var features []string
	for _, token := range strings.Split(description, " ") {
		if validToken(token) && !slices.Contains(features, token) {
			features = append(features, token)
		}
	}
	if category != "" {
		features = append(features, category)
	}
	return features
}

func buildTrainingMap(dataSet TransactionDataSet) map[string][]string {
	result := make(map[string][]string)
	for _, row := range dataSet {
		budget, description, category := row[0], row[1], row[2]
		features := extractFeatures(description, category)
		result[budget] = append(result[budget], features...)
	}
	return result
}

func classesFromMap(m map[string][]string) []bayesian.Class {
	var classes []bayesian.Class
	for k := range m {
		classes = append(classes, bayesian.Class(k))
	}
	return classes
}

func validToken(s string) bool {
	return len(s) > 1 && !isNumeric(s)
}

func isNumeric(s string) bool {
	matched, err := regexp.MatchString(`^-?\d+(\.\d+)?$`, s)
	return err == nil && matched
}
