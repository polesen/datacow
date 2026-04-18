package views

import (
	"strings"
	"testing"
)

func TestSuggestFilename(t *testing.T) {
	tests := []struct {
		name     string
		table    string
		pkValues []string
		column   string
		raw      []byte
		want     string
	}{
		{
			name:     "simple text",
			table:    "users",
			pkValues: []string{"42"},
			column:   "bio",
			raw:      []byte("hello"),
			want:     "users-42-bio.txt",
		},
		{
			name:     "json column",
			table:    "documents",
			pkValues: []string{"7"},
			column:   "body",
			raw:      []byte(`{"key":"value"}`),
			want:     "documents-7-body.json",
		},
		{
			name:     "compound pk",
			table:    "orders",
			pkValues: []string{"5", "12"},
			column:   "notes",
			raw:      []byte("text"),
			want:     "orders-5-12-notes.txt",
		},
		{
			name:     "no pk values",
			table:    "users",
			pkValues: nil,
			column:   "bio",
			raw:      []byte("text"),
			want:     "users-bio.txt",
		},
		{
			name:     "spaces in table name",
			table:    "my table",
			pkValues: []string{"1"},
			column:   "col",
			raw:      []byte("x"),
			want:     "my-table-1-col.txt",
		},
		{
			name:     "special chars stripped",
			table:    "users/data",
			pkValues: []string{"1"},
			column:   "bio",
			raw:      []byte("text"),
			want:     "usersdata-1-bio.txt",
		},
		{
			name:     "segment truncated at 40",
			table:    strings.Repeat("a", 50),
			pkValues: []string{"1"},
			column:   "col",
			raw:      []byte("text"),
			want:     strings.Repeat("a", 40) + "-1-col.txt",
		},
		{
			name:     "empty segment becomes underscore",
			table:    "!!!",
			pkValues: []string{"1"},
			column:   "col",
			raw:      []byte("text"),
			want:     "_-1-col.txt",
		},
		{
			name:     "pk value with equals sign stripped",
			table:    "t",
			pkValues: []string{"id=5"},
			column:   "c",
			raw:      []byte("x"),
			want:     "t-id5-c.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := suggestFilename(tt.table, tt.pkValues, tt.column, tt.raw)
			if got != tt.want {
				t.Errorf("suggestFilename(%q, %v, %q, ...) = %q, want %q",
					tt.table, tt.pkValues, tt.column, got, tt.want)
			}
		})
	}
}

func TestInferExtension(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"json object", []byte(`{"a":1}`), ".json"},
		{"json array", []byte(`[1,2,3]`), ".json"},
		{"json string", []byte(`"hello"`), ".json"},
		{"xml", []byte(`<root><child/></root>`), ".xml"},
		{"xml with leading whitespace", []byte("  \n<root/>"), ".xml"},
		{"png magic", []byte("\x89PNG\r\n\x1a\n"), ".png"},
		{"jpg magic", []byte("\xFF\xD8\xFF\xE0"), ".jpg"},
		{"gif magic", []byte("GIF89a"), ".gif"},
		{"gif87 magic", []byte("GIF87a"), ".gif"},
		{"webp magic", append(append([]byte("RIFF"), 0, 0, 0, 0), []byte("WEBP")...), ".webp"},
		{"pdf magic", []byte("%PDF-1.4"), ".pdf"},
		{"zip magic", []byte("PK\x03\x04\x14\x00"), ".zip"},
		{"plain text", []byte("hello world"), ".txt"},
		{"empty", []byte{}, ".txt"},
		{"utf8 text", []byte("こんにちは"), ".txt"},
		{"binary non-utf8", []byte{0x80, 0xFE, 0xFF, 0x00}, ".bin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferExtension(tt.data)
			if got != tt.want {
				t.Errorf("inferExtension() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWrapText(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  []string
	}{
		{
			name:  "short line",
			text:  "hello",
			width: 10,
			want:  []string{"hello"},
		},
		{
			name:  "exact width",
			text:  "hello",
			width: 5,
			want:  []string{"hello"},
		},
		{
			name:  "wrap long line",
			text:  "hello world",
			width: 5,
			want:  []string{"hello", " worl", "d"},
		},
		{
			name:  "newlines preserved",
			text:  "hello\nworld",
			width: 10,
			want:  []string{"hello", "world"},
		},
		{
			name:  "empty string",
			text:  "",
			width: 10,
			want:  []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := wrapText(tt.text, tt.width)
			if len(got) != len(tt.want) {
				t.Fatalf("wrapText() len=%d, want %d: got %v, want %v", len(got), len(tt.want), got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("wrapText()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
