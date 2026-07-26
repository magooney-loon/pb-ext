package alerts

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

const testToken = "123456789:AAHverySecretTokenValue"

// fakeAPI stands in for api.telegram.org. Every transport test runs against it,
// so nothing in this package ever touches the network.
type fakeAPI struct {
	*httptest.Server

	mu       sync.Mutex
	requests []fakeRequest
	status   int
	body     string
}

type fakeRequest struct {
	path    string
	payload sendMessageRequest
}

func newFakeAPI(t testing.TB) *fakeAPI {
	t.Helper()

	api := &fakeAPI{status: http.StatusOK, body: `{"ok":true,"result":{"message_id":1}}`}
	api.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload sendMessageRequest
		_ = json.NewDecoder(r.Body).Decode(&payload)

		api.mu.Lock()
		api.requests = append(api.requests, fakeRequest{path: r.URL.Path, payload: payload})
		status, body := api.status, api.body
		api.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(api.Close)

	return api
}

func (a *fakeAPI) respond(status int, body string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.status, a.body = status, body
}

func (a *fakeAPI) lastRequest(t testing.TB) fakeRequest {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.requests) == 0 {
		t.Fatal("the API received no requests")
	}
	return a.requests[len(a.requests)-1]
}

func newTestTransport(t testing.TB, baseURL string) *telegramTransport {
	t.Helper()
	cfg := DefaultConfig()
	cfg.BotToken = testToken
	cfg.ChatID = "-1001234567890"
	cfg.APIBaseURL = baseURL
	return newTelegramTransport(cfg)
}

func TestTelegramSend_PostsAHTMLMessage(t *testing.T) {
	api := newFakeAPI(t)
	transport := newTestTransport(t, api.URL)

	err := transport.Send(context.Background(), Message{
		Level: LevelError,
		Title: "Cron job failed",
		Text:  "boom",
	}, "prod-1")
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	req := api.lastRequest(t)
	if want := "/bot" + testToken + "/sendMessage"; req.path != want {
		t.Fatalf("path = %q, want %q", req.path, want)
	}
	if req.payload.ParseMode != "HTML" {
		t.Fatalf("parse_mode = %q, want HTML — MarkdownV2 would reject half of what we send", req.payload.ParseMode)
	}
	if !strings.Contains(req.payload.Text, "<b>Cron job failed</b>") {
		t.Fatalf("text = %q, want a bold title", req.payload.Text)
	}
	if !strings.Contains(req.payload.Text, "prod-1") {
		t.Fatalf("text = %q, want the instance label", req.payload.Text)
	}
	if !req.payload.LinkPreviewOptions.IsDisabled {
		t.Fatal("link previews are enabled; error text full of URLs would bury the message")
	}
}

// Info-level alerts arrive silently. A channel that buzzes for "server started"
// gets muted, and a muted channel delivers nothing at all.
func TestTelegramSend_InfoIsDeliveredSilently(t *testing.T) {
	api := newFakeAPI(t)
	transport := newTestTransport(t, api.URL)

	_ = transport.Send(context.Background(), Message{Level: LevelInfo, Title: "Server started"}, "")
	if !api.lastRequest(t).payload.DisableNotification {
		t.Fatal("info alert was not silenced")
	}

	_ = transport.Send(context.Background(), Message{Level: LevelCritical, Title: "Server crashed"}, "")
	if api.lastRequest(t).payload.DisableNotification {
		t.Fatal("critical alert was silenced")
	}
}

func TestTelegramSend_HonoursRetryAfterOn429(t *testing.T) {
	api := newFakeAPI(t)
	api.respond(http.StatusTooManyRequests,
		`{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":7}}`)

	transport := newTestTransport(t, api.URL)

	err := transport.Send(context.Background(), Message{Title: "x"}, "")

	var se *SendError
	if !errors.As(err, &se) {
		t.Fatalf("error = %v, want a *SendError", err)
	}
	if se.Permanent {
		t.Fatal("429 was classified permanent; it is the one 4xx that means later, not never")
	}
	if se.RetryAfter != 7*time.Second {
		t.Fatalf("RetryAfter = %v, want 7s", se.RetryAfter)
	}
}

