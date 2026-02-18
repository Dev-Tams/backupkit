package schedule

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type CronSpec struct {
	minute fieldSet
	hour   fieldSet
	dom    fieldSet
	month  fieldSet
	dow    fieldSet
}

type fieldSet struct {
	any    bool
	values map[int]struct{}
}

func ParseCronSpec(expr string) (CronSpec, error) {
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return CronSpec{}, fmt.Errorf("expected 5 fields")
	}

	minute, err := parseField(parts[0], 0, 59)
	if err != nil {
		return CronSpec{}, fmt.Errorf("minute: %w", err)
	}
	hour, err := parseField(parts[1], 0, 23)
	if err != nil {
		return CronSpec{}, fmt.Errorf("hour: %w", err)
	}
	dom, err := parseField(parts[2], 1, 31)
	if err != nil {
		return CronSpec{}, fmt.Errorf("day-of-month: %w", err)
	}
	month, err := parseField(parts[3], 1, 12)
	if err != nil {
		return CronSpec{}, fmt.Errorf("month: %w", err)
	}
	dow, err := parseField(parts[4], 0, 6)
	if err != nil {
		return CronSpec{}, fmt.Errorf("day-of-week: %w", err)
	}

	return CronSpec{
		minute: minute,
		hour:   hour,
		dom:    dom,
		month:  month,
		dow:    dow,
	}, nil
}

func (s CronSpec) Matches(t time.Time) bool {
	return s.minute.has(t.Minute()) &&
		s.hour.has(t.Hour()) &&
		s.dom.has(t.Day()) &&
		s.month.has(int(t.Month())) &&
		s.dow.has(int(t.Weekday()))
}

func (f fieldSet) has(v int) bool {
	if f.any {
		return true
	}
	_, ok := f.values[v]
	return ok
}

func parseField(token string, min, max int) (fieldSet, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return fieldSet{}, fmt.Errorf("empty field")
	}
	if token == "*" {
		return fieldSet{any: true}, nil
	}

	set := make(map[int]struct{})
	parts := strings.Split(token, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return fieldSet{}, fmt.Errorf("empty list element")
		}

		if strings.HasPrefix(part, "*/") {
			step, err := strconv.Atoi(strings.TrimPrefix(part, "*/"))
			if err != nil || step <= 0 {
				return fieldSet{}, fmt.Errorf("invalid step %q", part)
			}
			for v := min; v <= max; v += step {
				set[v] = struct{}{}
			}
			continue
		}

		if strings.Contains(part, "-") {
			ends := strings.SplitN(part, "-", 2)
			start, errA := strconv.Atoi(strings.TrimSpace(ends[0]))
			end, errB := strconv.Atoi(strings.TrimSpace(ends[1]))
			if errA != nil || errB != nil {
				return fieldSet{}, fmt.Errorf("invalid range %q", part)
			}
			if start > end || start < min || end > max {
				return fieldSet{}, fmt.Errorf("range out of bounds %q", part)
			}
			for v := start; v <= end; v++ {
				set[v] = struct{}{}
			}
			continue
		}

		v, err := strconv.Atoi(part)
		if err != nil {
			return fieldSet{}, fmt.Errorf("invalid value %q", part)
		}
		if v < min || v > max {
			return fieldSet{}, fmt.Errorf("value out of bounds %d", v)
		}
		set[v] = struct{}{}
	}

	if len(set) == 0 {
		return fieldSet{}, fmt.Errorf("no values")
	}
	return fieldSet{values: set}, nil
}
