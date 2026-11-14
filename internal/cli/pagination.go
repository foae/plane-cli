package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
)

func nextPage(data json.RawMessage, kind string, query url.Values) (url.Values, bool, error) {
	var page map[string]json.RawMessage
	invalid := errors.New("invalid or nonprogressing pagination continuation")
	if json.Unmarshal(data, &page) != nil || page == nil {
		return nil, false, invalid
	}
	next := url.Values{}
	for k, v := range query {
		next[k] = append([]string(nil), v...)
	}
	if kind == "v1" {
		more, valid := continuationBool(page["next_page_results"])
		if !valid {
			return nil, false, invalid
		}
		if !more {
			return nil, false, nil
		}
		var cursor string
		if json.Unmarshal(page["next_cursor"], &cursor) != nil || cursor == "" || cursor == query.Get("cursor") {
			return nil, false, invalid
		}
		next.Set("cursor", cursor)
	} else {
		var pagination struct {
			Style string `json:"style"`
		}
		if json.Unmarshal(page["pagination"], &pagination) != nil {
			return nil, false, invalid
		}
		switch pagination.Style {
		case "offset":
			raw, exists := page["next"]
			if !exists {
				return nil, false, invalid
			}
			if string(raw) == "null" {
				return nil, false, nil
			}
			var offset int
			if json.Unmarshal(raw, &offset) != nil || offset < 0 {
				return nil, false, invalid
			}
			current := 0
			if query.Get("offset") != "" {
				var err error
				current, err = strconv.Atoi(query.Get("offset"))
				if err != nil {
					return nil, false, invalid
				}
			}
			if offset <= current {
				return nil, false, invalid
			}
			if offset > 10000 {
				return nil, false, errors.New("upstream offset limit reached; use explicit cursor pagination")
			}
			next.Set("offset", strconv.Itoa(offset))
		case "cursor":
			more, valid := continuationBool(page["has_more"])
			if !valid {
				return nil, false, invalid
			}
			if !more {
				return nil, false, nil
			}
			var cursor string
			if json.Unmarshal(page["next_cursor"], &cursor) != nil || cursor == "" || cursor == query.Get("cursor") {
				return nil, false, invalid
			}
			next.Set("cursor", cursor)
		default:
			return nil, false, invalid
		}
	}
	return next, true, nil
}

func continuationBool(raw json.RawMessage) (bool, bool) {
	switch string(bytes.TrimSpace(raw)) {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
	}
}
