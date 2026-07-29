package tools

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	gmail "google.golang.org/api/gmail/v1"
)

// --- formatSearchResults ---

func TestGmailFormatSearchResults(t *testing.T) {
	tests := []struct {
		name          string
		messages      []*gmail.Message
		query         string
		nextPageToken string
		wantContains  []string
		wantExact     string // only checked if non-empty
	}{
		{
			name:      "no messages",
			messages:  []*gmail.Message{},
			query:     "from:test@example.com",
			wantExact: "No messages found for query: 'from:test@example.com'",
		},
		{
			name: "single message",
			messages: []*gmail.Message{
				{Id: "msg1", ThreadId: "thread1"},
			},
			query: "is:unread",
			wantContains: []string{
				"Found 1 messages for query: 'is:unread'",
				"1. Message ID: msg1",
				"Thread ID: thread1",
				"https://mail.google.com/mail/u/0/#inbox/msg1",
			},
		},
		{
			name: "multiple messages",
			messages: []*gmail.Message{
				{Id: "msg1", ThreadId: "thread1"},
				{Id: "msg2", ThreadId: "thread2"},
			},
			query: "test",
			wantContains: []string{
				"Found 2 messages",
				"1. Message ID: msg1",
				"2. Message ID: msg2",
			},
		},
		{
			name: "with next page token",
			messages: []*gmail.Message{
				{Id: "msg1", ThreadId: "thread1"},
			},
			query:         "label:inbox",
			nextPageToken: "token123",
			wantContains: []string{
				"More results available",
				"page_token: 'token123'",
			},
		},
		{
			name: "without next page token",
			messages: []*gmail.Message{
				{Id: "msg1", ThreadId: "thread1"},
			},
			query:         "test",
			nextPageToken: "",
			wantContains: []string{
				"Found 1 messages",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSearchResults(tt.messages, tt.query, tt.nextPageToken)
			if tt.wantExact != "" {
				if got != tt.wantExact {
					t.Errorf("got %q, want %q", got, tt.wantExact)
				}
				return
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q\ngot: %s", want, got)
				}
			}
			// Verify no page token text when not provided
			if tt.nextPageToken == "" && strings.Contains(got, "More results available") {
				t.Error("unexpected 'More results available' when no page token")
			}
		})
	}
}

// --- extractHeaders ---

func TestGmailExtractHeaders(t *testing.T) {
	tests := []struct {
		name        string
		payload     *gmail.MessagePart
		headerNames []string
		want        map[string]string
	}{
		{
			name:        "nil payload",
			payload:     nil,
			headerNames: []string{"Subject"},
			want:        map[string]string{},
		},
		{
			name: "all headers present",
			payload: &gmail.MessagePart{
				Headers: []*gmail.MessagePartHeader{
					{Name: "Subject", Value: "Test Subject"},
					{Name: "From", Value: "sender@example.com"},
					{Name: "To", Value: "recipient@example.com"},
					{Name: "Date", Value: "Mon, 1 Jan 2024 00:00:00 +0000"},
				},
			},
			headerNames: []string{"Subject", "From", "To", "Date"},
			want: map[string]string{
				"Subject": "Test Subject",
				"From":    "sender@example.com",
				"To":      "recipient@example.com",
				"Date":    "Mon, 1 Jan 2024 00:00:00 +0000",
			},
		},
		{
			name: "partial headers",
			payload: &gmail.MessagePart{
				Headers: []*gmail.MessagePartHeader{
					{Name: "Subject", Value: "Test"},
				},
			},
			headerNames: []string{"Subject", "From"},
			want: map[string]string{
				"Subject": "Test",
			},
		},
		{
			name: "extra headers ignored",
			payload: &gmail.MessagePart{
				Headers: []*gmail.MessagePartHeader{
					{Name: "Subject", Value: "Test"},
					{Name: "X-Custom", Value: "custom"},
				},
			},
			headerNames: []string{"Subject"},
			want: map[string]string{
				"Subject": "Test",
			},
		},
		{
			name: "empty payload headers",
			payload: &gmail.MessagePart{
				Headers: []*gmail.MessagePartHeader{},
			},
			headerNames: []string{"Subject"},
			want:        map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractHeaders(tt.payload, tt.headerNames)
			if len(got) != len(tt.want) {
				t.Errorf("got %d headers, want %d: %v", len(got), len(tt.want), got)
			}
			for k, wantV := range tt.want {
				if gotV, ok := got[k]; !ok || gotV != wantV {
					t.Errorf("header %q = %q, want %q", k, gotV, wantV)
				}
			}
		})
	}
}

