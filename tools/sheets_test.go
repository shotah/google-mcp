package tools

import (
	"strings"
	"testing"

	sheets "google.golang.org/api/sheets/v4"
)

// --- columnToIndex ---

func TestParseSheetUpdates(t *testing.T) {
	updates, errMsg := parseSheetUpdates([]any{
		map[string]any{"range_name": "Sheet1!A1", "values": []any{[]any{"a", "b"}, []any{"1", "2"}}},
		map[string]any{"range_name": "Sheet1!A3", "values": []any{[]any{"c"}}},
	})
	if errMsg != "" {
		t.Fatal(errMsg)
	}
	if len(updates) != 2 || updates[0].Range != "Sheet1!A1" || len(updates[0].Values) != 2 {
		t.Fatalf("updates=%+v", updates)
	}
	if _, errMsg := parseSheetUpdates([]any{map[string]any{"values": []any{[]any{"a"}}}}); errMsg == "" {
		t.Fatal("expected missing range_name error")
	}
}

func TestSheetsColumnToIndex(t *testing.T) {
	tests := []struct {
		name string
		col  string
		want int
	}{
		{name: "empty string", col: "", want: -1},
		{name: "column A", col: "A", want: 0},
		{name: "column B", col: "B", want: 1},
		{name: "column Z", col: "Z", want: 25},
		{name: "column AA", col: "AA", want: 26},
		{name: "column AB", col: "AB", want: 27},
		{name: "column AZ", col: "AZ", want: 51},
		{name: "column BA", col: "BA", want: 52},
		{name: "lowercase a", col: "a", want: 0},
		{name: "lowercase z", col: "z", want: 25},
		{name: "lowercase aa", col: "aa", want: 26},
		{name: "mixed case Ab", col: "Ab", want: 27},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := columnToIndex(tt.col)
			if got != tt.want {
				t.Errorf("columnToIndex(%q) = %d, want %d", tt.col, got, tt.want)
			}
		})
	}
}

// --- indexToColumn ---

func TestSheetsIndexToColumn(t *testing.T) {
	tests := []struct {
		name string
		idx  int
		want string
	}{
		{name: "negative index", idx: -1, want: ""},
		{name: "index 0 is A", idx: 0, want: "A"},
		{name: "index 1 is B", idx: 1, want: "B"},
		{name: "index 25 is Z", idx: 25, want: "Z"},
		{name: "index 26 is AA", idx: 26, want: "AA"},
		{name: "index 27 is AB", idx: 27, want: "AB"},
		{name: "index 51 is AZ", idx: 51, want: "AZ"},
		{name: "index 52 is BA", idx: 52, want: "BA"},
		{name: "negative -100", idx: -100, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := indexToColumn(tt.idx)
			if got != tt.want {
				t.Errorf("indexToColumn(%d) = %q, want %q", tt.idx, got, tt.want)
			}
		})
	}
}

// --- columnToIndex / indexToColumn roundtrip ---

func TestSheetsColumnIndexRoundtrip(t *testing.T) {
	tests := []struct {
		name string
		col  string
	}{
		{name: "A roundtrip", col: "A"},
		{name: "Z roundtrip", col: "Z"},
		{name: "AA roundtrip", col: "AA"},
		{name: "AZ roundtrip", col: "AZ"},
		{name: "BA roundtrip", col: "BA"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := columnToIndex(tt.col)
			got := indexToColumn(idx)
			if got != tt.col {
				t.Errorf("roundtrip(%q) -> %d -> %q, want %q", tt.col, idx, got, tt.col)
			}
		})
	}
}

// --- parseA1Part ---

func TestSheetsParseA1Part(t *testing.T) {
	tests := []struct {
		name    string
		part    string
		wantCol int
		wantRow int
	}{
		{name: "cell A1", part: "A1", wantCol: 0, wantRow: 0},
		{name: "cell B2", part: "B2", wantCol: 1, wantRow: 1},
		{name: "cell Z26", part: "Z26", wantCol: 25, wantRow: 25},
		{name: "cell AA100", part: "AA100", wantCol: 26, wantRow: 99},
		{name: "column only A", part: "A", wantCol: 0, wantRow: -1},
		{name: "row only 1", part: "1", wantCol: -1, wantRow: 0},
		{name: "row only 10", part: "10", wantCol: -1, wantRow: 9},
		{name: "with dollar sign $A$1", part: "$A$1", wantCol: 0, wantRow: 0},
		{name: "with dollar sign $B2", part: "$B2", wantCol: 1, wantRow: 1},
		{name: "with dollar sign A$3", part: "A$3", wantCol: 0, wantRow: 2},
		{name: "empty string", part: "", wantCol: -1, wantRow: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCol, gotRow := parseA1Part(tt.part)
			if gotCol != tt.wantCol {
				t.Errorf("parseA1Part(%q) col = %d, want %d", tt.part, gotCol, tt.wantCol)
			}
			if gotRow != tt.wantRow {
				t.Errorf("parseA1Part(%q) row = %d, want %d", tt.part, gotRow, tt.wantRow)
			}
		})
	}
}

