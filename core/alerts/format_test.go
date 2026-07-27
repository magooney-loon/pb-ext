package alerts

import (
	"strings"
	"testing"
)

func TestEscapeHTML_EscapesAmpersandFirst(t *testing.T) {
	// Escaping < before & would turn "<" into "&amp;lt;".
	got := escapeHTML(`a & b < c > d`)
	if want := `a &amp; b &lt; c &gt; d`; got != want {
		t.Fatalf("escapeHTML = %q, want %q", got, want)
	}
}

func TestRenderMessage_EscapesEveryField(t *testing.T) {
	got := renderMessage(Message{
		Level:  LevelError,
		Title:  `handler <script>alert(1)</script>`,
		Text:   `err: a & b`,
		Fields: map[string]string{"path": "/x?a=1&b=2"},
	}, "prod & staging")

	if strings.Contains(got, "<script>") {
		t.Fatalf("an unescaped tag reached the message: %q", got)
	}
	for _, want := range []string{"&lt;script&gt;", "a &amp; b", "a=1&amp;b=2", "prod &amp; staging"} {
		if !strings.Contains(got, want) {
			t.Fatalf("rendered message %q is missing %q", got, want)
		}
	}
}

// Map iteration order is random; the same alert must not reshuffle its fields
// on every send.
func TestRenderMessage_SortsFields(t *testing.T) {
	fields := map[string]string{"zebra": "1", "alpha": "2", "middle": "3"}

	first := renderMessage(Message{Title: "t", Fields: fields}, "")
	for range 10 {
		if got := renderMessage(Message{Title: "t", Fields: fields}, ""); got != first {
			t.Fatal("field order is not stable across renders")
		}
	}

	alpha := strings.Index(first, "alpha")
	middle := strings.Index(first, "middle")
	zebra := strings.Index(first, "zebra")
	if !(alpha < middle && middle < zebra) {
		t.Fatalf("fields are not sorted: %q", first)
	}
}

func TestRenderMessage_TruncatesToTelegramsLimit(t *testing.T) {
	cases := map[string]string{
		"plain":      strings.Repeat("stack frame line\n", 5000),
		"escaping":   strings.Repeat("&<>", 5000),
		"multibyte":  strings.Repeat("日本語のテキスト", 2000),
		"emoji":      strings.Repeat("🚨", 5000),
		"whitespace": strings.Repeat(" ", 20000) + "tail",
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			got := renderMessage(Message{
				Level:     LevelError,
				Title:     "Panic recovered",
				Text:      body,
				Monospace: true,
			}, "host")

			if n := utf16Len(got); n > MessageLimit {
				t.Fatalf("rendered %d UTF-16 units, Telegram rejects anything over %d", n, MessageLimit)
			}
			if !strings.Contains(got, "Panic recovered") {
				t.Fatal("the title was truncated away; the body must yield first")
			}
			if strings.Count(got, "<pre>") != strings.Count(got, "</pre>") {
				t.Fatalf("truncation split the <pre> wrapper: %q", got[max(0, len(got)-80):])
			}
		})
	}
}

func TestRenderMessage_KeepsShortBodiesIntact(t *testing.T) {
	got := renderMessage(Message{Title: "t", Text: "short body"}, "")
	if strings.Contains(got, "truncated") {
		t.Fatalf("a short body was truncated: %q", got)
	}
}

// Emoji sit outside the BMP and cost two UTF-16 units each. Counting runes or
// bytes would misjudge the limit in opposite directions, and every message this
// package sends starts with an emoji.
func TestUTF16Len(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"abc", 3},
		{"日本", 2},
		{"🚨", 2},
		{"🚨 ok", 5},
	}

	for _, tc := range cases {
		if got := utf16Len(tc.in); got != tc.want {
			t.Fatalf("utf16Len(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestScrub_RemovesTokensInEveryForm(t *testing.T) {
	token := "123456789:AAHverySecret"

	cases := []string{
		"Post \"https://api.telegram.org/bot" + token + "/sendMessage\": dial tcp: refused",
		"unauthorized for " + token,
		"see bot987654321:OTHERtokenValue for details",
	}

	for _, in := range cases {
		got := scrub(in, token)
		if strings.Contains(got, "verySecret") || strings.Contains(got, "OTHERtokenValue") {
			t.Fatalf("scrub(%q) left a credential: %q", in, got)
		}
	}
}

func TestRedactToken_KeepsEnoughToTellTwoApart(t *testing.T) {
	got := redactToken("123456789:AAHverySecretValue")

	if strings.Contains(got, "verySecret") {
		t.Fatalf("redactToken leaked the secret: %q", got)
	}
	if !strings.HasPrefix(got, "123456789:") {
		t.Fatalf("redactToken = %q, want the bot id preserved", got)
	}
	if redactToken("") != "" {
		t.Fatal("redactToken on an empty token should stay empty")
	}
	if got := redactToken("garbage"); got != "…" {
		t.Fatalf("redactToken(garbage) = %q, want a placeholder", got)
	}
}