// --- headerOrDefault ---

func TestGmailHeaderOrDefault(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		key     string
		def     string
		want    string
	}{
		{
			name:    "key present",
			headers: map[string]string{"Subject": "Hello"},
			key:     "Subject",
			def:     "(no subject)",
			want:    "Hello",
		},
		{
			name:    "key absent",
			headers: map[string]string{},
			key:     "Subject",
			def:     "(no subject)",
			want:    "(no subject)",
		},
		{
			name:    "key present but empty",
			headers: map[string]string{"Subject": ""},
			key:     "Subject",
			def:     "(no subject)",
			want:    "(no subject)",
		},
		{
			name:    "nil map",
			headers: nil,
			key:     "Subject",
			def:     "default",
			want:    "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := headerOrDefault(tt.headers, tt.key, tt.def)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- extractAttachments ---

func TestGmailExtractAttachments(t *testing.T) {
	tests := []struct {
		name    string
		payload *gmail.MessagePart
		want    int
		check   func(t *testing.T, atts []attachmentMeta)
	}{
		{
			name:    "nil payload",
			payload: nil,
			want:    0,
		},
		{
			name: "no attachments",
			payload: &gmail.MessagePart{
				MimeType: "text/plain",
				Body:     &gmail.MessagePartBody{Data: "aGVsbG8="},
			},
			want: 0,
		},
		{
			name: "single attachment",
			payload: &gmail.MessagePart{
				Parts: []*gmail.MessagePart{
					{
						MimeType: "text/plain",
						Body:     &gmail.MessagePartBody{Data: "aGVsbG8="},
					},
					{
						Filename: "doc.pdf",
						MimeType: "application/pdf",
						Body:     &gmail.MessagePartBody{AttachmentId: "att1", Size: 2048},
					},
				},
			},
			want: 1,
			check: func(t *testing.T, atts []attachmentMeta) {
				if atts[0].filename != "doc.pdf" {
					t.Errorf("filename = %q, want %q", atts[0].filename, "doc.pdf")
				}
				if atts[0].mimeType != "application/pdf" {
					t.Errorf("mimeType = %q, want %q", atts[0].mimeType, "application/pdf")
				}
				if atts[0].size != 2048 {
					t.Errorf("size = %d, want %d", atts[0].size, 2048)
				}
				if atts[0].attachmentID != "att1" {
					t.Errorf("attachmentID = %q, want %q", atts[0].attachmentID, "att1")
				}
			},
		},
		{
			name: "nested attachments",
			payload: &gmail.MessagePart{
				Parts: []*gmail.MessagePart{
					{
						MimeType: "multipart/alternative",
						Parts: []*gmail.MessagePart{
							{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: "aGk="}},
							{MimeType: "text/html", Body: &gmail.MessagePartBody{Data: "PGI+aGk8L2I+"}},
						},
					},
					{
						Filename: "image.png",
						MimeType: "image/png",
						Body:     &gmail.MessagePartBody{AttachmentId: "att2", Size: 4096},
					},
				},
			},
			want: 1,
		},
		{
			name: "filename present but no attachment ID",
			payload: &gmail.MessagePart{
				Parts: []*gmail.MessagePart{
					{
						Filename: "inline.jpg",
						MimeType: "image/jpeg",
						Body:     &gmail.MessagePartBody{Size: 1024},
					},
				},
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAttachments(tt.payload)
			if len(got) != tt.want {
				t.Errorf("got %d attachments, want %d", len(got), tt.want)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

// --- extractMessageBodies ---

func TestGmailExtractMessageBodies(t *testing.T) {
	encode := func(s string) string {
		return base64.URLEncoding.EncodeToString([]byte(s))
	}

	tests := []struct {
		name     string
		payload  *gmail.MessagePart
		wantText string
		wantHTML string
	}{
		{
			name:     "nil payload",
			payload:  nil,
			wantText: "",
			wantHTML: "",
		},
		{
			name: "text/plain only",
			payload: &gmail.MessagePart{
				MimeType: "text/plain",
				Body:     &gmail.MessagePartBody{Data: encode("Hello, world!")},
			},
			wantText: "Hello, world!",
			wantHTML: "",
		},
		{
			name: "text/html only",
			payload: &gmail.MessagePart{
				MimeType: "text/html",
				Body:     &gmail.MessagePartBody{Data: encode("<b>Hello</b>")},
			},
			wantText: "",
			wantHTML: "<b>Hello</b>",
		},
		{
			name: "multipart with both",
			payload: &gmail.MessagePart{
				MimeType: "multipart/alternative",
				Parts: []*gmail.MessagePart{
					{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: encode("plain text")}},
					{MimeType: "text/html", Body: &gmail.MessagePartBody{Data: encode("<p>html</p>")}},
				},
			},
			wantText: "plain text",
			wantHTML: "<p>html</p>",
		},
		{
			name: "empty body data",
			payload: &gmail.MessagePart{
				MimeType: "text/plain",
				Body:     &gmail.MessagePartBody{Data: ""},
			},
			wantText: "",
			wantHTML: "",
		},
		{
			name: "nil body",
			payload: &gmail.MessagePart{
				MimeType: "text/plain",
				Body:     nil,
			},
			wantText: "",
			wantHTML: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotText, gotHTML := extractMessageBodies(tt.payload)
			if gotText != tt.wantText {
				t.Errorf("text = %q, want %q", gotText, tt.wantText)
			}
			if gotHTML != tt.wantHTML {
				t.Errorf("html = %q, want %q", gotHTML, tt.wantHTML)
			}
		})
	}
}

// --- formatBodyContent ---

func TestGmailFormatBodyContent(t *testing.T) {
	tests := []struct {
		name     string
		textBody string
		htmlBody string
		want     string
	}{
		{
			name:     "text body present",
			textBody: "Hello, world!",
			htmlBody: "<b>Hello</b>",
			want:     "Hello, world!",
		},
		{
			name:     "text body empty, HTML present",
			textBody: "",
			htmlBody: "<b>Hello</b>",
			want:     "[HTML body - plain text not available]\n<b>Hello</b>",
		},
		{
			name:     "both empty",
			textBody: "",
			htmlBody: "",
			want:     "",
		},
		{
			name:     "text body present, HTML empty",
			textBody: "Hello",
			htmlBody: "",
			want:     "Hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBodyContent(tt.textBody, tt.htmlBody)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- decodeBase64URL ---

func TestGmailDecodeBase64URL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "standard base64url with padding",
			input: base64.URLEncoding.EncodeToString([]byte("Hello, World!")),
			want:  "Hello, World!",
		},
		{
			name:  "base64url without padding",
			input: base64.RawURLEncoding.EncodeToString([]byte("Hello, World!")),
			want:  "Hello, World!",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "invalid base64 returns raw",
			input: "!!!not-valid-base64!!!",
			want:  "!!!not-valid-base64!!!",
		},
		{
			name:  "unicode content",
			input: base64.URLEncoding.EncodeToString([]byte("Héllo wörld")),
			want:  "Héllo wörld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decodeBase64URL(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- formatMessageContent ---

func TestGmailFormatMessageContent(t *testing.T) {
	tests := []struct {
		name         string
		messageID    string
		headers      map[string]string
		bodyData     string
		attachments  []attachmentMeta
		wantContains []string
	}{
		{
			name:      "basic message",
			messageID: "msg123",
			headers: map[string]string{
				"Subject": "Test Subject",
				"From":    "sender@example.com",
				"Date":    "2024-01-01",
				"To":      "recipient@example.com",
			},
			bodyData:    "Hello body",
			attachments: nil,
			wantContains: []string{
				"Subject: Test Subject",
				"From:    sender@example.com",
				"Date:    2024-01-01",
				"To:      recipient@example.com",
				"--- BODY ---",
				"Hello body",
			},
		},
		{
			name:      "missing headers use defaults",
			messageID: "msg456",
			headers:   map[string]string{},
			bodyData:  "body",
			wantContains: []string{
				"Subject: (no subject)",
				"From:    (unknown sender)",
				"Date:    (unknown date)",
			},
		},
		{
			name:      "empty body shows placeholder",
			messageID: "msg789",
			headers:   map[string]string{"Subject": "Test"},
			bodyData:  "",
			wantContains: []string{
				"[No text/plain body found]",
			},
		},
		{
			name:      "with attachments",
			messageID: "msg101",
			headers:   map[string]string{"Subject": "With Attachment"},
			bodyData:  "body",
			attachments: []attachmentMeta{
				{filename: "doc.pdf", mimeType: "application/pdf", size: 10240, attachmentID: "att1"},
			},
			wantContains: []string{
				"--- ATTACHMENTS ---",
				"1. doc.pdf (application/pdf, 10.0 KB)",
				"Attachment ID: att1",
				"gmail_get_attachment(message_id='msg101', attachment_id='att1')",
			},
		},
		{
			name:      "with Cc and Message-ID headers",
			messageID: "msg202",
			headers: map[string]string{
				"Subject":    "Test",
				"From":       "a@b.com",
				"Date":       "2024-01-01",
				"Cc":         "cc@example.com",
				"Message-ID": "<msg@example.com>",
			},
			bodyData: "body",
			wantContains: []string{
				"Cc:      cc@example.com",
				"Message-ID: <msg@example.com>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatMessageContent(tt.messageID, tt.headers, tt.bodyData, tt.attachments)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q\ngot: %s", want, got)
				}
			}
		})
	}
}

// --- formatBatchMessage ---

func TestGmailFormatBatchMessage(t *testing.T) {
	encode := func(s string) string {
		return base64.URLEncoding.EncodeToString([]byte(s))
	}

	tests := []struct {
		name         string
		mid          string
		msg          *gmail.Message
		format       string
		wantContains []string
	}{
		{
			name: "metadata format",
			mid:  "msg1",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{
					Headers: []*gmail.MessagePartHeader{
						{Name: "Subject", Value: "Test"},
						{Name: "From", Value: "sender@test.com"},
						{Name: "Date", Value: "2024-01-01"},
					},
				},
			},
			format: "metadata",
			wantContains: []string{
				"Message ID: msg1",
				"Subject: Test",
				"From: sender@test.com",
				"Web Link: https://mail.google.com/mail/u/0/#inbox/msg1",
			},
		},
		{
			name: "full format with body",
			mid:  "msg2",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{
					Headers: []*gmail.MessagePartHeader{
						{Name: "Subject", Value: "Full message"},
						{Name: "From", Value: "sender@test.com"},
						{Name: "Date", Value: "2024-01-01"},
					},
					Parts: []*gmail.MessagePart{
						{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: encode("Hello body")}},
					},
				},
			},
			format: "full",
			wantContains: []string{
				"Message ID: msg2",
				"Subject: Full message",
				"Hello body",
			},
		},
		{
			name: "full format without body",
			mid:  "msg3",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{
					Headers: []*gmail.MessagePartHeader{
						{Name: "Subject", Value: "No body"},
					},
				},
			},
			format: "full",
			wantContains: []string{
				"Message ID: msg3",
				"Subject: No body",
			},
		},
		{
			name: "with To and Cc headers",
			mid:  "msg4",
			msg: &gmail.Message{
				Payload: &gmail.MessagePart{
					Headers: []*gmail.MessagePartHeader{
						{Name: "Subject", Value: "Test"},
						{Name: "From", Value: "a@b.com"},
						{Name: "Date", Value: "2024-01-01"},
						{Name: "To", Value: "to@b.com"},
						{Name: "Cc", Value: "cc@b.com"},
						{Name: "Message-ID", Value: "<id@b.com>"},
					},
				},
			},
			format: "metadata",
			wantContains: []string{
				"To: to@b.com",
				"Cc: cc@b.com",
				"Message-ID: <id@b.com>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatBatchMessage(tt.mid, tt.msg, tt.format)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q\ngot: %s", want, got)
				}
			}
		})
	}
}

