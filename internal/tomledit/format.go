package tomledit

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func formatValue(v any, newline string, multiline bool) (string, error) {
	switch x := v.(type) {
	case string:
		if strings.ContainsAny(x, "\r\n") {
			return "'''" + x + "'''", nil
		}
		return strconv.Quote(x), nil
	case bool:
		return strconv.FormatBool(x), nil
	case int:
		return strconv.FormatInt(int64(x), 10), nil
	case int8:
		return strconv.FormatInt(int64(x), 10), nil
	case int16:
		return strconv.FormatInt(int64(x), 10), nil
	case int32:
		return strconv.FormatInt(int64(x), 10), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case uint:
		return strconv.FormatUint(uint64(x), 10), nil
	case uint8:
		return strconv.FormatUint(uint64(x), 10), nil
	case uint16:
		return strconv.FormatUint(uint64(x), 10), nil
	case uint32:
		return strconv.FormatUint(uint64(x), 10), nil
	case uint64:
		return strconv.FormatUint(x, 10), nil
	case float32:
		return strconv.FormatFloat(float64(x), 'g', -1, 32), nil
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64), nil
	case []string:
		parts := make([]string, len(x))
		for i := range x {
			parts[i] = strconv.Quote(x[i])
		}
		return "[" + strings.Join(parts, ", ") + "]", nil
	case []any:
		parts := make([]string, len(x))
		for i := range x {
			var err error
			parts[i], err = formatValue(x[i], newline, false)
			if err != nil {
				return "", err
			}
		}
		one := "[" + strings.Join(parts, ", ") + "]"
		if !multiline && len(one) <= 80 && !strings.Contains(one, newline) {
			return one, nil
		}
		return "[" + newline + "  " + strings.Join(parts, ","+newline+"  ") + "," + newline + "]", nil
	case map[string]any:
		parts := make([]string, 0, len(x))
		for k, val := range x {
			f, err := formatValue(val, newline, false)
			if err != nil {
				return "", err
			}
			parts = append(parts, strconv.Quote(k)+" = "+f)
		}
		return "{" + strings.Join(parts, ", ") + "}", nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		if len(b) == 0 {
			return "", fmt.Errorf("%w: unsupported value %T", ErrUnsupportedLayout, v)
		}
		return string(b), nil
	}
}

func formatLike(v any, original []byte, newline string) (string, error) {
	if s, ok := v.(string); ok && len(original) >= 2 {
		raw := strings.TrimSpace(string(original))
		if strings.HasPrefix(raw, "'''") || strings.HasPrefix(raw, `"""`) || (raw[0] == '\'' && raw[len(raw)-1] == '\'') {
			if strings.HasPrefix(raw, "'''") && !strings.Contains(s, "'''") {
				return "'''" + s + "'''", nil
			}
			if raw[0] == '\'' && !strings.ContainsAny(s, "'\r\n") {
				return "'" + s + "'", nil
			}
		}
	}
	return formatValue(v, newline, strings.Contains(string(original), newline))
}
