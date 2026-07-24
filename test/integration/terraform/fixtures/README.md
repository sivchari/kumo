# Terraform e2e fixtures

Each directory here is one fixture: a self-contained `main.tf` that the
runner (`../runner_test.go`) applies against a local kumo instance through
the real `hashicorp/aws` provider. Adding a fixture never requires touching
the runner.

## Adding a fixture

1. Create a directory named after the resource under test, e.g.
   `fixtures/sqs_queue/`.
2. Add a `main.tf`. Name every resource `tf-<dirname>-...` so two fixtures
   running in parallel can never collide on a bucket/queue/table/etc. name,
   e.g. the `sqs_queue` fixture's queue is named `tf-sqs-queue-demo`.
3. Do not add a `provider` or `terraform { required_providers { ... } }`
   block — the runner generates and injects `provider.tf` for you.
4. Run the suite (see below) with `GOLDEN` not required here; the mandatory
   assertion is that a second `terraform plan` after `apply` reports no
   changes. If your fixture legitimately produces post-apply drift (e.g. a
   field kumo normalizes), fix the fixture or kumo — do not weaken the
   check.

### Optional: output assertions

If `main.tf` declares `output` blocks and the fixture directory has an
`expected_outputs.json` mapping output name to expected string value, the
runner compares them via `terraform output` after apply. See
`s3_bucket/expected_outputs.json` for an example.

### Optional: HCL-level assertions

Terraform `check` blocks only ever emit warnings on `plan`/`apply`, so they
cannot fail a fixture — don't rely on them for hard assertions. Instead, use
a data source with a `lifecycle { postcondition { ... } }` block: lifecycle
postconditions fail the `apply`/`plan` hard. See `sns_topic/main.tf`, which
reads the topic back through `data "aws_sns_topic"` and asserts the `tags`
read-back matches what was set.

## Running locally

```bash
# start kumo on :4566 from the repo root
go run ./cmd/kumo --port 4566 &

# run the suite (terraform must be on PATH, or set KUMO_TF_BIN)
make test-terraform

# or target a specific fixture
KUMO_TF_BIN=terraform go test -C test -v -race -tags=integration \
  -run 'TestTerraformFixtures/sqs_queue' ./integration/terraform/...
```

`KUMO_TF_BIN` overrides binary selection: set it to `tofu` or `terraform`
(or an absolute path) to force one over the other. Without it, the runner
prefers `tofu` on PATH, falling back to `terraform`. If neither is found,
the suite is skipped.
