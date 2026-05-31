# PRD: ffiibtc — Firefly III Budget Transaction Classification

## Problem Statement

When adding transactions to Firefly III (manually or via import), users must manually assign a budget to each transaction. This is tedious and error-prone, especially when the correct budget can be inferred from the transaction description and category. There is no automated way to assign budgets based on historical spending patterns.

## Solution

A standalone web service that automatically assigns budgets to Firefly III transactions using Naive Bayesian classification. The service trains on historical transactions (description + category → budget) and is triggered by Firefly III webhooks. It works independently or chained after ffiiitc (the category classifier).

## User Stories

1. As a Firefly III user, I want new transactions to automatically receive a budget assignment, so that I don't have to manually assign budgets every time.
2. As a Firefly III user, I want the classifier to use both the transaction description and category to predict the budget, so that predictions are more accurate when category is available.
3. As a Firefly III user, I want the classifier to still work when no category is present, so that it functions independently without ffiiitc.
4. As a Firefly III user, I want transactions that already have a budget assigned to be skipped, so that my manual budget assignments are never overwritten.
5. As a Firefly III user, I want a service-specific tag added to classified transactions, so that I can identify which budgets were auto-assigned.
6. As a Firefly III user, I want transactions that already have the service tag to be skipped, so that transactions are not re-processed.
7. As a Firefly III user, I want to force-retrain the model via an HTTP endpoint, so that the model learns from my corrections without rebuilding the container.
8. As a Firefly III user, I want to optionally limit retraining to a date range, so that I can control which transactions influence the model.
9. As a Firefly III user, I want the model to hot-reload after retraining, so that I don't have to restart the container to pick up the new model.
10. As a Firefly III user, I want the service to run as a Docker container alongside Firefly III, so that deployment is consistent with my existing setup.
11. As a Firefly III user, I want the service to train itself on first startup from my existing transaction history, so that it works immediately without manual setup beyond configuration.
12. As a Firefly III user, I want the trained model persisted to a file, so that subsequent container restarts don't require retraining.
13. As a Firefly III user, I want the service to set `fire_webhooks: false` when updating transactions, so that no infinite webhook loops occur.
14. As a Firefly III user, I want to configure the service via environment variables (API key, Firefly URL), so that it follows the same pattern as ffiiitc.
15. As a Firefly III user, I want the service to handle paginated API responses, so that it trains on my full transaction history regardless of size.

## Implementation Decisions

### Architecture

- Standalone Go service, same structure as ffiiitc: `main.go`, `internal/classifier`, `internal/firefly`, `internal/handlers`, `internal/router`, `internal/config`.
- Uses the `navossoc/bayesian` Naive Bayes library, same as ffiiitc.
- Deployed as a Docker container on the same network as Firefly III.

### Feature Extraction

- Features are extracted from the transaction description: split by spaces, filter out single-character tokens and pure numeric tokens.
- When category is available (non-empty), it is appended as an additional feature token to the feature set. This gives it strong weight in the Bayesian model.
- Training always uses description + category (historical transactions all have categories). At prediction time, category may be absent — the model still works using description features alone, with reduced accuracy.

### Training Data

- Fetched from Firefly III API: for each transaction, extract `budget_name`, `description`, and `category_name`.
- The `FireFlyTransaction` struct must be extended to include `budget_name` from the API response.
- Training dataset format: `[budget, description, category]` triples (compared to ffiiitc's `[category, description]` pairs).
- Transactions without a budget assigned are excluded from training data.

### Webhook Handling

- Endpoint: `POST /classify` — receives webhook payload from Firefly III.
- Skip logic (in order):
  1. If transaction already has the service tag → skip.
  2. If transaction already has a budget assigned → skip.
  3. Otherwise → classify and update.
- On update: set `budget_name`, append service tag, set `fire_webhooks: false`.

### Force Retraining

- Endpoint: `GET /train` with optional `start` and `end` query parameters (format: `yyyy-mm-dd`).
- After training completes and model is saved to file, the in-memory classifier pointer is swapped atomically (hot-reload). No container restart required.

### Configuration

- `FF_API_KEY` / `FF_API_KEY_FILE` — Firefly III personal access token.
- `FF_APP_URL` — Firefly III base URL.
- Model file path: `data/model.gob`.
- Supports Docker secrets via `_FILE` suffix convention, same as ffiiitc.

### Chaining with ffiiitc (optional, out of scope for initial build)

- To chain: change ffiiitc's `fire_webhooks` to `true`, configure a Firefly "on transaction update" webhook pointing to ffiibtc's `/classify` endpoint.
- ffiibtc sets `fire_webhooks: false`, terminating the chain.

## Testing Decisions

A good test for this service tests external behavior through the public interfaces, not internal implementation details. Tests should verify what the service does given specific inputs, not how it does it internally.

### Seams to test

1. **Classifier** — Given a training dataset, verify that `ClassifyTransaction(description, category)` returns the correct budget. Test that including category improves accuracy over description-only. Test the fallback when category is empty.

2. **Feature extraction** — Given a description string and optional category, verify the correct feature set is produced (filtering rules applied, category appended when present, absent when empty).

3. **Webhook handler (skip logic)** — Given webhook payloads with various states (budget already set, tag already present, both empty), verify the handler skips or processes correctly. Verify the response includes the correct `budget_name`, tag, and `fire_webhooks: false`.

4. **Model hot-reload** — After calling `/train`, verify that subsequent classification calls use the new model. This can be tested by training with dataset A, classifying, then retraining with dataset B, and verifying the prediction changes.

### Prior art

- `internal/router/router_test.go` in ffiiitc for HTTP routing tests.
- `internal/config/config_test.go` in ffiiitc for configuration tests.

## Out of Scope

- Chaining ffiibtc with ffiiitc (the `fire_webhooks: true` change in ffiiitc). That's a separate, one-line change to the other repo.
- UI or dashboard for monitoring predictions.
- Confidence thresholds (skipping low-confidence predictions). Can be added later.
- Online learning (incremental training on corrections without full retrain).
- Multi-language support.

## Further Notes

- The service will be deployed on a Raspberry Pi, so resource usage should remain minimal. Naive Bayes is appropriate for this constraint.
- Budget names in Firefly III: Needs, Extra, Risparmi, Fun, Gestione terreno, Vacanze, Scooter.
- The Bayesian model requires at least 2 different budgets in the training data to initialize (same constraint as ffiiitc with categories).
- Since `gh` CLI is not available on this machine, this PRD should be published as a GitHub issue manually or once `gh` is installed.