// --- formatThreadContent ---

func TestGmailFormatThreadContent(t *testing.T) {
	encode := func(s string) string {
		return base64.URLEncoding.EncodeToString([]byte(s))
	}

	tests := []struct {
		name         string
		thread       *gmail.Thread
		threadID     string
		wantContains []string
	}{
		{
			name:     "empty thread",
			thread:   &gmail.Thread{Messages: []*gmail.Message{}},
			threadID: "thread1",
			wantContains: []string{
				"Thread ID: thread1",
				"Messages in thread: 0",
			},
		},
		{
			name: "single message thread",
			thread: &gmail.Thread{
				Messages: []*gmail.Message{
					{
						Id: "msg1",
						Payload: &gmail.MessagePart{
							Headers: []*gmail.MessagePartHeader{
								{Name: "Subject", Value: "Thread Subject"},
								{Name: "From", Value: "sender@test.com"},
								{Name: "Date", Value: "2024-01-01"},
							},
							Parts: []*gmail.MessagePart{
								{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: encode("First message")}},
							},
						},
					},
				},
			},
			threadID: "thread1",
			wantContains: []string{
				"Thread ID: thread1",
				"Messages in thread: 1",
				"--- Message 1 of 1 ---",
				"Message ID: msg1",
				"Subject: Thread Subject",
				"First message",
			},
		},
		{
			name: "multi-message thread",
			thread: &gmail.Thread{
				Messages: []*gmail.Message{
					{
						Id: "msg1",
						Payload: &gmail.MessagePart{
							Headers: []*gmail.MessagePartHeader{
								{Name: "Subject", Value: "First"},
								{Name: "From", Value: "a@b.com"},
								{Name: "Date", Value: "2024-01-01"},
							},
							Parts: []*gmail.MessagePart{
								{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: encode("Hello")}},
							},
						},
					},
					{
						Id: "msg2",
						Payload: &gmail.MessagePart{
							Headers: []*gmail.MessagePartHeader{
								{Name: "Subject", Value: "Re: First"},
								{Name: "From", Value: "c@d.com"},
								{Name: "Date", Value: "2024-01-02"},
							},
							Parts: []*gmail.MessagePart{
								{MimeType: "text/plain", Body: &gmail.MessagePartBody{Data: encode("Reply")}},
							},
						},
					},
				},
			},
			threadID: "thread2",
			wantContains: []string{
				"Messages in thread: 2",
				"--- Message 1 of 2 ---",
				"--- Message 2 of 2 ---",
				"Message ID: msg1",
				"Message ID: msg2",
			},
		},
		{
			name: "message with no body shows placeholder",
			thread: &gmail.Thread{
				Messages: []*gmail.Message{
					{
						Id: "msg1",
						Payload: &gmail.MessagePart{
							Headers: []*gmail.MessagePartHeader{
								{Name: "Subject", Value: "Empty"},
							},
						},
					},
				},
			},
			threadID: "thread3",
			wantContains: []string{
				"[No body content]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatThreadContent(tt.thread, tt.threadID)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q\ngot: %s", want, got)
				}
			}
		})
	}
}

