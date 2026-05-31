package firefly

import (
	"encoding/json"
	"ffiibtc/internal/classifier"
	"ffiibtc/internal/config"
	"fmt"
	"net/http"
	"time"
)

type FireFlyTransaction struct {
	Description  string `json:"description"`
	CategoryName string `json:"category_name"`
	BudgetName   string `json:"budget_name"`
}

type transactionAttributes struct {
	Transactions []FireFlyTransaction `json:"transactions"`
}

type transactionGroup struct {
	Attributes transactionAttributes `json:"attributes"`
}

type apiPagination struct {
	CurrentPage int `json:"current_page"`
	TotalPages  int `json:"total_pages"`
}

type apiMeta struct {
	Pagination apiPagination `json:"pagination"`
}

type apiResponse struct {
	Data []transactionGroup `json:"data"`
	Meta apiMeta            `json:"meta"`
}

type Client struct {
	cfg        *config.Config
	httpClient *http.Client
}

func NewClient(cfg *config.Config, httpClient *http.Client) *Client {
	return &Client{cfg: cfg, httpClient: httpClient}
}

func (c *Client) FetchTransactions(start, end *time.Time) (classifier.TransactionDataSet, error) {
	var dataset classifier.TransactionDataSet
	page := 1

	for {
		resp, err := c.fetchPage(page, start, end)
		if err != nil {
			return nil, err
		}

		for _, group := range resp.Data {
			for _, tx := range group.Attributes.Transactions {
				if tx.BudgetName == "" {
					continue
				}
				dataset = append(dataset, []string{tx.BudgetName, tx.Description, tx.CategoryName})
			}
		}

		if page >= resp.Meta.Pagination.TotalPages {
			break
		}
		page++
	}

	return dataset, nil
}

func (c *Client) fetchPage(page int, start, end *time.Time) (*apiResponse, error) {
	url := fmt.Sprintf("%s/api/v1/transactions?page=%d", c.cfg.FFApp, page)
	if start != nil {
		url += "&start=" + start.Format("2006-01-02")
	}
	if end != nil {
		url += "&end=" + end.Format("2006-01-02")
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)

	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("firefly API returned status %d", httpResp.StatusCode)
	}

	var result apiResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}