// --- splitSheetAndRange ---

func TestSheetsSplitSheetAndRange(t *testing.T) {
	tests := []struct {
		name          string
		rangeName     string
		wantSheetName string
		wantA1Range   string
	}{
		{
			name:          "no sheet prefix",
			rangeName:     "A1:B2",
			wantSheetName: "",
			wantA1Range:   "A1:B2",
		},
		{
			name:          "simple sheet prefix",
			rangeName:     "Sheet1!A1:B2",
			wantSheetName: "Sheet1",
			wantA1Range:   "A1:B2",
		},
		{
			name:          "quoted sheet name",
			rangeName:     "'My Sheet'!C3:D4",
			wantSheetName: "My Sheet",
			wantA1Range:   "C3:D4",
		},
		{
			name:          "quoted sheet name with escaped single quote",
			rangeName:     "'It''s a sheet'!A1",
			wantSheetName: "It's a sheet",
			wantA1Range:   "A1",
		},
		{
			name:          "single cell no sheet",
			rangeName:     "A1",
			wantSheetName: "",
			wantA1Range:   "A1",
		},
		{
			name:          "column-only range with sheet",
			rangeName:     "Data!A:C",
			wantSheetName: "Data",
			wantA1Range:   "A:C",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSheet, gotRange := splitSheetAndRange(tt.rangeName)
			if gotSheet != tt.wantSheetName {
				t.Errorf("splitSheetAndRange(%q) sheetName = %q, want %q", tt.rangeName, gotSheet, tt.wantSheetName)
			}
			if gotRange != tt.wantA1Range {
				t.Errorf("splitSheetAndRange(%q) a1Range = %q, want %q", tt.rangeName, gotRange, tt.wantA1Range)
			}
		})
	}
}

// --- parseA1Range ---

