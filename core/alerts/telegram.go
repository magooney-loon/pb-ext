package alerts

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SendError classifies a delivery failure for the worker.
//
// The distinction matters: retrying a bad token forever is a hot loop against
// someone else's API, and giving up on a 429 throws away an alert that would
// have gone through a second later.
type SendError struct {
	// Permanent means the request will never succeed as configured — a bad
	// token, a wrong chat id, a bot that was kicked from the group.
	Permanent bool
	// RetryAfter is the delay Telegram asked for on a 429.
	RetryAfter time.Duration
	// Err is the underlying cause, already scrubbed of credentials.
	Err error
}

func (e *SendError) Error() string { return e.Err.Error() }
func (e *SendError) Unwrap() error { return e.Err }

// maxResponseBytes caps how much of an API response is read. Telegram's replies
// are small; an unbounded read would let a misconfigured base URL (a proxy
// serving something else entirely) pull an arbitrary body into memory.
const maxResponseBytes = 64 << 10

// telegramTransport talks to the Bot API.
type telegramTransport struct {
	token   string
	chatID  string
	topicID int
	baseURL string
	client  *http.Client

	// mu guards username, which Verify fills in from a background goroutine
	// while the dashboard may be reading Target concurrently.
	mu       sync.Mutex
	username string
}

func newTelegramTransport(cfg Config) *telegramTransport {
	return &telegramTransport{
		token:   cfg.BotToken,
		chatID:  cfg.ChatID,
		topicID: cfg.TopicID,
		baseURL: strings.TrimSuffix(cfg.APIBaseURL, "/"),
		client:  &http.Client{Timeout: cfg.SendTimeout},
	}
}

func (t *telegramTransport) Name() string { return "telegram" }

// Target describes the destination without disclosing the token.
func (t *telegramTransport) Target() string {
	t.mu.Lock()
	name := t.username
	t.mu.Unlock()

	target := "chat " + t.chatID
	if t.topicID != 0 {
		target += " topic " + strconv.Itoa(t.topicID)
	}
	if name != "" {
		target = "@" + name + " → " + target
	}
	return target
}

// sendMessageRequest is the Bot API sendMessage payload.
type sendMessageRequest struct {
	ChatID          string `json:"chat_id"`
	Text            string `json:"text"`
	ParseMode       string `json:"parse_mode"`
	MessageThreadID int    `json:"message_thread_id,omitempty"`
	// Info-level alerts arrive without a sound. A dashboard that buzzes a phone
	// for "server started" gets muted, and a muted channel delivers nothing.
	DisableNotification bool `json:"disable_notification,omitempty"`
	// Error text is full of URLs; without this every one of them renders a
	// link preview card and buries the message.
	LinkPreviewOptions linkPreviewOptions `json:"link_preview_options"`
}

type linkPreviewOptions struct {
	IsDisabled bool `json:"is_disabled"`
}

// apiResponse is the envelope every Bot API method returns.
type apiResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
	ErrorCode   int    `json:"error_code"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
	Result json.RawMessage `json:"result"`
}

// Send delivers one rendered message.
func (t *telegramTransport) Send(ctx context.Context, m Message, instance string) error {
	payload := sendMessageRequest{
		ChatID:              t.chatID,
		Text:                renderMessage(m, instance),
		ParseMode:           "HTML",
		MessageThreadID:     t.topicID,
		DisableNotification: m.Level == LevelInfo,
		LinkPreviewOptions:  linkPreviewOptions{IsDisabled: true},
	}

	_, err := t.call(ctx, "sendMessage", payload)
	return err
}

// Verify confirms the token with getMe. It is called asynchronously at startup:
// learning that the token is wrong at boot beats learning it during the first
// incident, but it must never block or fail startup.
func (t *telegramTransport) Verify(ctx context.Context) error {
	raw, err := t.call(ctx, "getMe", nil)
	if err != nil {
		return err
	}

	var me struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(raw, &me); err == nil && me.Username != "" {
		t.mu.Lock()
		t.username = me.Username
		t.mu.Unlock()
	}
	return nil
}

// call performs one Bot API request and classifies the outcome.
func (t *telegramTransport) call(ctx context.Context, method string, payload any) (json.RawMessage, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			// Nothing about a marshalling bug improves on a retry.
			return nil, &SendError{Permanent: true, Err: fmt.Errorf("encode %s payload: %w", method, err)}
		}
		body = bytes.NewReader(encoded)
	}

	// The token is a path segment, which is why every error below is scrubbed
	// before it escapes this function.
	url := t.baseURL + "/bot" + t.token + "/" + method

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, t.wrap(false, 0, fmt.Errorf("build %s request: %w", method, err))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		// DNS failures, timeouts, connection resets: all worth retrying, and all
		// carrying the request URL — and therefore the token — in their text.
		return nil, t.wrap(false, 0, fmt.Errorf("%s request failed: %w", method, err))
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, t.wrap(false, 0, fmt.Errorf("read %s response: %w", method, err))
	}

	var parsed apiResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		// A non-JSON body means we are not talking to the Bot API — a captive
		// portal, a proxy error page, a wrong base URL. Retrying is harmless and
		// the status code still decides.
		return nil, t.wrap(resp.StatusCode == http.StatusBadRequest, 0,
			fmt.Errorf("%s returned %d with an unreadable body", method, resp.StatusCode))
	}

	if parsed.OK {
		return parsed.Result, nil
	}

	code := parsed.ErrorCode
	if code == 0 {
		code = resp.StatusCode
	}

	retryAfter := time.Duration(parsed.Parameters.RetryAfter) * time.Second
	desc := parsed.Description
	if desc == "" {
		desc = http.StatusText(code)
	}

	return nil, t.wrap(isPermanentCode(code), retryAfter,
		fmt.Errorf("%s failed: %d %s", method, code, desc))
}

// wrap scrubs an error of credentials and tags it for the retry logic.
func (t *telegramTransport) wrap(permanent bool, retryAfter time.Duration, err error) *SendError {
	return &SendError{
		Permanent:  permanent,
		RetryAfter: retryAfter,
		Err:        fmt.Errorf("%s", scrub(err.Error(), t.token)),
	}
}

// isPermanentCode reports whether an API error code will still fail on a retry.
//
// 429 is deliberately absent: it is the one 4xx that means "later, not never".
func isPermanentCode(code int) bool {
	switch code {
	case http.StatusBadRequest, // malformed chat id, unparseable HTML
		http.StatusUnauthorized, // bad or revoked token
		http.StatusForbidden,    // bot blocked, kicked, or never started
		http.StatusNotFound:     // wrong base URL or method
		return true
	}
	return false
}
