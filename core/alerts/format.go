package alerts

import (
	"regexp"
	"sort"
	"strings"
)

// Messages are rendered with Telegram's HTML parse mode rather than MarkdownV2.
//
// MarkdownV2 requires escaping every one of _*[]()~`>#+-=|{}.! — and an alert
// body is made of exactly those characters: file paths, stack traces, Go error
// strings, durations like "1.2s". A single missed escape is a 400 from the API,
// so the message that silently fails to send is precisely the interesting one.
// HTML needs three characters escaped and supports the four tags used here.
const (
	// renderBudget leaves room under MessageLimit for the tags and the emoji,
	// so the escaped output never lands on the wrong side of the cap.
	renderBudget = MessageLimit - 256

	// truncationMarker is appended to a body that did not fit.
	truncationMarker = "\n… (truncated)"
)

// escapeHTML escapes the three characters Telegram's HTML parse mode reserves.
// The ampersand must go first, or the escapes introduced for < and > would be
// escaped again.
func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// renderMessage produces the final HTML body for a message, guaranteed to fit
// inside Telegram's length cap.
//
// The header (emoji, title, fields, instance label) is rendered first and the
// remaining budget goes to the body, so a 40KB stack trace can never push the
// title itself out of the message.
func renderMessage(m Message, instance string) string {
	var head strings.Builder

	head.WriteString(m.Level.Emoji())
	head.WriteString(" <b>")
	head.WriteString(escapeHTML(strings.TrimSpace(m.Title)))
	head.WriteString("</b>")

	if len(m.Fields) > 0 {
		keys := make([]string, 0, len(m.Fields))
		for k := range m.Fields {
			keys = append(keys, k)
		}
		// Sorted so the same alert always looks the same — map order would
		// otherwise reshuffle the fields on every send.
		sort.Strings(keys)

		head.WriteString("\n")
		for _, k := range keys {
			head.WriteString("\n<b>")
			head.WriteString(escapeHTML(k))
			head.WriteString(":</b> ")
			head.WriteString(escapeHTML(m.Fields[k]))
		}
	}

	var foot string
	if instance != "" {
		foot = "\n\n<i>" + escapeHTML(instance) + "</i>"
	}

	body := strings.TrimSpace(m.Text)
	if body == "" {
		return head.String() + foot
	}

	budget := renderBudget - utf16Len(head.String()) - utf16Len(foot)
	if budget < 64 {
		// Pathological title/field set: drop the body rather than the header.
		return head.String() + foot
	}

	return head.String() + "\n\n" + fitBody(body, m.Monospace, budget) + foot
}

// fitBody escapes and optionally wraps a body, shrinking the raw text until the
// rendered result fits the budget.
//
// Shrinking is iterative because escaping expands: a body of ampersands grows
// fivefold, so a raw-length check would under-count and still overshoot. Each
// pass removes at least the overflow, so this converges in a couple of rounds.
func fitBody(raw string, monospace bool, budget int) string {
	wrap := func(s string) string {
		if monospace {
			return "<pre>" + escapeHTML(s) + "</pre>"
		}
		return escapeHTML(s)
	}

	out := wrap(raw)
	if utf16Len(out) <= budget {
		return out
	}

	runes := []rune(raw)
	marker := utf16Len(truncationMarker)

	for len(runes) > 0 {
		over := utf16Len(out) - budget + marker
		if over <= 0 {
			break
		}
		// Trim by at least the overflow, plus a small margin so a body of
		// wide/escaping characters converges instead of inching down.
		cut := over + over/4 + 8
		if cut >= len(runes) {
			runes = nil
		} else {
			runes = runes[:len(runes)-cut]
		}
		out = wrap(string(runes) + truncationMarker)
		if utf16Len(out) <= budget {
			return out
		}
	}

	return wrap(truncationMarker)
}

// utf16Len counts UTF-16 code units, which is the unit Telegram's 4096-character
// limit is expressed in. Runes outside the BMP — emoji, most notably, and this
// package prefixes every message with one — cost two units, so counting runes
// or bytes would misjudge the limit in opposite directions.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r > 0xFFFF {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// tokenPattern matches a bot token wherever it appears, including inside the
// API URLs that net/http embeds in its error strings.
var tokenPattern = regexp.MustCompile(`bot\d+:[A-Za-z0-9_-]+`)

// scrub removes credentials from a string bound for a log, an error, an API
// response or the delivery log.
//
// The Bot API takes the token as a *path segment*, so any error from the HTTP
// client — a DNS failure, a timeout, a redirect loop — quotes the URL and with
// it the token. Every error leaving telegram.go goes through here; that is the
// single reason a token cannot leak into logs.
func scrub(s, token string) string {
	if token != "" {
		s = strings.ReplaceAll(s, token, "<token>")
	}
	return tokenPattern.ReplaceAllString(s, "bot<token>")
}

// redactToken renders a token for display: enough to tell two apart, useless to
// anyone who reads it. Telegram tokens are "<bot id>:<secret>".
func redactToken(token string) string {
	if token == "" {
		return ""
	}

	id, secret, ok := strings.Cut(token, ":")
	if !ok || id == "" {
		return "…"
	}
	if len(secret) > 3 {
		secret = secret[:3]
	}
	return id + ":" + secret + "…"
}
