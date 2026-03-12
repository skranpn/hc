package hc

import (
	"bufio"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/skranpn/hc/metadata"
)

type State int

const (
	StateIdle    State = iota // 次のリクエスト待ち
	StateHeaders              // ヘッダー読み込み中
	StateBody                 // ボディ読み込み中
)

type parser struct {
	state      State
	currentReq *HttpRequest
	requests   []HttpRequest
}

func NewParser() *parser {
	return &parser{
		state: StateIdle,
	}
}

func (p *parser) Parse(r io.Reader) ([]HttpRequest, error) {
	scanner := bufio.NewScanner(r)
	p.requests = []HttpRequest{}
	p.currentReq = nil
	p.state = StateIdle

	for scanner.Scan() {
		line := scanner.Text()
		err := p.parseLine(line)
		if err != nil {
			return nil, err
		}
	}

	if err := scanner.Err(); err != nil {
		return p.requests, err
	}

	// 最後のリクエストを追加
	if p.currentReq != nil {
		p.requests = append(p.requests, *p.currentReq)
	}

	return p.requests, nil
}

func (p *parser) parseLine(line string) error {
	trimmed := strings.TrimSpace(line)

	// ### は区切り文字
	if trimmed == "###" && p.currentReq != nil && p.currentReq.IsValid() {
		if p.currentReq.Body != "" {
			p.currentReq.Body = strings.TrimSpace(p.currentReq.Body)
		}
		p.requests = append(p.requests, *p.currentReq)
		p.currentReq = nil
		p.state = StateIdle
		return nil
	}

	// metadata は状態に関係なく処理
	if strings.HasPrefix(trimmed, "@") || strings.HasPrefix(trimmed, "# @") || strings.HasPrefix(trimmed, "// @") {
		if p.currentReq == nil {
			p.currentReq = &HttpRequest{
				Headers: make(map[string]string),
			}
		}
		err := p.handleMetadata(trimmed)
		if err != nil {
			return err
		}

		return nil
	}

	switch p.state {
	case StateIdle:
		// 空行やコメントはスキップ
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			return nil
		}

		// リクエスト行が来たら開始
		if p.isRequestLine(trimmed) {
			if p.currentReq == nil {
				p.currentReq = &HttpRequest{
					Headers: make(map[string]string),
				}
			}
			p.parseRequestLine(trimmed)
			p.state = StateHeaders
		}

	case StateHeaders:
		if trimmed == "" {
			// ヘッダのあとに空行が来たらボディの開始
			p.state = StateBody
			return nil
		}
		p.parseHeaderLine(trimmed)

	case StateBody:
		if p.currentReq.Body != "" {
			p.currentReq.Body += "\n"
		}
		p.currentReq.Body += strings.TrimSuffix(line, "\n")
	}

	return nil
}

func (p *parser) isRequestLine(line string) bool {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return false
	}
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	return slices.Contains(methods, parts[0])
}

func (p *parser) parseRequestLine(line string) {
	parts := strings.Fields(line)
	if len(parts) >= 2 {
		p.currentReq.Method = parts[0]
		p.currentReq.URL = parts[1]
	}
}

func (p *parser) parseHeaderLine(line string) {
	if colonIndex := strings.Index(line, ":"); colonIndex > 0 {
		key := strings.TrimSpace(line[:colonIndex])
		value := strings.TrimSpace(line[colonIndex+1:])
		p.currentReq.Headers[key] = value
	}
}

// @, # @, // @ で始まる行の処理
func (p *parser) handleMetadata(line string) error {

	line = strings.TrimPrefix(line, "@")
	line = strings.TrimPrefix(line, "# @")
	line = strings.TrimPrefix(line, "// @")

	switch {
	case strings.HasPrefix(line, "name "):
		// Syntax: # @name <request_name>
		name := strings.TrimPrefix(line, "name ")
		p.currentReq.Name = name

	default:
		v, err := metadata.Parse(line)
		if err != nil {
			return fmt.Errorf("failed to parse metadata, %w", err)
		}
		p.currentReq.Metadata = append(p.currentReq.Metadata, v)
	}

	return nil
}
