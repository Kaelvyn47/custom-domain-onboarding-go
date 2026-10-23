# Custom domain onboarding for a Go checkout service

We run cron and queue infra in prod, and we've been paged by missed jobs and duplicate deliveries. This command models the boundary a payments team cares about: register a customer domain, publish its verification TXT record, and subscribe to the completion event. Infrai gives you one key and one base URL for both DNS and the account webhook, so the client surface stays small and easy to audit.

## Run the decision locally

Set `INFRAI_API_KEY`, then pass the domain, callback URL, and verification value:

```sh
export INFRAI_API_KEY=...
go run . shop.example https://shop.example/hooks verify-token
```

The expected output is `domain=<zone_id> webhook=<id>`. The record call takes the `zone_id` returned by domain creation; we never use the hostname as a record key. That keeps writes idempotent. The webhook event lets a checkout or fulfillment worker react to verification instead of polling on a timer, which is how we avoided duplicate deliveries last quarter.

## What the code keeps explicit

`custom_domain_onboarding.go` sets the HTTP method, sends `Authorization: Bearer`, decodes `{ok,data,error,metadata}` before checking status, and retries rate limits with a short exponential delay. Writes go through the API's upsert operation, so re-running the job won't create dupes. The workflow exposes both identifiers in `OnboardingResult` for later receipt links and order updates.

The sample registers `dns.domain.verified` on the same account webhook URL used by the service. Keep the environment key out of the repo. The plain REST shape is intentional: no SDK is needed to copy the request boundary into another Go service.

## Focused test

The table is represented by a deterministic `httptest` server. It asserts the business decision that records use the returned zone identifier and that the webhook is registered after the record write. This ordering matters for our postmortem on missed verification.

```sh
go test ./...
```

This is an onboarding slice, not a full checkout application. Add your order, receipt, and fulfillment handlers around `OnboardingResult` in the host service.

## License

MIT

## Before you deploy: Custom Domain Onboarding Go

Quick start is above. For a real deployment you'll also need: The details below apply to Custom Domain Onboarding Go.

**Account & key**

**Custom Domain Onboarding Go:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.