// --- buildRawMessage ---

func TestGmailBuildRawMessage(t *testing.T) {
	decodeRaw := func(t *testing.T, raw string) string {
		t.Helper()
		data, err := base64.URLEncoding.DecodeString(raw)
		if err != nil {
			t.Fatalf("failed to decode raw message: %v", err)
		}
		return string(data)
	}

	tests := []struct {
		name         string
		from         string
		fromName     string
		to           string
		cc           string
		bcc          string
		subject      string
		body         string
		bodyFormat   string
		inReplyTo    string
		references   string
		attachments  []emailAttachment
		wantContains []string
	}{
		{
			name:       "simple plain text message",
			from:       "sender@example.com",
			to:         "recipient@example.com",
			subject:    "Test Subject",
			body:       "Hello, World!",
			bodyFormat: "plain",
			wantContains: []string{
				"From: sender@example.com\r\n",
				"To: recipient@example.com\r\n",
				"Subject: Test Subject\r\n",
				"Content-Type: text/plain; charset=\"UTF-8\"\r\n",
				"Hello, World!",
				"MIME-Version: 1.0\r\n",
			},
		},
		{
			name:       "HTML message",
			from:       "sender@example.com",
			to:         "recipient@example.com",
			subject:    "HTML Test",
			body:       "<b>Bold</b>",
			bodyFormat: "html",
			wantContains: []string{
				"Content-Type: text/html; charset=\"UTF-8\"\r\n",
			},
		},
		{
			name:       "with CC and BCC",
			from:       "sender@example.com",
			to:         "to@example.com",
			cc:         "cc@example.com",
			bcc:        "bcc@example.com",
			subject:    "Test",
			body:       "body",
			bodyFormat: "plain",
			wantContains: []string{
				"Cc: cc@example.com\r\n",
				"Bcc: bcc@example.com\r\n",
			},
		},
		{
			name:       "with from name",
			from:       "sender@example.com",
			fromName:   "John Doe",
			to:         "to@example.com",
			subject:    "Test",
			body:       "body",
			bodyFormat: "plain",
			wantContains: []string{
				"John Doe",
				"sender@example.com",
			},
		},
		{
			name:       "with reply headers",
			from:       "sender@example.com",
			to:         "to@example.com",
			subject:    "Test",
			body:       "body",
			bodyFormat: "plain",
			inReplyTo:  "<original@example.com>",
			references: "<ref1@example.com> <ref2@example.com>",
			wantContains: []string{
				"In-Reply-To: <original@example.com>\r\n",
				"References: <ref1@example.com> <ref2@example.com>\r\n",
			},
		},
		{
			name:       "with attachment",
			from:       "sender@example.com",
			to:         "to@example.com",
			subject:    "Test",
			body:       "body",
			bodyFormat: "plain",
			attachments: []emailAttachment{
				{filename: "test.txt", mimeType: "text/plain", data: []byte("file content")},
			},
			wantContains: []string{
				"multipart/mixed",
				"boundary",
				"Content-Disposition: attachment; filename=\"test.txt\"",
				"Content-Transfer-Encoding: base64",
			},
		},
		{
			name:       "no to address",
			from:       "sender@example.com",
			to:         "",
			subject:    "Draft",
			body:       "body",
			bodyFormat: "plain",
			wantContains: []string{
				"From: sender@example.com",
				"Subject: Draft",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := buildRawMessage(tt.from, tt.fromName, tt.to, tt.cc, tt.bcc, tt.subject, tt.body, tt.bodyFormat, tt.inReplyTo, tt.references, tt.attachments)

			decoded := decodeRaw(t, raw)
			for _, want := range tt.wantContains {
				if !strings.Contains(decoded, want) {
					t.Errorf("decoded message missing %q\ngot: %s", want, decoded)
				}
			}

			// Verify "To" header is absent when to is empty
			if tt.to == "" && strings.Contains(decoded, "To: \r\n") {
				t.Error("unexpected empty To header")
			}
		})
	}
}

