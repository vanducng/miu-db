package protocol

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/query"
	"github.com/vanducng/miu-db/internal/result"
)

type Server struct {
	Store     *config.Store
	Registry  *adapter.Registry
	PageStore *result.PageStore
	Protocol  string
}

func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	scanner := bufio.NewScanner(in)
	enc := json.NewEncoder(out)
	for scanner.Scan() {
		var req Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			_ = enc.Encode(s.response(nil, nil, err))
			continue
		}
		result, err := s.handle(ctx, req)
		if err != nil {
			_ = enc.Encode(s.response(req.ID, nil, err))
			continue
		}
		if err := enc.Encode(s.response(req.ID, result, nil)); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (s *Server) handle(ctx context.Context, req Request) (any, error) {
	switch req.Method {
	case "server.info":
		return map[string]any{"name": "miudb", "protocol": s.Protocol}, nil
	case "connection.list":
		items := []any{}
		for _, conn := range s.Store.Connections() {
			items = append(items, config.RedactedConnection(conn))
		}
		return items, nil
	case "query.run":
		name, _ := req.Params["connection"].(string)
		sqlText, _ := req.Params["sql"].(string)
		limit := intFromParam(req.Params["limit"], 100)
		conn, ok, err := s.Store.FindResolved(name)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, fmt.Errorf("connection %q not found", name)
		}
		service := query.Service{Registry: s.Registry, PageStore: s.PageStore}
		return service.Run(ctx, conn, sqlText, limit)
	case "call.fetch_page":
		cursor, _ := req.Params["cursor"].(string)
		return s.PageStore.Fetch(cursor)
	default:
		return nil, fmt.Errorf("unknown method %q", req.Method)
	}
}

func (s *Server) response(id any, result any, err error) Response {
	resp := Response{ID: id}
	if s.Protocol == "jsonrpc" {
		resp.JSONRPC = "2.0"
	}
	if err != nil {
		resp.Error = &ErrorReply{Code: -32000, Message: config.RedactString(err.Error())}
		return resp
	}
	resp.Result = result
	return resp
}

func (r Response) MarshalJSON() ([]byte, error) {
	type alias Response
	if r.JSONRPC == "" {
		frame := map[string]any{"type": "response", "id": r.ID}
		if r.Error != nil {
			frame["type"] = "error"
			frame["error"] = r.Error
		} else {
			frame["result"] = r.Result
		}
		return json.Marshal(frame)
	}
	return json.Marshal(alias(r))
}

func intFromParam(value any, fallback int) int {
	switch v := value.(type) {
	case float64:
		if v > 0 {
			return int(v)
		}
	case int:
		if v > 0 {
			return v
		}
	}
	return fallback
}
