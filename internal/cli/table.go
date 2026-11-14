package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
)

func tablePage(w io.Writer, data json.RawMessage, columns []string, header bool) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	if obj, ok := value.(map[string]any); ok {
		if rows, exists := obj["results"]; exists {
			value = rows
		} else if rows, exists := obj["data"]; exists {
			value = rows
		}
	}
	rows, ok := value.([]any)
	if !ok {
		rows = []any{value}
	}
	clean := func(text string) string {
		return strings.Map(func(r rune) rune {
			if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
				return -1
			}
			return r
		}, text)
	}
	if header {
		if _, err := fmt.Fprintln(w, strings.ToUpper(strings.Join(columns, "\t"))); err != nil {
			return err
		}
	}
	for _, row := range rows {
		obj, ok := row.(map[string]any)
		if !ok {
			return errors.New("table response must contain objects")
		}
		cells := make([]string, len(columns))
		for i, key := range columns {
			if field, exists := obj[key]; exists && field != nil {
				if text, ok := field.(string); ok {
					cells[i] = clean(text)
				} else {
					raw, err := json.Marshal(field)
					if err != nil {
						return err
					}
					cells[i] = clean(string(raw))
				}
			}
		}
		if _, err := fmt.Fprintln(w, strings.Join(cells, "\t")); err != nil {
			return err
		}
	}
	return nil
}
