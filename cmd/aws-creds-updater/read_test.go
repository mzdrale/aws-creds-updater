package main

import (
	"strings"
	"testing"
)

func TestCredPasteCRLFAndBracketedPaste(t *testing.T) {
	token := "IQoJ" + strings.Repeat("A", 1500) + "5XA=="
	raw := "\x1b[200~[256099319689_NC2-FA-Admin]\r\n" +
		"aws_access_key_id=ASIATH3HLO\r\n" +
		"aws_secret_access_key=3jv3+secretBWcbR\r\n" +
		"aws_session_token=" + token + "\r\n" +
		"\x1b[201~"

	var p credPaste
	if err := p.feed([]byte(raw)); err != nil {
		t.Fatal(err)
	}
	if !hasCompleteCredentials(p.currentLines()) {
		t.Fatalf("expected complete credentials, got %#v", p.currentLines())
	}
	if p.lines[3] != "aws_session_token="+token {
		t.Fatalf("session token line truncated: len=%d", len(p.lines[3]))
	}
}

func TestCredPasteCROnlyDoesNotNeedSecondEnter(t *testing.T) {
	var p credPaste
	if err := p.feed([]byte("[acc]\raws_access_key_id=ASIAA\r")); err != nil {
		t.Fatal(err)
	}
	if len(p.lines) != 2 {
		t.Fatalf("got %d lines, want 2 (Peek-after-CR must not wait): %#v", len(p.lines), p.lines)
	}
}

func TestCredPastePartialLastLine(t *testing.T) {
	var p credPaste
	raw := "[acc]\naws_access_key_id=ASIAA\naws_secret_access_key=secret\naws_session_token=" + strings.Repeat("T", 1200)
	if err := p.feed([]byte(raw)); err != nil {
		t.Fatal(err)
	}
	if !hasCompleteCredentials(p.currentLines()) {
		t.Fatalf("partial last line should still complete the block: %#v", p.currentLines())
	}
	p.finishPartial()
	if got := p.lines[3]; !strings.HasSuffix(got, strings.Repeat("T", 1200)) {
		t.Fatalf("unexpected last line: %s", got)
	}
}
