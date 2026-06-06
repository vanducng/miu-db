package protocol

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/vanducng/miu-db/internal/adapter"
	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/core"
	"github.com/vanducng/miu-db/internal/result"
)

type Server struct {
	Services  *core.Services
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
		for _, conn := range s.services().Connections() {
			items = append(items, config.RedactedConnection(conn))
		}
		return items, nil
	case "query.run":
		name, _ := req.Params["connection"].(string)
		sqlText, _ := req.Params["sql"].(string)
		limit := intFromParam(req.Params["limit"], 100)
		_, outcome, err := s.services().RunQuery(ctx, name, sqlText, limit)
		return outcome, err
	case "query.script":
		name, _ := req.Params["connection"].(string)
		sqlText, _ := req.Params["sql"].(string)
		limit := intFromParam(req.Params["limit"], 100)
		atomic, _ := req.Params["atomic"].(bool)
		_, sr, err := s.services().RunScript(ctx, name, sqlText, limit, adapter.ScriptOptions{Atomic: atomic})
		return sr, err
	case "call.fetch_page":
		cursor, _ := req.Params["cursor"].(string)
		return s.services().FetchPage(cursor)
	default:
		return nil, fmt.Errorf("unknown method %q", req.Method)
	}
}

func (s *Server) services() *core.Services {
	if s.Services != nil {
		return s.Services
	}
	return &core.Services{
		Store:     s.Store,
		Registry:  s.Registry,
		PageStore: s.PageStore,
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