func TestTelegramSend_ClassifiesErrors(t *testing.T) {
	cases := []struct {
		name      string
		status    int
		body      string
		permanent bool
	}{
		{"bad token", http.StatusUnauthorized, `{"ok":false,"error_code":401,"description":"Unauthorized"}`, true},
		{"bot kicked", http.StatusForbidden, `{"ok":false,"error_code":403,"description":"Forbidden"}`, true},
		{"bad chat id", http.StatusBadRequest, `{"ok":false,"error_code":400,"description":"chat not found"}`, true},
		{"server error", http.StatusBadGateway, `{"ok":false,"error_code":502,"description":"Bad Gateway"}`, false},
		{"service unavailable", http.StatusServiceUnavailable, `{"ok":false,"error_code":503,"description":"try later"}`, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := newFakeAPI(t)
			api.respond(tc.status, tc.body)
			transport := newTestTransport(t, api.URL)

			err := transport.Send(context.Background(), Message{Title: "x"}, "")

			var se *SendError
			if !errors.As(err, &se) {
				t.Fatalf("error = %v, want a *SendError", err)
			}
			if se.Permanent != tc.permanent {
				t.Fatalf("Permanent = %v, want %v (retrying a permanent failure is a hot loop; giving up on a transient one loses the alert)",
					se.Permanent, tc.permanent)
			}
		})
	}
}

// The Bot API takes the token as a path segment, so every transport error
// quotes a URL containing it. This is the test that keeps a token out of logs.
func TestTelegram_NeverLeaksTheTokenInErrors(t *testing.T) {
	t.Run("network failure", func(t *testing.T) {
		// Port 1 refuses connections, so net/http reports the full request URL.
		transport := newTestTransport(t, "http://127.0.0.1:1")

		err := transport.Send(context.Background(), Message{Title: "x"}, "")
		if err == nil {
			t.Fatal("expected a connection error")
		}
		assertNoToken(t, err.Error())
	})

	t.Run("api rejection", func(t *testing.T) {
		api := newFakeAPI(t)
		api.respond(http.StatusUnauthorized,
			`{"ok":false,"error_code":401,"description":"Unauthorized for bot`+testToken+`"}`)
		transport := newTestTransport(t, api.URL)

		err := transport.Send(context.Background(), Message{Title: "x"}, "")
		if err == nil {
			t.Fatal("expected an API error")
		}
		assertNoToken(t, err.Error())
	})

	t.Run("verify failure", func(t *testing.T) {
		transport := newTestTransport(t, "http://127.0.0.1:1")

		err := transport.Verify(context.Background())
		if err == nil {
			t.Fatal("expected a connection error")
		}
		assertNoToken(t, err.Error())
	})
}

// A failed delivery must not smuggle the token into Stats or the delivery log
// either — those reach the dashboard and the database.
func TestNotifier_NeverLeaksTheTokenIntoStats(t *testing.T) {
	n := Initialize(nil,
		WithTelegram(testToken, "-100123"),
		WithAPIBaseURL("http://127.0.0.1:1"),
		WithMaxRetries(0),
		WithMinSendInterval(0),
		WithPersistence(false),
		WithLifecycleAlerts(false),
		WithEvaluateInterval(time.Hour),
	)
	t.Cleanup(func() { _ = n.Close() })

	n.Send(Message{Title: "will fail"})
	waitFor(t, func() bool { return n.Stats().LastError != "" })

	stats := n.Stats()
	assertNoToken(t, stats.LastError)
	assertNoToken(t, stats.Reason)
	assertNoToken(t, stats.Target)
}

func assertNoToken(t testing.TB, s string) {
	t.Helper()
	if strings.Contains(s, testToken) {
		t.Fatalf("the bot token leaked into %q", s)
	}
	if strings.Contains(s, "verySecretTokenValue") {
		t.Fatalf("the token secret leaked into %q", s)
	}
}

func TestTelegramVerify_ReportsTheBotUsername(t *testing.T) {
	api := newFakeAPI(t)
	api.respond(http.StatusOK, `{"ok":true,"result":{"id":1,"username":"pbext_alerts_bot"}}`)
	transport := newTestTransport(t, api.URL)

	if err := transport.Verify(context.Background()); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if got := transport.Target(); !strings.Contains(got, "pbext_alerts_bot") {
		t.Fatalf("Target = %q, want it to name the verified bot", got)
	}
	assertNoToken(t, transport.Target())
}

func TestTelegram_NonJSONResponseIsHandled(t *testing.T) {
	api := newFakeAPI(t)
	// A proxy error page, a captive portal, a wrong base URL.
	api.respond(http.StatusBadGateway, "<html>502 Bad Gateway</html>")
	transport := newTestTransport(t, api.URL)

	err := transport.Send(context.Background(), Message{Title: "x"}, "")
	if err == nil {
		t.Fatal("expected an error for a non-JSON body")
	}
	assertNoToken(t, err.Error())
}