func TestSheetsParseA1Range(t *testing.T) {
	defaultInfos := []sheetInfo{
		{SheetID: 0, Title: "Sheet1", Rows: 1000, Cols: 26},
		{SheetID: 1, Title: "Sheet2", Rows: 500, Cols: 10},
	}

	tests := []struct {
		name        string
		rangeName   string
		sheetInfos  []sheetInfo
		wantErr     bool
		errContains string
		check       func(t *testing.T, gr *sheetsGridRange)
	}{
		{
			name:       "simple range A1:B2 defaults to first sheet",
			rangeName:  "A1:B2",
			sheetInfos: defaultInfos,
			check: func(t *testing.T, gr *sheetsGridRange) {
				if gr.SheetID != 0 {
					t.Errorf("SheetID = %d, want 0", gr.SheetID)
				}
				if gr.StartRowIndex != 0 || !gr.hasStartRow {
					t.Errorf("StartRowIndex = %d (has=%v), want 0 (has=true)", gr.StartRowIndex, gr.hasStartRow)
				}
				if gr.StartColumnIndex != 0 || !gr.hasStartCol {
					t.Errorf("StartColumnIndex = %d (has=%v), want 0 (has=true)", gr.StartColumnIndex, gr.hasStartCol)
				}
				if gr.EndRowIndex != 2 || !gr.hasEndRow {
					t.Errorf("EndRowIndex = %d (has=%v), want 2 (has=true)", gr.EndRowIndex, gr.hasEndRow)
				}
				if gr.EndColumnIndex != 2 || !gr.hasEndCol {
					t.Errorf("EndColumnIndex = %d (has=%v), want 2 (has=true)", gr.EndColumnIndex, gr.hasEndCol)
				}
			},
		},
		{
			name:       "with sheet name Sheet2!C3:D4",
			rangeName:  "Sheet2!C3:D4",
			sheetInfos: defaultInfos,
			check: func(t *testing.T, gr *sheetsGridRange) {
				if gr.SheetID != 1 {
					t.Errorf("SheetID = %d, want 1", gr.SheetID)
				}
				if gr.StartColumnIndex != 2 {
					t.Errorf("StartColumnIndex = %d, want 2", gr.StartColumnIndex)
				}
				if gr.StartRowIndex != 2 {
					t.Errorf("StartRowIndex = %d, want 2", gr.StartRowIndex)
				}
				if gr.EndColumnIndex != 4 {
					t.Errorf("EndColumnIndex = %d, want 4", gr.EndColumnIndex)
				}
				if gr.EndRowIndex != 4 {
					t.Errorf("EndRowIndex = %d, want 4", gr.EndRowIndex)
				}
			},
		},
		{
			name:       "single cell A1",
			rangeName:  "A1",
			sheetInfos: defaultInfos,
			check: func(t *testing.T, gr *sheetsGridRange) {
				if gr.StartRowIndex != 0 || gr.EndRowIndex != 1 {
					t.Errorf("row range = [%d, %d), want [0, 1)", gr.StartRowIndex, gr.EndRowIndex)
				}
				if gr.StartColumnIndex != 0 || gr.EndColumnIndex != 1 {
					t.Errorf("col range = [%d, %d), want [0, 1)", gr.StartColumnIndex, gr.EndColumnIndex)
				}
			},
		},
		{
			name:       "column-only range A:C",
			rangeName:  "A:C",
			sheetInfos: defaultInfos,
			check: func(t *testing.T, gr *sheetsGridRange) {
				if gr.hasStartRow {
					t.Error("expected hasStartRow=false for column-only range")
				}
				if gr.hasEndRow {
					t.Error("expected hasEndRow=false for column-only range")
				}
				if !gr.hasStartCol || gr.StartColumnIndex != 0 {
					t.Errorf("StartColumnIndex = %d (has=%v), want 0 (has=true)", gr.StartColumnIndex, gr.hasStartCol)
				}
				if !gr.hasEndCol || gr.EndColumnIndex != 3 {
					t.Errorf("EndColumnIndex = %d (has=%v), want 3 (has=true)", gr.EndColumnIndex, gr.hasEndCol)
				}
			},
		},
		{
			name:        "empty sheetInfos returns error",
			rangeName:   "A1:B2",
			sheetInfos:  []sheetInfo{},
			wantErr:     true,
			errContains: "no sheets",
		},
		{
			name:        "unknown sheet name returns error",
			rangeName:   "Unknown!A1:B2",
			sheetInfos:  defaultInfos,
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:        "empty a1Range returns error",
			rangeName:   "Sheet1!",
			sheetInfos:  defaultInfos,
			wantErr:     true,
			errContains: "must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseA1Range(tt.rangeName, tt.sheetInfos)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

// --- sheetsGridRange.toMap ---

func TestSheetsGridRangeToMap(t *testing.T) {
	tests := []struct {
		name     string
		gr       sheetsGridRange
		wantKeys []string
		check    func(t *testing.T, m map[string]int64)
	}{
		{
			name: "all fields set",
			gr: sheetsGridRange{
				SheetID: 42, StartRowIndex: 1, EndRowIndex: 5,
				StartColumnIndex: 2, EndColumnIndex: 8,
				hasStartRow: true, hasEndRow: true,
				hasStartCol: true, hasEndCol: true,
			},
			wantKeys: []string{"sheetId", "startRowIndex", "endRowIndex", "startColumnIndex", "endColumnIndex"},
			check: func(t *testing.T, m map[string]int64) {
				if m["sheetId"] != 42 {
					t.Errorf("sheetId = %d, want 42", m["sheetId"])
				}
				if m["startRowIndex"] != 1 {
					t.Errorf("startRowIndex = %d, want 1", m["startRowIndex"])
				}
				if m["endRowIndex"] != 5 {
					t.Errorf("endRowIndex = %d, want 5", m["endRowIndex"])
				}
			},
		},
		{
			name:     "only sheetId when no flags set",
			gr:       sheetsGridRange{SheetID: 7},
			wantKeys: []string{"sheetId"},
			check: func(t *testing.T, m map[string]int64) {
				if len(m) != 1 {
					t.Errorf("got %d keys, want 1", len(m))
				}
				if m["sheetId"] != 7 {
					t.Errorf("sheetId = %d, want 7", m["sheetId"])
				}
			},
		},
		{
			name: "partial fields set",
			gr: sheetsGridRange{
				SheetID: 0, StartColumnIndex: 3, EndColumnIndex: 5,
				hasStartCol: true, hasEndCol: true,
			},
			wantKeys: []string{"sheetId", "startColumnIndex", "endColumnIndex"},
			check: func(t *testing.T, m map[string]int64) {
				if _, ok := m["startRowIndex"]; ok {
					t.Error("unexpected startRowIndex in map")
				}
				if _, ok := m["endRowIndex"]; ok {
					t.Error("unexpected endRowIndex in map")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.gr.toMap()
			for _, key := range tt.wantKeys {
				if _, ok := m[key]; !ok {
					t.Errorf("missing expected key %q in map", key)
				}
			}
			if tt.check != nil {
				tt.check(t, m)
			}
		})
	}
}

// --- sheetsGridRange.toSheetsGridRange ---

func TestSheetsGridRangeToSheetsGridRange(t *testing.T) {
	tests := []struct {
		name  string
		gr    sheetsGridRange
		check func(t *testing.T, r *sheets.GridRange)
	}{
		{
			name: "all fields set",
			gr: sheetsGridRange{
				SheetID: 10, StartRowIndex: 2, EndRowIndex: 8,
				StartColumnIndex: 1, EndColumnIndex: 4,
				hasStartRow: true, hasEndRow: true,
				hasStartCol: true, hasEndCol: true,
			},
			check: func(t *testing.T, r *sheets.GridRange) {
				if r.SheetId != 10 {
					t.Errorf("SheetId = %d, want 10", r.SheetId)
				}
				if r.StartRowIndex != 2 {
					t.Errorf("StartRowIndex = %d, want 2", r.StartRowIndex)
				}
				if r.EndRowIndex != 8 {
					t.Errorf("EndRowIndex = %d, want 8", r.EndRowIndex)
				}
				if r.StartColumnIndex != 1 {
					t.Errorf("StartColumnIndex = %d, want 1", r.StartColumnIndex)
				}
				if r.EndColumnIndex != 4 {
					t.Errorf("EndColumnIndex = %d, want 4", r.EndColumnIndex)
				}
				// ForceSendFields should contain all fields
				wantForce := map[string]bool{
					"SheetId": true, "StartRowIndex": true, "EndRowIndex": true,
					"StartColumnIndex": true, "EndColumnIndex": true,
				}
				for _, f := range r.ForceSendFields {
					delete(wantForce, f)
				}
				if len(wantForce) > 0 {
					t.Errorf("missing ForceSendFields: %v", wantForce)
				}
			},
		},
		{
			name: "only sheetId",
			gr:   sheetsGridRange{SheetID: 5},
			check: func(t *testing.T, r *sheets.GridRange) {
				if r.SheetId != 5 {
					t.Errorf("SheetId = %d, want 5", r.SheetId)
				}
				if len(r.ForceSendFields) != 1 || r.ForceSendFields[0] != "SheetId" {
					t.Errorf("ForceSendFields = %v, want [SheetId]", r.ForceSendFields)
				}
			},
		},
		{
			name: "sheetId zero with row flags",
			gr: sheetsGridRange{
				SheetID: 0, StartRowIndex: 0, EndRowIndex: 1,
				hasStartRow: true, hasEndRow: true,
			},
			check: func(t *testing.T, r *sheets.GridRange) {
				if r.SheetId != 0 {
					t.Errorf("SheetId = %d, want 0", r.SheetId)
				}
				if r.StartRowIndex != 0 {
					t.Errorf("StartRowIndex = %d, want 0", r.StartRowIndex)
				}
				if r.EndRowIndex != 1 {
					t.Errorf("EndRowIndex = %d, want 1", r.EndRowIndex)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.gr.toSheetsGridRange()
			if tt.check != nil {
				tt.check(t, r)
			}
		})
	}
}

// --- parseHexColorSheets ---

func TestSheetsParseHexColorSheets(t *testing.T) {
	tests := []struct {
		name      string
		color     string
		wantRed   float64
		wantGreen float64
		wantBlue  float64
		wantOK    bool
	}{
		{name: "empty string", color: "", wantOK: false},
		{name: "valid with hash", color: "#FF0000", wantRed: 1.0, wantGreen: 0, wantBlue: 0, wantOK: true},
		{name: "valid without hash", color: "00FF00", wantRed: 0, wantGreen: 1.0, wantBlue: 0, wantOK: true},
		{name: "blue", color: "#0000FF", wantRed: 0, wantGreen: 0, wantBlue: 1.0, wantOK: true},
		{name: "white", color: "#FFFFFF", wantRed: 1.0, wantGreen: 1.0, wantBlue: 1.0, wantOK: true},
		{name: "black", color: "#000000", wantRed: 0, wantGreen: 0, wantBlue: 0, wantOK: true},
		{name: "mixed case hex", color: "#aaBBcc", wantRed: 170.0 / 255.0, wantGreen: 187.0 / 255.0, wantBlue: 204.0 / 255.0, wantOK: true},
		{name: "too short", color: "#FFF", wantOK: false},
		{name: "too long", color: "#FF00FF00", wantOK: false},
		{name: "invalid hex chars", color: "#GGHHII", wantOK: false},
		{name: "with leading space", color: "  #FF0000", wantRed: 1.0, wantGreen: 0, wantBlue: 0, wantOK: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b, ok := parseHexColorSheets(tt.color)
			if ok != tt.wantOK {
				t.Errorf("parseHexColorSheets(%q) ok = %v, want %v", tt.color, ok, tt.wantOK)
				return
			}
			if !ok {
				return
			}
			const epsilon = 0.001
			if diff := r - tt.wantRed; diff > epsilon || diff < -epsilon {
				t.Errorf("red = %f, want %f", r, tt.wantRed)
			}
			if diff := g - tt.wantGreen; diff > epsilon || diff < -epsilon {
				t.Errorf("green = %f, want %f", g, tt.wantGreen)
			}
			if diff := b - tt.wantBlue; diff > epsilon || diff < -epsilon {
				t.Errorf("blue = %f, want %f", b, tt.wantBlue)
			}
		})
	}
}

// --- colorToHex ---

func TestSheetsColorToHex(t *testing.T) {
	tests := []struct {
		name string
		c    *sheets.Color
		want string
	}{
		{name: "nil color", c: nil, want: ""},
		{name: "pure red", c: &sheets.Color{Red: 1.0, Green: 0, Blue: 0}, want: "#FF0000"},
		{name: "pure green", c: &sheets.Color{Red: 0, Green: 1.0, Blue: 0}, want: "#00FF00"},
		{name: "pure blue", c: &sheets.Color{Red: 0, Green: 0, Blue: 1.0}, want: "#0000FF"},
		{name: "white", c: &sheets.Color{Red: 1.0, Green: 1.0, Blue: 1.0}, want: "#FFFFFF"},
		{name: "black", c: &sheets.Color{Red: 0, Green: 0, Blue: 0}, want: "#000000"},
		{name: "mid gray", c: &sheets.Color{Red: 0.5, Green: 0.5, Blue: 0.5}, want: "#808080"},
		{name: "clamp negative", c: &sheets.Color{Red: -0.5, Green: 0, Blue: 0}, want: "#000000"},
		{name: "clamp over 1", c: &sheets.Color{Red: 1.5, Green: 0, Blue: 0}, want: "#FF0000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := colorToHex(tt.c)
			if got != tt.want {
				t.Errorf("colorToHex(%+v) = %q, want %q", tt.c, got, tt.want)
			}
		})
	}
}

// --- gridRangeToA1 ---

func TestSheetsGridRangeToA1(t *testing.T) {
	titles := map[int64]string{
		0: "Sheet1",
		1: "Data",
	}

	tests := []struct {
		name   string
		gr     *sheets.GridRange
		titles map[int64]string
		want   string
	}{
		{
			name:   "no start or end returns just title",
			gr:     &sheets.GridRange{SheetId: 0},
			titles: titles,
			want:   "Sheet1",
		},
		{
			name:   "single cell A1",
			gr:     &sheets.GridRange{SheetId: 0, StartRowIndex: 0, StartColumnIndex: 0, EndRowIndex: 1, EndColumnIndex: 1},
			titles: titles,
			want:   "Sheet1!A1",
		},
		{
			name:   "range A1:B2",
			gr:     &sheets.GridRange{SheetId: 0, StartRowIndex: 0, StartColumnIndex: 0, EndRowIndex: 2, EndColumnIndex: 2},
			titles: titles,
			want:   "Sheet1!A1:B2",
		},
		{
			name:   "unknown sheetId uses fallback",
			gr:     &sheets.GridRange{SheetId: 99, StartRowIndex: 0, StartColumnIndex: 0, EndRowIndex: 1, EndColumnIndex: 1},
			titles: titles,
			want:   "Sheet 99!A1",
		},
		{
			name:   "named sheet with range",
			gr:     &sheets.GridRange{SheetId: 1, StartRowIndex: 2, StartColumnIndex: 1, EndRowIndex: 5, EndColumnIndex: 4},
			titles: titles,
			want:   "Data!B3:D5",
		},
		{
			name:   "start only no end",
			gr:     &sheets.GridRange{SheetId: 0, StartRowIndex: 3, StartColumnIndex: 2},
			titles: titles,
			want:   "Sheet1!C4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gridRangeToA1(tt.gr, tt.titles)
			if got != tt.want {
				t.Errorf("gridRangeToA1() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- summarizeConditionalRule ---

func TestSheetsSummarizeConditionalRule(t *testing.T) {
	titles := map[int64]string{0: "Sheet1"}

	tests := []struct {
		name         string
		rule         *sheets.ConditionalFormatRule
		index        int
		wantContains []string
	}{
		{
			name: "boolean rule with background color",
			rule: &sheets.ConditionalFormatRule{
				Ranges: []*sheets.GridRange{{SheetId: 0, StartRowIndex: 0, EndRowIndex: 1, EndColumnIndex: 1}},
				BooleanRule: &sheets.BooleanRule{
					Condition: &sheets.BooleanCondition{
						Type:   "NUMBER_GREATER",
						Values: []*sheets.ConditionValue{{UserEnteredValue: "100"}},
					},
					Format: &sheets.CellFormat{
						BackgroundColor: &sheets.Color{Red: 1.0, Green: 0, Blue: 0},
					},
				},
			},
			index:        0,
			wantContains: []string{"[0]", "NUMBER_GREATER", "values=[100]", "bg #FF0000"},
		},
		{
			name: "boolean rule with text color",
			rule: &sheets.ConditionalFormatRule{
				Ranges: []*sheets.GridRange{{SheetId: 0, StartRowIndex: 0, EndRowIndex: 1, EndColumnIndex: 1}},
				BooleanRule: &sheets.BooleanRule{
					Condition: &sheets.BooleanCondition{Type: "BLANK"},
					Format: &sheets.CellFormat{
						TextFormat: &sheets.TextFormat{
							ForegroundColor: &sheets.Color{Red: 0, Green: 0, Blue: 1.0},
						},
					},
				},
			},
			index:        1,
			wantContains: []string{"[1]", "BLANK", "text #0000FF"},
		},
		{
			name: "gradient rule",
			rule: &sheets.ConditionalFormatRule{
				Ranges: []*sheets.GridRange{{SheetId: 0, StartRowIndex: 0, EndRowIndex: 10, EndColumnIndex: 1}},
				GradientRule: &sheets.GradientRule{
					Minpoint: &sheets.InterpolationPoint{
						Type:  "MIN",
						Color: &sheets.Color{Red: 1.0, Green: 0, Blue: 0},
					},
					Maxpoint: &sheets.InterpolationPoint{
						Type:  "MAX",
						Color: &sheets.Color{Red: 0, Green: 1.0, Blue: 0},
					},
				},
			},
			index:        2,
			wantContains: []string{"[2]", "gradient", "MIN", "MAX", "#FF0000", "#00FF00"},
		},
		{
			name: "unknown rule type",
			rule: &sheets.ConditionalFormatRule{
				Ranges: []*sheets.GridRange{{SheetId: 0}},
			},
			index:        3,
			wantContains: []string{"[3]", "(unknown rule)"},
		},
		{
			name: "no ranges",
			rule: &sheets.ConditionalFormatRule{
				Ranges: nil,
			},
			index:        0,
			wantContains: []string{"(no range)"},
		},
		{
			name: "boolean rule no format",
			rule: &sheets.ConditionalFormatRule{
				Ranges: []*sheets.GridRange{{SheetId: 0}},
				BooleanRule: &sheets.BooleanRule{
					Condition: &sheets.BooleanCondition{Type: "NOT_BLANK"},
				},
			},
			index:        0,
			wantContains: []string{"NOT_BLANK", "no format"},
		},
		{
			name: "gradient rule with midpoint and value",
			rule: &sheets.ConditionalFormatRule{
				Ranges: []*sheets.GridRange{{SheetId: 0, EndRowIndex: 1, EndColumnIndex: 1}},
				GradientRule: &sheets.GradientRule{
					Minpoint: &sheets.InterpolationPoint{
						Type:  "NUMBER",
						Value: "0",
						Color: &sheets.Color{Red: 1.0},
					},
					Midpoint: &sheets.InterpolationPoint{
						Type:  "PERCENTILE",
						Value: "50",
						Color: &sheets.Color{Green: 1.0},
					},
					Maxpoint: &sheets.InterpolationPoint{
						Type:  "NUMBER",
						Value: "100",
						Color: &sheets.Color{Blue: 1.0},
					},
				},
			},
			index:        0,
			wantContains: []string{"gradient", "NUMBER:0", "PERCENTILE:50", "NUMBER:100"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := summarizeConditionalRule(tt.rule, tt.index, titles)
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("output missing %q\ngot: %s", want, got)
				}
			}
		})
	}
}

// --- formatConditionalRulesSection ---

func TestSheetsFormatConditionalRulesSection(t *testing.T) {
	titles := map[int64]string{0: "Sheet1"}

	tests := []struct {
		name         string
		sheetTitle   string
		rules        []*sheets.ConditionalFormatRule
		indent       string
		wantContains []string
		wantExact    string
	}{
		{
			name:       "no rules",
			sheetTitle: "Sheet1",
			rules:      nil,
			indent:     "",
			wantExact:  `Conditional formats for "Sheet1": none.`,
		},
		{
			name:       "empty rules slice",
			sheetTitle: "Data",
			rules:      []*sheets.ConditionalFormatRule{},
			indent:     "  ",
			wantExact:  `  Conditional formats for "Data": none.`,
		},
		{
			name:       "single rule",
			sheetTitle: "Sheet1",
			rules: []*sheets.ConditionalFormatRule{
				{
					Ranges: []*sheets.GridRange{{SheetId: 0}},
					BooleanRule: &sheets.BooleanRule{
						Condition: &sheets.BooleanCondition{Type: "NOT_BLANK"},
					},
				},
			},
			indent: "",
			wantContains: []string{
				`Conditional formats for "Sheet1" (1):`,
				"[0]",
				"NOT_BLANK",
			},
		},
		{
			name:       "multiple rules",
			sheetTitle: "Sheet1",
			rules: []*sheets.ConditionalFormatRule{
				{
					Ranges:      []*sheets.GridRange{{SheetId: 0}},
					BooleanRule: &sheets.BooleanRule{Condition: &sheets.BooleanCondition{Type: "BLANK"}},
				},
				{
					Ranges:      []*sheets.GridRange{{SheetId: 0}},
					BooleanRule: &sheets.BooleanRule{Condition: &sheets.BooleanCondition{Type: "NOT_BLANK"}},
				},
			},
			indent: "  ",
			wantContains: []string{
				`Conditional formats for "Sheet1" (2):`,
				"[0]",
				"BLANK",
				"[1]",
				"NOT_BLANK",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatConditionalRulesSection(tt.sheetTitle, tt.rules, titles, tt.indent)
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
		})
	}
}

// --- selectSheet ---

func TestSheetsSelectSheet(t *testing.T) {
	infos := []sheetInfo{
		{SheetID: 0, Title: "First"},
		{SheetID: 1, Title: "Second"},
		{SheetID: 2, Title: "Third"},
	}

	tests := []struct {
		name        string
		infos       []sheetInfo
		sheetName   string
		wantTitle   string
		wantErr     bool
		errContains string
	}{
		{
			name:      "empty name returns first sheet",
			infos:     infos,
			sheetName: "",
			wantTitle: "First",
		},
		{
			name:      "exact match by name",
			infos:     infos,
			sheetName: "Second",
			wantTitle: "Second",
		},
		{
			name:      "last sheet by name",
			infos:     infos,
			sheetName: "Third",
			wantTitle: "Third",
		},
		{
			name:        "not found returns error",
			infos:       infos,
			sheetName:   "NonExistent",
			wantErr:     true,
			errContains: "not found",
		},
		{
			name:        "empty infos returns error",
			infos:       []sheetInfo{},
			sheetName:   "",
			wantErr:     true,
			errContains: "no sheets",
		},
		{
			name:        "empty infos with name returns error",
			infos:       []sheetInfo{},
			sheetName:   "Sheet1",
			wantErr:     true,
			errContains: "no sheets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectSheet(tt.infos, tt.sheetName)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Title != tt.wantTitle {
				t.Errorf("got Title = %q, want %q", got.Title, tt.wantTitle)
			}
		})
	}
}

// --- parseConditionValues ---

func TestSheetsParseConditionValues(t *testing.T) {
	tests := []struct {
		name    string
		raw     any
		want    []string
		wantErr bool
	}{
		{
			name: "nil returns nil",
			raw:  nil,
			want: nil,
		},
		{
			name: "JSON string array",
			raw:  `["100", "200"]`,
			want: []string{"100", "200"},
		},
		{
			name: "JSON number array",
			raw:  `[10, 20]`,
			want: []string{"10", "20"},
		},
		{
			name: "[]any of strings",
			raw:  []any{"foo", "bar"},
			want: []string{"foo", "bar"},
		},
		{
			name: "[]any of mixed types",
			raw:  []any{"text", 42, true},
			want: []string{"text", "42", "true"},
		},
		{
			name: "fallback single int value",
			raw:  42,
			want: []string{"42"},
		},
		{
			name: "fallback single bool value",
			raw:  true,
			want: []string{"true"},
		},
		{
			name:    "invalid JSON string",
			raw:     "not-json",
			wantErr: true,
		},
		{
			name:    "JSON string but not array",
			raw:     `"single"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseConditionValues(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.want == nil {
				if got != nil {
					t.Errorf("got %v, want nil", got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v (len %d), want %v (len %d)", got, len(got), tt.want, len(tt.want))
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

// --- parseGradientPoints ---

func TestSheetsParseGradientPoints(t *testing.T) {
	tests := []struct {
		name        string
		raw         any
		wantLen     int
		wantErr     bool
		errContains string
		check       func(t *testing.T, pts []gradientPoint)
	}{
		{
			name:    "nil returns nil",
			raw:     nil,
			wantLen: 0,
		},
		{
			name: "valid 2 points from []any",
			raw: []any{
				map[string]any{"type": "MIN", "color": "#FF0000"},
				map[string]any{"type": "MAX", "color": "#00FF00"},
			},
			wantLen: 2,
			check: func(t *testing.T, pts []gradientPoint) {
				if pts[0].Type != "MIN" {
					t.Errorf("pts[0].Type = %q, want MIN", pts[0].Type)
				}
				if pts[1].Type != "MAX" {
					t.Errorf("pts[1].Type = %q, want MAX", pts[1].Type)
				}
			},
		},
		{
			name: "valid 3 points with value",
			raw: []any{
				map[string]any{"type": "NUMBER", "color": "#FF0000", "value": "0"},
				map[string]any{"type": "PERCENTILE", "color": "#FFFF00", "value": "50"},
				map[string]any{"type": "NUMBER", "color": "#00FF00", "value": "100"},
			},
			wantLen: 3,
			check: func(t *testing.T, pts []gradientPoint) {
				if pts[0].Value != "0" {
					t.Errorf("pts[0].Value = %q, want 0", pts[0].Value)
				}
				if pts[1].Value != "50" {
					t.Errorf("pts[1].Value = %q, want 50", pts[1].Value)
				}
				if pts[2].Value != "100" {
					t.Errorf("pts[2].Value = %q, want 100", pts[2].Value)
				}
			},
		},
		{
			name:    "valid JSON string",
			raw:     `[{"type":"MIN","color":"#FF0000"},{"type":"MAX","color":"#00FF00"}]`,
			wantLen: 2,
		},
		{
			name:        "too few points",
			raw:         []any{map[string]any{"type": "MIN", "color": "#FF0000"}},
			wantErr:     true,
			errContains: "2 or 3",
		},
		{
			name: "too many points",
			raw: []any{
				map[string]any{"type": "MIN", "color": "#FF0000"},
				map[string]any{"type": "PERCENT", "color": "#FFFF00"},
				map[string]any{"type": "PERCENT", "color": "#00FFFF"},
				map[string]any{"type": "MAX", "color": "#00FF00"},
			},
			wantErr:     true,
			errContains: "2 or 3",
		},
		{
			name: "invalid type",
			raw: []any{
				map[string]any{"type": "INVALID", "color": "#FF0000"},
				map[string]any{"type": "MAX", "color": "#00FF00"},
			},
			wantErr:     true,
			errContains: "type must be one of",
		},
		{
			name: "missing color",
			raw: []any{
				map[string]any{"type": "MIN"},
				map[string]any{"type": "MAX", "color": "#00FF00"},
			},
			wantErr:     true,
			errContains: "color is required",
		},
		{
			name: "invalid color format",
			raw: []any{
				map[string]any{"type": "MIN", "color": "bad"},
				map[string]any{"type": "MAX", "color": "#00FF00"},
			},
			wantErr:     true,
			errContains: "color is required",
		},
		{
			name: "non-object item",
			raw: []any{
				"not-an-object",
				map[string]any{"type": "MAX", "color": "#00FF00"},
			},
			wantErr:     true,
			errContains: "must be an object",
		},
		{
			name:        "invalid JSON string",
			raw:         "not-valid-json",
			wantErr:     true,
			errContains: "must be a list",
		},
		{
			name:        "non-list type",
			raw:         42,
			wantErr:     true,
			errContains: "must be a list",
		},
		{
			name: "lowercase type accepted",
			raw: []any{
				map[string]any{"type": "min", "color": "#FF0000"},
				map[string]any{"type": "max", "color": "#00FF00"},
			},
			wantLen: 2,
			check: func(t *testing.T, pts []gradientPoint) {
				if pts[0].Type != "MIN" {
					t.Errorf("pts[0].Type = %q, want MIN (uppercased)", pts[0].Type)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseGradientPoints(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.raw == nil {
				if got != nil {
					t.Errorf("got %v, want nil", got)
				}
				return
			}
			if len(got) != tt.wantLen {
				t.Fatalf("got %d points, want %d", len(got), tt.wantLen)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

// --- gradientPointToInterpolation ---

func TestSheetsGradientPointToInterpolation(t *testing.T) {
	tests := []struct {
		name  string
		pt    gradientPoint
		check func(t *testing.T, ip *sheets.InterpolationPoint)
	}{
		{
			name: "with value",
			pt: gradientPoint{
				Type:  "NUMBER",
				Value: "50",
				Color: &sheets.Color{Red: 1.0},
			},
			check: func(t *testing.T, ip *sheets.InterpolationPoint) {
				if ip.Type != "NUMBER" {
					t.Errorf("Type = %q, want NUMBER", ip.Type)
				}
				if ip.Value != "50" {
					t.Errorf("Value = %q, want 50", ip.Value)
				}
				if ip.Color == nil || ip.Color.Red != 1.0 {
					t.Error("Color not set correctly")
				}
			},
		},
		{
			name: "without value",
			pt: gradientPoint{
				Type:  "MIN",
				Value: "",
				Color: &sheets.Color{Green: 1.0},
			},
			check: func(t *testing.T, ip *sheets.InterpolationPoint) {
				if ip.Type != "MIN" {
					t.Errorf("Type = %q, want MIN", ip.Type)
				}
				if ip.Value != "" {
					t.Errorf("Value = %q, want empty", ip.Value)
				}
				if ip.Color == nil || ip.Color.Green != 1.0 {
					t.Error("Color not set correctly")
				}
			},
		},
		{
			name: "nil color passes through",
			pt: gradientPoint{
				Type:  "MAX",
				Value: "100",
				Color: nil,
			},
			check: func(t *testing.T, ip *sheets.InterpolationPoint) {
				if ip.Type != "MAX" {
					t.Errorf("Type = %q, want MAX", ip.Type)
				}
				if ip.Color != nil {
					t.Error("expected nil Color")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gradientPointToInterpolation(tt.pt)
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

// --- sortedKeys ---

func TestSheetsSortedKeys(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]bool
		want []string
	}{
		{
			name: "nil map",
			m:    nil,
			want: []string{},
		},
		{
			name: "empty map",
			m:    map[string]bool{},
			want: []string{},
		},
		{
			name: "single key",
			m:    map[string]bool{"alpha": true},
			want: []string{"alpha"},
		},
		{
			name: "multiple keys sorted",
			m:    map[string]bool{"charlie": true, "alpha": true, "bravo": true},
			want: []string{"alpha", "bravo", "charlie"},
		},
		{
			name: "keys with false values still included",
			m:    map[string]bool{"b": false, "a": true, "c": false},
			want: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortedKeys(tt.m)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v (len %d), want %v (len %d)", got, len(got), tt.want, len(tt.want))
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}
