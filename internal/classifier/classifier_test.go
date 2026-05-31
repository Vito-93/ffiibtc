package classifier

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractFeatures_FiltersShortAndNumericTokens(t *testing.T) {
	got := extractFeatures("AMAZON 3 a PRIME 99.50", "")
	assert.Equal(t, []string{"AMAZON", "PRIME"}, got)
}

func TestExtractFeatures_AppendsCategoryWhenPresent(t *testing.T) {
	got := extractFeatures("RESTAURANT LUIGI", "Food")
	assert.Equal(t, []string{"RESTAURANT", "LUIGI", "Food"}, got)
}

func TestExtractFeatures_OmitsCategoryWhenEmpty(t *testing.T) {
	got := extractFeatures("RESTAURANT LUIGI", "")
	assert.Equal(t, []string{"RESTAURANT", "LUIGI"}, got)
}

func TestClassifyTransaction_PredictsBudgetFromTraining(t *testing.T) {
	dataset := TransactionDataSet{
		{"Needs", "SUPERMERCATO COOP alimentari", "Food"},
		{"Needs", "FARMACIA CENTRALE medicina", "Health"},
		{"Fun", "NETFLIX abbonamento streaming", "Entertainment"},
		{"Fun", "CINEMA ODEON biglietto film", "Entertainment"},
	}
	bc, err := NewBudgetClassifierWithTraining(dataset)
	assert.NoError(t, err)

	assert.Equal(t, "Needs", bc.ClassifyTransaction("SUPERMERCATO spesa", "Food"))
	assert.Equal(t, "Fun", bc.ClassifyTransaction("NETFLIX streaming", "Entertainment"))
}

func TestClassifyTransaction_CategoryImprovesAccuracy(t *testing.T) {
	// Train with ambiguous descriptions that only differ by category
	dataset := TransactionDataSet{
		{"Needs", "PAGAMENTO ONLINE acquisto", "Health"},
		{"Needs", "PAGAMENTO ONLINE acquisto", "Health"},
		{"Fun", "PAGAMENTO ONLINE acquisto", "Entertainment"},
		{"Fun", "PAGAMENTO ONLINE acquisto", "Entertainment"},
		{"Fun", "PAGAMENTO ONLINE acquisto", "Entertainment"},
	}
	bc, err := NewBudgetClassifierWithTraining(dataset)
	assert.NoError(t, err)

	// Without category the description is ambiguous — with category it disambiguates
	withCategory := bc.ClassifyTransaction("PAGAMENTO ONLINE acquisto", "Health")
	assert.Equal(t, "Needs", withCategory)

	withCategory2 := bc.ClassifyTransaction("PAGAMENTO ONLINE acquisto", "Entertainment")
	assert.Equal(t, "Fun", withCategory2)
}

func TestClassifyTransaction_WorksWithoutCategory(t *testing.T) {
	dataset := TransactionDataSet{
		{"Needs", "SUPERMERCATO COOP spesa alimentari", "Food"},
		{"Needs", "FARMACIA CENTRALE medicina salute", "Health"},
		{"Fun", "NETFLIX streaming film serie", "Entertainment"},
		{"Fun", "SPOTIFY musica streaming", "Entertainment"},
	}
	bc, err := NewBudgetClassifierWithTraining(dataset)
	assert.NoError(t, err)

	// No category — relies on description alone
	assert.Equal(t, "Needs", bc.ClassifyTransaction("SUPERMERCATO spesa", ""))
	assert.Equal(t, "Fun", bc.ClassifyTransaction("NETFLIX streaming", ""))
}

func TestPersistence_SaveAndLoadProducesSameResults(t *testing.T) {
	dataset := TransactionDataSet{
		{"Needs", "SUPERMERCATO COOP spesa alimentari", "Food"},
		{"Needs", "FARMACIA CENTRALE medicina salute", "Health"},
		{"Fun", "NETFLIX streaming film serie", "Entertainment"},
		{"Fun", "SPOTIFY musica streaming", "Entertainment"},
	}
	bc, err := NewBudgetClassifierWithTraining(dataset)
	require.NoError(t, err)

	modelFile := filepath.Join(t.TempDir(), "model.gob")
	require.NoError(t, bc.SaveClassifierToFile(modelFile))

	loaded, err := NewBudgetClassifierFromFile(modelFile)
	require.NoError(t, err)

	assert.Equal(t, bc.ClassifyTransaction("SUPERMERCATO spesa", "Food"), loaded.ClassifyTransaction("SUPERMERCATO spesa", "Food"))
	assert.Equal(t, bc.ClassifyTransaction("NETFLIX streaming", "Entertainment"), loaded.ClassifyTransaction("NETFLIX streaming", "Entertainment"))

	os.Remove(modelFile)
}
