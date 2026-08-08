package tools

import "testing"

func TestExtractGoogleResourceID(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "raw id", input: "abc123xyz", want: "abc123xyz"},
		{name: "trimmed raw id", input: "  abc123xyz  ", want: "abc123xyz"},
		{name: "docs share url", input: "https://docs.google.com/document/d/DOCID123/edit?usp=sharing", want: "DOCID123"},
		{name: "docs url with angle brackets", input: "<https://docs.google.com/document/d/DOCID123/edit>", want: "DOCID123"},
		{name: "sheets url", input: "https://docs.google.com/spreadsheets/d/SHEET99/edit#gid=0", want: "SHEET99"},
		{name: "slides url", input: "https://docs.google.com/presentation/d/PRES55/edit", want: "PRES55"},
		{name: "forms edit url", input: "https://docs.google.com/forms/d/FORM77/edit", want: "FORM77"},
		{name: "drive file url", input: "https://drive.google.com/file/d/FILE01/view?usp=sharing", want: "FILE01"},
		{name: "drive folder url", input: "https://drive.google.com/drive/folders/FOLDER42", want: "FOLDER42"},
		{name: "drive open id", input: "https://drive.google.com/open?id=OPEN99&authuser=0", want: "OPEN99"},
		{name: "http scheme", input: "http://drive.google.com/file/d/HTTP1/view", want: "HTTP1"},
		{name: "no scheme docs host", input: "docs.google.com/document/d/NOSCHEME/edit", want: "NOSCHEME"},
		{name: "url without id", input: "https://example.com/some/page", want: ""},
		{name: "empty", input: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractGoogleResourceID(tt.input)
			if got != tt.want {
				t.Errorf("extractGoogleResourceID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
