# FireFly III Budget Classification

This little web service performs transaction budget classification and integrates with [FireFly III](https://github.com/firefly-iii/firefly-iii) (A free and open source personal finance manager) via web hooks.

### What it does?

Every time you add a new transaction to FireFly III, either manually or via import tool, a web hook triggers and provides the transaction description (and optionally its category) to `ffiiibc`. It will then be classified using [Naive Bayesian Classification](https://en.wikipedia.org/wiki/Naive_Bayes_classifier) and the transaction will be updated with the matching budget.

> Naive Bayesian classifier go package used by `ffiiibc` is available [here](https://github.com/navossoc/bayesian). Please read the [license](https://github.com/navossoc/bayesian/blob/master/LICENSE).

### How it differs from ffiiitc?

| | ffiiitc | ffiiibc |
|---|---|---|
| Classifies | Category | Budget |
| Webhook trigger | After transaction creation | After transaction creation or after transaction update |
| Features used | Description only | Description + category (when available) |
| Tag applied | `ffiiitc` | `ffiiibc` |

ffiiibc can be used standalone or chained after ffiiitc (see [Chained mode](#chained-mode-with-ffiiitc) below).

### How to run?

#### Pre-requisites

- [Docker desktop](https://www.docker.com/products/docker-desktop/) or any other form of running containers on your computer
- [FireFly III](https://github.com/firefly-iii/firefly-iii) up and running as per [docs](https://docs.firefly-iii.org/firefly-iii/installation/docker/?mtm_campaign=docu-internal&mtm_kwd=docker)
- At least **a few transactions** imported into FireFly with budgets manually **assigned**. This is required for the classifier to train on your dataset and is very important.
- Have a personal access token (PAT) generated in FireFly III. Go to `Options → Profile → OAuth` and click `Create new token`.

##### Docker Compose

- `git clone https://github.com/YOUR_GITHUB_USER/ffiiibc.git`
- `docker buildx build --load --platform=linux/amd64 -t ffiiibc:latest .`

#### Run

##### Docker Compose

- Stop FireFly III: `docker compose -f docker-compose.yml down`
- Modify your FireFly III docker compose file and add the following:

```yaml
  ffbc:
    image: YOUR_DOCKERHUB_USER/ffiiibc:latest
    hostname: ffbc
    networks:
      - firefly_iii
    restart: always
    container_name: ffiiibc
    environment:
      - FF_API_KEY=<YOUR_PAT_GOES_HERE>
      - FF_APP_URL=<FIREFLY_ADDRESS:PORT>
    volumes:
      - ffiiibc-data:/app/data
    ports:
      - '<EXPOSED_PORT>:8080'
    depends_on:
      - app

volumes:
    ...
    ffiiibc-data:
```

You can also append your environment variable names with `_FILE` instead, having their value point to the file where the actual sensitive value is stored. This works with any environment variable.

```yaml
secrets:
  ffiiibc-personal-access-token:
    file: "<path/to/secrets/location>/ffiiibc-personal-access-token"

services:
  ...
  ffbc:
    image: YOUR_DOCKERHUB_USER/ffiiibc:latest
    hostname: ffbc
    networks:
      - firefly_iii
    restart: always
    container_name: ffiiibc
    secrets:
      - "ffiiibc-personal-access-token"
    environment:
      - FF_API_KEY_FILE="/run/secrets/ffiiibc-personal-access-token"
      - FF_APP_URL=<FIREFLY_ADDRESS:PORT>
    volumes:
      - ffiiibc-data:/app/data
    ports:
      - '<EXPOSED_PORT>:8080'
    depends_on:
      - app

volumes:
  ...
  ffiiibc-data:
```

- Start: `docker compose -f docker-compose.yml up -d`

##### Docker

```
docker run \
  -d \
  --name='ffiiibc' \
  -e 'FF_API_KEY'='<YOUR_PAT_GOES_HERE>' \
  -e 'FF_APP_URL'='<FIREFLY_ADDRESS:PORT>' \
  -p '<EXPOSED_PORT>:8080' \
  -v '<TRAINED_MODEL_FOLDER>':'/app/data':'rw' \
  'YOUR_DOCKERHUB_USER/ffiiibc'
```

### Configure Webhooks in FireFly III

In FireFly III, go to `Automation → Webhooks` and click `Create new webhook`.

#### Standalone mode

Create a single webhook that triggers on transaction creation:

```yaml
title: classify_budget
trigger: after transaction creation
response: transaction details
delivery: json
url: http://ffbc:<EXPOSED_PORT>/classify
active: checked
```

This works best when transactions already have a category assigned (manually or via import). It still works without a category — the classifier will use the description alone to predict the budget.

#### Chained mode (with ffiiitc)

Use this mode when you want both automatic category classification (ffiiitc) AND automatic budget classification (ffiiibc). The chain works as follows:

1. Transaction is created in FireFly III
2. "After transaction creation" webhook fires → ffiiitc receives it
3. ffiiitc classifies the **category** (or preserves it if already set), adds its `ffiiitc` tag, and updates the transaction with `fire_webhooks: true`
4. The update triggers an "After transaction update" webhook → ffiiibc receives it
5. ffiiibc classifies the **budget** (using the category as a feature), adds its `ffiiibc` tag, and updates the transaction with `fire_webhooks: false` — terminating the chain

**Guard rules that prevent loops and overwrites:**

| Service | Skips if tag present? | Preserves existing value? | `fire_webhooks` |
|---------|----------------------|---------------------------|-----------------|
| ffiiitc | ✅ `ffiiitc` tag | ✅ Preserves category if already set | `true` (configurable) |
| ffiiibc | ✅ `ffiiibc` tag | ✅ Skips if budget already set | `false` (hardcoded) |

Because ffiiitc preserves existing categories and skips when its tag is present, manually categorized transactions are never overwritten. Because ffiiibc terminates the chain with `fire_webhooks: false`, there is no infinite loop.

**Setup:**

1. **ffiiitc**: You need a version of ffiiitc that supports chained mode (tag guard, category preservation, configurable `fire_webhooks`). A fork with these features is available at [Vito-93/ffiiitc](https://github.com/Vito-93/ffiiitc) on the `ffiiibc-workflow-adapter` branch. Set the environment variable `FF_FIRE_WEBHOOKS=true` (default on that branch) to enable chaining, or `FF_FIRE_WEBHOOKS=false` for standalone mode.

2. **Create two webhooks in FireFly III:**

   **Webhook 1** — category classification (on create):
   ```yaml
   title: classify_category
   trigger: after transaction creation
   response: transaction details
   delivery: json
   url: http://fftc:<FFTC_PORT>/classify
   active: checked
   ```

   **Webhook 2** — budget classification (on update):
   ```yaml
   title: classify_budget
   trigger: after transaction update
   response: transaction details
   delivery: json
   url: http://ffbc:<FFBC_PORT>/classify
   active: checked
   ```

   > ⚠️ **Do not** point both webhooks at the same service — that would create a loop. ffiiitc only listens on "after transaction creation" and ffiiibc only listens on "after transaction update".

### Troubleshooting

#### Logs

You can check `ffiiibc` logs to see if there are any errors:
```
docker compose logs ffbc -f
```

For chained mode, also check ffiiitc logs:
```
docker compose logs fftc -f
```

#### Checking the classifier state

To see which budgets the classifier has learned, check the startup logs:
```
docker compose logs ffbc | grep "learned classes"
```

#### Forced training of your model

There is an option to force retrain the model from your transactions if required. To trigger force training run:

```
curl -i http://localhost:<EXPOSED_PORT>/train
```

where `EXPOSED_PORT` is the port you provided in your docker compose for `ffbc`. The model is hot-reloaded after training — no container restart needed.

You can also provide optional `start` and `end` date query parameters (in `yyyy-mm-dd` format) to limit the transactions used for training. For example:

```
curl -i "http://localhost:<EXPOSED_PORT>/train?start=2024-01-01&end=2024-06-01"
```

If `start` and/or `end` are omitted, all available transactions will be used for training.