// --- formatAddress ---

func TestGmailFormatAddress(t *testing.T) {
	tests := []struct {
		name  string
		dname string
		email string
		want  string
	}{
		{
			name:  "simple name",
			dname: "John Doe",
			email: "john@example.com",
			want:  `"John Doe" <john@example.com>`,
		},
		{
			name:  "empty name",
			dname: "",
			email: "john@example.com",
			want:  "<john@example.com>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAddress(tt.dname, tt.email)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- encodeSubject ---

func TestGmailEncodeSubject(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		wantRaw bool // if true, expect subject returned as-is
	}{
		{
			name:    "ASCII only",
			subject: "Hello World",
			wantRaw: true,
		},
		{
			name:    "empty string",
			subject: "",
			wantRaw: true,
		},
		{
			name:    "with special ASCII chars",
			subject: "Re: [PR] Fix bug #123",
			wantRaw: true,
		},
		{
			name:    "with unicode characters",
			subject: "Héllo Wörld",
			wantRaw: false,
		},
		{
			name:    "with emoji",
			subject: "Hello 🌍",
			wantRaw: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encodeSubject(tt.subject)
			if tt.wantRaw {
				if got != tt.subject {
					t.Errorf("got %q, want %q (unchanged)", got, tt.subject)
				}
			} else {
				if got == tt.subject {
					t.Error("expected encoded subject, got unchanged")
				}
				if !strings.Contains(got, "=?utf-8?") {
					t.Errorf("expected RFC 2047 encoding, got %q", got)
				}
			}
		})
	}
}

// --- getStringSlice ---

func TestGmailGetStringSlice(t *testing.T) {
	tests := []struct {
		name string
		args map[string]any
		key  string
		want []string
	}{
		{
			name: "key present with string values",
			args: map[string]any{"labels": []any{"INBOX", "STARRED"}},
			key:  "labels",
			want: []string{"INBOX", "STARRED"},
		},
		{
			name: "key absent",
			args: map[string]any{},
			key:  "labels",
			want: nil,
		},
		{
			name: "key present but nil",
			args: map[string]any{"labels": nil},
			key:  "labels",
			want: nil,
		},
		{
			name: "key present but not array",
			args: map[string]any{"labels": "not-an-array"},
			key:  "labels",
			want: nil,
		},
		{
			name: "mixed types in array",
			args: map[string]any{"labels": []any{"valid", 123, "also-valid"}},
			key:  "labels",
			want: []string{"valid", "also-valid"},
		},
		{
			name: "empty array",
			args: map[string]any{"labels": []any{}},
			key:  "labels",
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := mcp.CallToolRequest{}
			request.Params.Arguments = tt.args

			got := getStringSlice(request, tt.key)

			if tt.want == nil {
				if got != nil {
					t.Errorf("got %v, want nil", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("got %v (len %d), want %v (len %d)", got, len(got), tt.want, len(tt.want))
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

// --- getAttachments ---

func TestGmailGetAttachments(t *testing.T) {
	tests := []struct {
		name  string
		args  map[string]any
		want  int
		check func(t *testing.T, atts []emailAttachment)
	}{
		{
			name: "no attachments key",
			args: map[string]any{},
			want: 0,
		},
		{
			name: "nil attachments",
			args: map[string]any{"attachments": nil},
			want: 0,
		},
		{
			name: "not an array",
			args: map[string]any{"attachments": "bad"},
			want: 0,
		},
		{
			name: "empty array",
			args: map[string]any{"attachments": []any{}},
			want: 0,
		},
		{
			name: "content-based attachment",
			args: map[string]any{
				"attachments": []any{
					map[string]any{
						"filename":  "test.txt",
						"content":   base64.StdEncoding.EncodeToString([]byte("hello")),
						"mime_type": "text/plain",
					},
				},
			},
			want: 1,
			check: func(t *testing.T, atts []emailAttachment) {
				if atts[0].filename != "test.txt" {
					t.Errorf("filename = %q, want %q", atts[0].filename, "test.txt")
				}
				if atts[0].mimeType != "text/plain" {
					t.Errorf("mimeType = %q, want %q", atts[0].mimeType, "text/plain")
				}
				if string(atts[0].data) != "hello" {
					t.Errorf("data = %q, want %q", string(atts[0].data), "hello")
				}
			},
		},
		{
			name: "invalid base64 content skipped",
			args: map[string]any{
				"attachments": []any{
					map[string]any{
						"filename": "bad.txt",
						"content":  "!!!not-valid!!!",
					},
				},
			},
			want: 0,
		},
		{
			name: "non-map item skipped",
			args: map[string]any{
				"attachments": []any{"not-a-map"},
			},
			want: 0,
		},
		{
			name: "default filename and mime type",
			args: map[string]any{
				"attachments": []any{
					map[string]any{
						"content": base64.StdEncoding.EncodeToString([]byte("data")),
					},
				},
			},
			want: 1,
			check: func(t *testing.T, atts []emailAttachment) {
				if atts[0].filename != "attachment" {
					t.Errorf("filename = %q, want %q", atts[0].filename, "attachment")
				}
				if atts[0].mimeType != "application/octet-stream" {
					t.Errorf("mimeType = %q, want %q", atts[0].mimeType, "application/octet-stream")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := mcp.CallToolRequest{}
			request.Params.Arguments = tt.args

			got := getAttachments(request)
			if len(got) != tt.want {
				t.Errorf("got %d attachments, want %d", len(got), tt.want)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}
