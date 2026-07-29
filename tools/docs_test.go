package tools

import (
	"strings"
	"testing"

	docs "google.golang.org/api/docs/v1"
)

// ---------------------------------------------------------------------------
// extractDocText
// ---------------------------------------------------------------------------

func TestDocsExtractDocText(t *testing.T) {
	tests := []struct {
		name string
		doc  *docs.Document
		want string
	}{
		{
			name: "nil body returns empty",
			doc:  &docs.Document{},
			want: "",
		},
		{
			name: "single paragraph",
			doc: &docs.Document{
				Body: &docs.Body{
					Content: []*docs.StructuralElement{
						{Paragraph: &docs.Paragraph{
							Elements: []*docs.ParagraphElement{
								{TextRun: &docs.TextRun{Content: "Hello world"}},
							},
						}},
					},
				},
			},
			want: "Hello world",
		},
		{
			name: "body with tabs",
			doc: &docs.Document{
				Body: &docs.Body{
					Content: []*docs.StructuralElement{
						{Paragraph: &docs.Paragraph{
							Elements: []*docs.ParagraphElement{
								{TextRun: &docs.TextRun{Content: "Main body"}},
							},
						}},
					},
				},
				Tabs: []*docs.Tab{
					{
						TabProperties: &docs.TabProperties{Title: "Tab1", TabId: "t1"},
						DocumentTab: &docs.DocumentTab{
							Body: &docs.Body{
								Content: []*docs.StructuralElement{
									{Paragraph: &docs.Paragraph{
										Elements: []*docs.ParagraphElement{
											{TextRun: &docs.TextRun{Content: "Tab content"}},
										},
									}},
								},
							},
						},
					},
				},
			},
			want: "Main body\n--- TAB: Tab1 ---\nTab content",
		},
		{
			name: "whitespace-only body ignored",
			doc: &docs.Document{
				Body: &docs.Body{
					Content: []*docs.StructuralElement{
						{Paragraph: &docs.Paragraph{
							Elements: []*docs.ParagraphElement{
								{TextRun: &docs.TextRun{Content: "   "}},
							},
						}},
					},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDocText(tt.doc)
			if got != tt.want {
				t.Errorf("extractDocText() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// processTabHierarchy
// ---------------------------------------------------------------------------

func TestDocsProcessTabHierarchy(t *testing.T) {
	tests := []struct {
		name  string
		tab   *docs.Tab
		level int
		want  string
	}{
		{
			name: "nil document tab",
			tab:  &docs.Tab{},
			want: "",
		},
		{
			name: "top-level tab with content",
			tab: &docs.Tab{
				TabProperties: &docs.TabProperties{Title: "MyTab", TabId: "t1"},
				DocumentTab: &docs.DocumentTab{
					Body: &docs.Body{
						Content: []*docs.StructuralElement{
							{Paragraph: &docs.Paragraph{
								Elements: []*docs.ParagraphElement{
									{TextRun: &docs.TextRun{Content: "Content"}},
								},
							}},
						},
					},
				},
			},
			level: 0,
			want:  "\n--- TAB: MyTab ---\nContent",
		},
		{
			name: "nested tab adds indentation",
			tab: &docs.Tab{
				TabProperties: &docs.TabProperties{Title: "Nested", TabId: "t2"},
				DocumentTab: &docs.DocumentTab{
					Body: &docs.Body{
						Content: []*docs.StructuralElement{
							{Paragraph: &docs.Paragraph{
								Elements: []*docs.ParagraphElement{
									{TextRun: &docs.TextRun{Content: "Deep"}},
								},
							}},
						},
					},
				},
			},
			level: 2,
			want:  "\n--- TAB:         Nested ( ID: t2) ---\nDeep",
		},
		{
			name: "tab with child tabs",
			tab: &docs.Tab{
				TabProperties: &docs.TabProperties{Title: "Parent", TabId: "p1"},
				DocumentTab: &docs.DocumentTab{
					Body: &docs.Body{
						Content: []*docs.StructuralElement{
							{Paragraph: &docs.Paragraph{
								Elements: []*docs.ParagraphElement{
									{TextRun: &docs.TextRun{Content: "Parent text"}},
								},
							}},
						},
					},
				},
				ChildTabs: []*docs.Tab{
					{
						TabProperties: &docs.TabProperties{Title: "Child", TabId: "c1"},
						DocumentTab: &docs.DocumentTab{
							Body: &docs.Body{
								Content: []*docs.StructuralElement{
									{Paragraph: &docs.Paragraph{
										Elements: []*docs.ParagraphElement{
											{TextRun: &docs.TextRun{Content: "Child text"}},
										},
									}},
								},
							},
						},
					},
				},
			},
			level: 0,
			want:  "\n--- TAB: Parent ---\nParent text\n--- TAB:     Child ( ID: c1) ---\nChild text",
		},
		{
			name: "tab with no properties uses defaults",
			tab: &docs.Tab{
				DocumentTab: &docs.DocumentTab{
					Body: &docs.Body{
						Content: []*docs.StructuralElement{
							{Paragraph: &docs.Paragraph{
								Elements: []*docs.ParagraphElement{
									{TextRun: &docs.TextRun{Content: "No title"}},
								},
							}},
						},
					},
				},
			},
			level: 0,
			want:  "\n--- TAB: Untitled Tab ---\nNo title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processTabHierarchy(tt.tab, tt.level)
			if got != tt.want {
				t.Errorf("processTabHierarchy() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// extractTextFromElements / extractTextFromElementsWithDepth
// ---------------------------------------------------------------------------

func TestDocsExtractTextFromElements(t *testing.T) {
	tests := []struct {
		name     string
		elements []*docs.StructuralElement
		tabName  string
		want     string
	}{
		{
			name:     "nil elements",
			elements: nil,
			tabName:  "",
			want:     "",
		},
		{
			name: "paragraph text",
			elements: []*docs.StructuralElement{
				{Paragraph: &docs.Paragraph{
					Elements: []*docs.ParagraphElement{
						{TextRun: &docs.TextRun{Content: "Line one"}},
					},
				}},
				{Paragraph: &docs.Paragraph{
					Elements: []*docs.ParagraphElement{
						{TextRun: &docs.TextRun{Content: "Line two"}},
					},
				}},
			},
			tabName: "",
			want:    "Line oneLine two",
		},
		{
			name: "with tab name adds header",
			elements: []*docs.StructuralElement{
				{Paragraph: &docs.Paragraph{
					Elements: []*docs.ParagraphElement{
						{TextRun: &docs.TextRun{Content: "Text"}},
					},
				}},
			},
			tabName: "Notes",
			want:    "\n--- TAB: Notes ---\nText",
		},
		{
			name: "table with text in cells",
			elements: []*docs.StructuralElement{
				{Table: &docs.Table{
					TableRows: []*docs.TableRow{
						{TableCells: []*docs.TableCell{
							{Content: []*docs.StructuralElement{
								{Paragraph: &docs.Paragraph{
									Elements: []*docs.ParagraphElement{
										{TextRun: &docs.TextRun{Content: "Cell A"}},
									},
								}},
							}},
						}},
					},
				}},
			},
			tabName: "",
			want:    "Cell A",
		},
		{
			name: "whitespace-only paragraph skipped",
			elements: []*docs.StructuralElement{
				{Paragraph: &docs.Paragraph{
					Elements: []*docs.ParagraphElement{
						{TextRun: &docs.TextRun{Content: "   \n  "}},
					},
				}},
			},
			tabName: "",
			want:    "",
		},
		{
			name: "multiple text runs in paragraph",
			elements: []*docs.StructuralElement{
				{Paragraph: &docs.Paragraph{
					Elements: []*docs.ParagraphElement{
						{TextRun: &docs.TextRun{Content: "Hello "}},
						{TextRun: &docs.TextRun{Content: "World"}},
					},
				}},
			},
			tabName: "",
			want:    "Hello World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTextFromElements(tt.elements, tt.tabName)
			if got != tt.want {
				t.Errorf("extractTextFromElements() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDocsExtractTextFromElementsWithDepthLimit(t *testing.T) {
	// Verify depth > 5 returns empty
	elements := []*docs.StructuralElement{
		{Paragraph: &docs.Paragraph{
			Elements: []*docs.ParagraphElement{
				{TextRun: &docs.TextRun{Content: "Should not appear"}},
			},
		}},
	}
	got := extractTextFromElementsWithDepth(elements, "", 6)
	if got != "" {
		t.Errorf("extractTextFromElementsWithDepth(depth=6) = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// extractCellText
// ---------------------------------------------------------------------------

func TestDocsExtractCellText(t *testing.T) {
	tests := []struct {
		name string
		cell *docs.TableCell
		want string
	}{
		{
			name: "empty cell",
			cell: &docs.TableCell{},
			want: "",
		},
		{
			name: "cell with text",
			cell: &docs.TableCell{
				Content: []*docs.StructuralElement{
					{Paragraph: &docs.Paragraph{
						Elements: []*docs.ParagraphElement{
							{TextRun: &docs.TextRun{Content: "  Cell value  "}},
						},
					}},
				},
			},
			want: "Cell value",
		},
		{
			name: "cell with multiple paragraphs",
			cell: &docs.TableCell{
				Content: []*docs.StructuralElement{
					{Paragraph: &docs.Paragraph{
						Elements: []*docs.ParagraphElement{
							{TextRun: &docs.TextRun{Content: "Line 1"}},
						},
					}},
					{Paragraph: &docs.Paragraph{
						Elements: []*docs.ParagraphElement{
							{TextRun: &docs.TextRun{Content: "Line 2"}},
						},
					}},
				},
			},
			want: "Line 1Line 2",
		},
		{
			name: "cell with non-paragraph content",
			cell: &docs.TableCell{
				Content: []*docs.StructuralElement{
					{SectionBreak: &docs.SectionBreak{}},
				},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractCellText(tt.cell)
			if got != tt.want {
				t.Errorf("extractCellText() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildBasicStructure
// ---------------------------------------------------------------------------

func TestDocsBuildBasicStructure(t *testing.T) {
	tests := []struct {
		name string
		doc  *docs.Document
		// We check specific fields rather than full map equality
		wantTitle      string
		wantParagraphs int
		wantTables     int
	}{
		{
			name: "empty document",
			doc: &docs.Document{
				Title: "Empty",
				Body:  &docs.Body{Content: []*docs.StructuralElement{}},
			},
			wantTitle:      "Empty",
			wantParagraphs: 0,
			wantTables:     0,
		},
		{
			name: "document with paragraphs and table",
			doc: &docs.Document{
				Title: "Test Doc",
				Body: &docs.Body{
					Content: []*docs.StructuralElement{
						{StartIndex: 0, EndIndex: 10, Paragraph: &docs.Paragraph{}},
						{StartIndex: 10, EndIndex: 20, Paragraph: &docs.Paragraph{}},
						{StartIndex: 20, EndIndex: 50, Table: &docs.Table{
							TableRows: []*docs.TableRow{
								{TableCells: []*docs.TableCell{{}, {}}},
								{TableCells: []*docs.TableCell{{}, {}}},
							},
						}},
					},
				},
			},
			wantTitle:      "Test Doc",
			wantParagraphs: 2,
			wantTables:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildBasicStructure(tt.doc)
			if result["title"] != tt.wantTitle {
				t.Errorf("title = %v, want %v", result["title"], tt.wantTitle)
			}
			if result["paragraphs"] != tt.wantParagraphs {
				t.Errorf("paragraphs = %v, want %v", result["paragraphs"], tt.wantParagraphs)
			}
			if result["tables"] != tt.wantTables {
				t.Errorf("tables = %v, want %v", result["tables"], tt.wantTables)
			}
		})
	}
}

func TestDocsBuildBasicStructureTableDetails(t *testing.T) {
	doc := &docs.Document{
		Title: "With Table",
		Body: &docs.Body{
			Content: []*docs.StructuralElement{
				{StartIndex: 0, EndIndex: 5, Paragraph: &docs.Paragraph{}},
				{StartIndex: 5, EndIndex: 30, Table: &docs.Table{
					TableRows: []*docs.TableRow{
						{TableCells: []*docs.TableCell{{}, {}, {}}},
						{TableCells: []*docs.TableCell{{}, {}, {}}},
					},
				}},
			},
		},
	}

	result := buildBasicStructure(doc)
	details, ok := result["table_details"].([]map[string]any)
	if !ok || len(details) != 1 {
		t.Fatalf("expected 1 table detail, got %v", result["table_details"])
	}
	if details[0]["rows"] != int64(2) {
		t.Errorf("rows = %v, want 2", details[0]["rows"])
	}
	if details[0]["columns"] != int64(3) {
		t.Errorf("columns = %v, want 3", details[0]["columns"])
	}
}

// ---------------------------------------------------------------------------
// buildDetailedStructure
// ---------------------------------------------------------------------------

func TestDocsBuildDetailedStructure(t *testing.T) {
	doc := &docs.Document{
		Title: "Detailed Test",
		Body: &docs.Body{
			Content: []*docs.StructuralElement{
				{StartIndex: 0, EndIndex: 1, SectionBreak: &docs.SectionBreak{}},
				{StartIndex: 1, EndIndex: 15, Paragraph: &docs.Paragraph{
					Elements: []*docs.ParagraphElement{
						{TextRun: &docs.TextRun{Content: "Hello world"}},
					},
				}},
				{StartIndex: 15, EndIndex: 40, Table: &docs.Table{
					TableRows: []*docs.TableRow{
						{TableCells: []*docs.TableCell{
							{Content: []*docs.StructuralElement{
								{Paragraph: &docs.Paragraph{
									Elements: []*docs.ParagraphElement{
										{TextRun: &docs.TextRun{Content: "Header"}},
									},
								}},
							}},
						}},
					},
				}},
			},
		},
		Headers: map[string]docs.Header{"h1": {}},
	}

	result := buildDetailedStructure(doc)

	if result["title"] != "Detailed Test" {
		t.Errorf("title = %v, want 'Detailed Test'", result["title"])
	}

	stats, ok := result["statistics"].(map[string]any)
	if !ok {
		t.Fatal("missing statistics")
	}
	if stats["has_headers"] != true {
		t.Errorf("has_headers = %v, want true", stats["has_headers"])
	}
	if stats["has_footers"] != false {
		t.Errorf("has_footers = %v, want false", stats["has_footers"])
	}

	elems, ok := result["elements"].([]map[string]any)
	if !ok {
		t.Fatal("missing elements")
	}
	if len(elems) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(elems))
	}
	if elems[0]["type"] != "section_break" {
		t.Errorf("elem[0] type = %v, want section_break", elems[0]["type"])
	}
	if elems[1]["type"] != "paragraph" {
		t.Errorf("elem[1] type = %v, want paragraph", elems[1]["type"])
	}
	if elems[2]["type"] != "table" {
		t.Errorf("elem[2] type = %v, want table", elems[2]["type"])
	}
}

func TestDocsBuildDetailedStructureTextPreviewTruncation(t *testing.T) {
	// Text > 100 chars should be truncated in the preview
	longText := strings.Repeat("x", 120)
	doc := &docs.Document{
		Title: "Long Text",
		Body: &docs.Body{
			Content: []*docs.StructuralElement{
				{StartIndex: 0, EndIndex: 130, Paragraph: &docs.Paragraph{
					Elements: []*docs.ParagraphElement{
						{TextRun: &docs.TextRun{Content: longText}},
					},
				}},
			},
		},
	}

	result := buildDetailedStructure(doc)
	elems, ok := result["elements"].([]map[string]any)
	if !ok || len(elems) == 0 {
		t.Fatalf("expected elements slice, got %T", result["elements"])
	}
	preview, ok := elems[0]["text_preview"].(string)
	if !ok {
		t.Fatalf("expected text_preview string, got %T", elems[0]["text_preview"])
	}
	if len(preview) != 100 {
		t.Errorf("text_preview length = %d, want 100", len(preview))
	}
}

// ---------------------------------------------------------------------------
// normalizeColor
// ---------------------------------------------------------------------------

func TestDocsNormalizeColor(t *testing.T) {
	tests := []struct {
		name    string
		hex     string
		wantNil bool
		wantR   float64
		wantG   float64
		wantB   float64
	}{
		{
			name:  "black",
			hex:   "#000000",
			wantR: 0, wantG: 0, wantB: 0,
		},
		{
			name:  "white",
			hex:   "#FFFFFF",
			wantR: 1, wantG: 1, wantB: 1,
		},
		{
			name:  "red",
			hex:   "#FF0000",
			wantR: 1, wantG: 0, wantB: 0,
		},
		{
			name:    "too short",
			hex:     "#FFF",
			wantNil: true,
		},
		{
			name:    "no hash",
			hex:     "FF0000",
			wantNil: true,
		},
		{
			name:    "invalid hex chars",
			hex:     "#GGHHII",
			wantNil: true,
		},
		{
			name:    "empty string",
			hex:     "",
			wantNil: true,
		},
		{
			name:  "lowercase hex",
			hex:   "#ff8040",
			wantR: 1, wantG: float64(0x80) / 255, wantB: float64(0x40) / 255,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeColor(tt.hex)
			if tt.wantNil {
				if got != nil {
					t.Errorf("normalizeColor(%q) = %v, want nil", tt.hex, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("normalizeColor(%q) = nil, want non-nil", tt.hex)
				return
			}
			if got.Red != tt.wantR {
				t.Errorf("Red = %v, want %v", got.Red, tt.wantR)
			}
			if got.Green != tt.wantG {
				t.Errorf("Green = %v, want %v", got.Green, tt.wantG)
			}
			if got.Blue != tt.wantB {
				t.Errorf("Blue = %v, want %v", got.Blue, tt.wantB)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// hexToDec
// ---------------------------------------------------------------------------

func TestDocsHexToDec(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int
	}{
		{name: "00", s: "00", want: 0},
		{name: "FF", s: "FF", want: 255},
		{name: "ff lowercase", s: "ff", want: 255},
		{name: "0A", s: "0A", want: 10},
		{name: "7F", s: "7F", want: 127},
		{name: "invalid char", s: "GG", want: -1},
		{name: "mixed valid/invalid", s: "0G", want: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hexToDec(tt.s)
			if got != tt.want {
				t.Errorf("hexToDec(%q) = %d, want %d", tt.s, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildTextStyle
// ---------------------------------------------------------------------------

func TestDocsBuildTextStyle(t *testing.T) {
	tests := []struct {
		name       string
		args       map[string]any
		hasBold    bool
		hasItalic  bool
		hasUndline bool
		fontSize   int
		fontFamily string
		textColor  string
		bgColor    string
		wantFields int // number of fields
	}{
		{
			name:       "bold only",
			args:       map[string]any{"bold": true},
			hasBold:    true,
			wantFields: 1,
		},
		{
			name:       "italic false (explicit)",
			args:       map[string]any{"italic": false},
			hasItalic:  true,
			wantFields: 1,
		},
		{
			name:       "font size and family",
			args:       map[string]any{},
			fontSize:   14,
			fontFamily: "Arial",
			wantFields: 2,
		},
		{
			name:       "text color",
			args:       map[string]any{},
			textColor:  "#FF0000",
			wantFields: 1,
		},
		{
			name:       "invalid text color ignored",
			args:       map[string]any{},
			textColor:  "red",
			wantFields: 0,
		},
		{
			name:       "background color",
			args:       map[string]any{},
			bgColor:    "#00FF00",
			wantFields: 1,
		},
		{
			name:       "all formatting options",
			args:       map[string]any{"bold": true, "italic": true, "underline": true},
			hasBold:    true,
			hasItalic:  true,
			hasUndline: true,
			fontSize:   12,
			fontFamily: "Times New Roman",
			textColor:  "#000000",
			bgColor:    "#FFFF00",
			wantFields: 7,
		},
		{
			name:       "no formatting",
			args:       map[string]any{},
			wantFields: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style, fields := buildTextStyle(tt.args, tt.hasBold, tt.hasItalic, tt.hasUndline, tt.fontSize, tt.fontFamily, tt.textColor, tt.bgColor)
			if len(fields) != tt.wantFields {
				t.Errorf("buildTextStyle() fields count = %d, want %d; fields = %v", len(fields), tt.wantFields, fields)
			}
			if style == nil {
				t.Error("buildTextStyle() returned nil style")
			}
		})
	}
}

func TestDocsBuildTextStyleBoldFalse(t *testing.T) {
	// When bold is explicitly false, ForceSendFields should contain "Bold"
	args := map[string]any{"bold": false}
	style, _ := buildTextStyle(args, true, false, false, 0, "", "", "")
	if style.Bold != false {
		t.Errorf("Bold = %v, want false", style.Bold)
	}
	found := false
	for _, f := range style.ForceSendFields {
		if f == "Bold" {
			found = true
		}
	}
	if !found {
		t.Error("ForceSendFields should contain 'Bold' when bold is explicitly false")
	}
}

// ---------------------------------------------------------------------------
// buildBatchOperationRequest
// ---------------------------------------------------------------------------

func TestDocsBuildBatchOperationRequest(t *testing.T) {
	tests := []struct {
		name    string
		op      map[string]any
		opType  string
		opNum   int
		wantErr bool
		wantN   int // number of requests returned
	}{
		{
			name:   "insert_text",
			op:     map[string]any{"type": "insert_text", "index": float64(1), "text": "Hello"},
			opType: "insert_text",
			opNum:  1,
			wantN:  1,
		},
		{
			name:   "delete_text",
			op:     map[string]any{"type": "delete_text", "start_index": float64(1), "end_index": float64(5)},
			opType: "delete_text",
			opNum:  1,
			wantN:  1,
		},
		{
			name:   "replace_text",
			op:     map[string]any{"type": "replace_text", "start_index": float64(1), "end_index": float64(5), "text": "New"},
			opType: "replace_text",
			opNum:  1,
			wantN:  2, // delete + insert
		},
		{
			name:   "replace_text long preview truncated",
			op:     map[string]any{"type": "replace_text", "start_index": float64(1), "end_index": float64(5), "text": "This is a very long replacement text that exceeds twenty characters"},
			opType: "replace_text",
			opNum:  1,
			wantN:  2,
		},
		{
			name:   "format_text with bold",
			op:     map[string]any{"type": "format_text", "start_index": float64(1), "end_index": float64(5), "bold": true},
			opType: "format_text",
			opNum:  1,
			wantN:  1,
		},
		{
			name:    "format_text no formatting",
			op:      map[string]any{"type": "format_text", "start_index": float64(1), "end_index": float64(5)},
			opType:  "format_text",
			opNum:   1,
			wantErr: true,
		},
		{
			name:   "insert_table",
			op:     map[string]any{"type": "insert_table", "index": float64(1), "rows": float64(3), "columns": float64(2)},
			opType: "insert_table",
			opNum:  1,
			wantN:  1,
		},
		{
			name:   "insert_page_break",
			op:     map[string]any{"type": "insert_page_break", "index": float64(5)},
			opType: "insert_page_break",
			opNum:  1,
			wantN:  1,
		},
		{
			name:   "find_replace",
			op:     map[string]any{"type": "find_replace", "find_text": "old", "replace_text": "new", "match_case": true},
			opType: "find_replace",
			opNum:  1,
			wantN:  1,
		},
		{
			name:   "find_replace no match_case",
			op:     map[string]any{"type": "find_replace", "find_text": "old", "replace_text": "new"},
			opType: "find_replace",
			opNum:  1,
			wantN:  1,
		},
		{
			name:    "unsupported type",
			op:      map[string]any{"type": "unknown"},
			opType:  "unknown",
			opNum:   1,
			wantErr: true,
		},
		{
			name:    "insert_text missing index",
			op:      map[string]any{"type": "insert_text", "text": "Hello"},
			opType:  "insert_text",
			opNum:   1,
			wantErr: true,
		},
		{
			name:    "insert_text missing text",
			op:      map[string]any{"type": "insert_text", "index": float64(1)},
			opType:  "insert_text",
			opNum:   1,
			wantErr: true,
		},
		{
			name:    "delete_text missing end_index",
			op:      map[string]any{"type": "delete_text", "start_index": float64(1)},
			opType:  "delete_text",
			opNum:   1,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqs, desc, err := buildBatchOperationRequest(tt.op, tt.opType, tt.opNum)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(reqs) != tt.wantN {
				t.Errorf("got %d requests, want %d", len(reqs), tt.wantN)
			}
			if desc == "" {
				t.Error("description should not be empty")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseTableData
// ---------------------------------------------------------------------------

func TestDocsParseTableData(t *testing.T) {
	tests := []struct {
		name    string
		raw     any
		want    [][]string
		wantErr bool
	}{
		{
			name: "valid 2D string array",
			raw: []any{
				[]any{"A", "B"},
				[]any{"C", "D"},
			},
			want: [][]string{{"A", "B"}, {"C", "D"}},
		},
		{
			name: "non-string values converted",
			raw: []any{
				[]any{"A", float64(42)},
			},
			want: [][]string{{"A", "42"}},
		},
		{
			name:    "not an array",
			raw:     "not an array",
			wantErr: true,
		},
		{
			name:    "row is not an array",
			raw:     []any{"not-a-row"},
			wantErr: true,
		},
		{
			name: "empty table",
			raw:  []any{},
			want: nil,
		},
		{
			name: "single cell",
			raw: []any{
				[]any{"Only cell"},
			},
			want: [][]string{{"Only cell"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTableData(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d rows, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if len(got[i]) != len(tt.want[i]) {
					t.Fatalf("row %d: got %d cols, want %d", i, len(got[i]), len(tt.want[i]))
				}
				for j := range got[i] {
					if got[i][j] != tt.want[i][j] {
						t.Errorf("cell [%d][%d] = %q, want %q", i, j, got[i][j], tt.want[i][j])
					}
				}
			}
		})
	}
}
