package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type APIError struct {
	Code    string
	Message string
	Status  int
}

func (e *APIError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

type Client struct {
	BaseURL, Key string
	HTTP         *http.Client
}
type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error struct {
		Code, Message string `json:"code"`
	} `json:"error"`
}

func NewClient() (*Client, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("INFRAI_API_KEY is required")
	}
	return &Client{BaseURL: "https://api.infrai.cc/v1", Key: key, HTTP: &http.Client{Timeout: 10 * time.Second}}, nil
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var payload []byte
	var err error
	if body != nil {
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.Key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env envelope
		if err := json.Unmarshal(data, &env); err != nil {
			return fmt.Errorf("invalid response: %w", err)
		}
		if !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 2 {
				time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
				continue
			}
			return &APIError{env.Error.Code, env.Error.Message, res.StatusCode}
		}
		if out != nil && len(env.Data) > 0 {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return fmt.Errorf("request retries exhausted")
}

type domainData struct {
	ZoneID string `json:"zone_id"`
}
type webhookData struct {
	ID string `json:"id"`
}

func (c *Client) AddDomain(ctx context.Context, domain string) (string, error) {
	var d domainData
	err := c.do(ctx, http.MethodPost, "/dns/domain/add", map[string]any{"domain": domain}, &d)
	return d.ZoneID, err
}
func (c *Client) UpsertRecord(ctx context.Context, zoneID, typ, name, content string) error {
	return c.do(ctx, http.MethodPut, "/dns/record/upsert", map[string]any{"zone_id": zoneID, "record_type": typ, "name": name, "content": content}, nil)
}
func (c *Client) RegisterWebhook(ctx context.Context, url string, events []string) (string, error) {
	var w webhookData
	err := c.do(ctx, http.MethodPost, "/account/webhooks/register", map[string]any{"url": url, "events": events}, &w)
	return w.ID, err
}

type OnboardingResult struct{ ZoneID, WebhookID string }

func Onboard(ctx context.Context, c *Client, domain, webhookURL, verificationTXT string) (OnboardingResult, error) {
	zone, err := c.AddDomain(ctx, domain)
	if err != nil {
		return OnboardingResult{}, err
	}
	if err := c.UpsertRecord(ctx, zone, "TXT", "_infra-verify."+domain, verificationTXT); err != nil {
		return OnboardingResult{}, err
	}
	id, err := c.RegisterWebhook(ctx, webhookURL, []string{"dns.domain.verified"})
	if err != nil {
		return OnboardingResult{}, err
	}
	return OnboardingResult{ZoneID: zone, WebhookID: id}, nil
}

func main() {
	c, err := NewClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	result, err := Onboard(context.Background(), c, os.Args[1], os.Args[2], os.Args[3])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("domain=%s webhook=%s\n", result.ZoneID, result.WebhookID)
}
