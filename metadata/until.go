package metadata

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type Until struct {
	Raw            string
	Condition      *Assertion
	CurrentAttempt int
	MaxRetry       int
	Interval       time.Duration
	// Finish         bool
}

func NewUntil(line string) (*Until, error) {
	parts := strings.Fields(line)
	m := make(map[string]int)

	for i, v := range parts {
		m[v] = i
	}

	maxIdx, ok := m["max"]
	if !ok {
		return nil, fmt.Errorf("syntax error: until <condition> max <int> [interval <duration>]")
	}
	if len(parts) < maxIdx+2 {
		return nil, fmt.Errorf("syntax error: until <condition> max <int> [interval <duration>]")
	}
	maxN, err := strconv.Atoi(parts[maxIdx+1])
	if err != nil {
		return nil, fmt.Errorf("syntax error: max value should be integer")
	}

	intervalN := time.Second
	intervalIdx, ok := m["interval"]
	if ok && len(parts) >= intervalIdx+2 {
		intervalN, err = time.ParseDuration(parts[intervalIdx+1])
		if err != nil {
			n, err := strconv.Atoi(parts[intervalIdx+1])
			if err != nil {
				return nil, fmt.Errorf("syntax error: interval value should be duration string")
			}
			intervalN = time.Duration(n) * time.Second
		}

		if intervalN < 0 {
			return nil, fmt.Errorf("duration should be positive")
		}
	}

	idx := maxIdx
	if intervalIdx != 0 && min(maxIdx, intervalIdx) == intervalIdx {
		idx = intervalIdx
	}
	cond, err := NewAssertion(strings.Join(parts[0:idx], " "))
	if err != nil {
		return nil, err
	}

	return &Until{
		Raw:       line,
		Condition: cond,
		MaxRetry:  maxN,
		Interval:  intervalN,
	}, nil

}

func (u *Until) IsFinish() bool {
	// condition ok: left == right になったので終わり
	// attempt > max retry: 最大リトライ回数まで実行したので終わり
	// attempt == 0: エラーで until の処理が行われなかったら attempt == 0 になる。終わり判定にする
	return u.Condition.Ok() || u.CurrentAttempt >= u.MaxRetry || u.CurrentAttempt == 0
}
