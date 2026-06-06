package adapter

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/vanducng/miu-db/internal/config"
	"github.com/vanducng/miu-db/internal/result"
)

type ScriptOptions struct {
	Atomic bool
}

// ScriptRunner is implemented by providers that can execute a multi-statement
// script and return one result set per statement. Capability opt-in (analog of
// SessionConfigurable); providers without it are rejected before execution.
// The runner owns its full connection lifecycle (the dispatcher does NOT
// pre-open) so a provider can use a dedicated pool — e.g. MySQL needs a scoped
// multiStatements connection plus its own tunnel.
type ScriptRunner interface {
	RunScript(ctx context.Context, conn config.Connection, script string, limit int, opts ScriptOptions) (result.ScriptResult, error)
}

// UnsupportedScriptError is returned (before execution) when the target
// datasource cannot run multi-statement scripts.
type UnsupportedScriptError struct {
	DBType string
	Reason string
}

func (e *UnsupportedScriptError) Error() string {
	if e.Reason == "" {
		return fmt.Sprintf("datasource %q does not support multi-statement scripts", e.DBType)
	}
	return fmt.Sprintf("datasource %q does not support multi-statement scripts: %s", e.DBType, e.Reason)
}

// ScriptUnsupporter lets a non-script provider supply a specific rejection reason.
type ScriptUnsupporter interface {
	ScriptUnsupportedReason() string
}

// ResolveScriptRunner returns the provider's ScriptRunner, or a typed
// UnsupportedScriptError (with a per-provider reason if available) — strict
// opt-in, no generic database/sql fallback (a fallback would let drivers that
// silently drop result sets, e.g. sqlite/postgres, return wrong answers).
func ResolveScriptRunner(p Provider) (ScriptRunner, error) {
	if r, ok := p.(ScriptRunner); ok {
		return r, nil
	}
	reason := ""
	if u, ok := p.(ScriptUnsupporter); ok {
		reason = u.ScriptUnsupportedReason()
	}
	return nil, &UnsupportedScriptError{DBType: p.Type(), Reason: reason}
}

// RunScriptSQL executes a multi-statement script over a database/sql connection
// and collects one StatementResult per result set. The caller supplies ctx
// (Snowflake wraps it with WithMultiStatement; MySQL passes a multiStatements
// pool) and it honors ONLY the caller's ctx for timeouts — no extra hardcoded
// deadline, because a whole batch shares one budget.
//
// Mid-batch failure is provider-shaped and honest: streaming drivers (MySQL)
// keep the collected prefix and report the real failing index; batching drivers
// (Snowflake) return the error with no prefix and index -1 (unavailable).
// A statement failure is carried in ScriptResult.Errors, not the Go error —
// the Go error is reserved for infrastructure failures (begin/commit).
func RunScriptSQL(ctx context.Context, db *sql.DB, script string, limit int, opts ScriptOptions) (result.ScriptResult, error) {
	if limit <= 0 {
		limit = 100
	}

	var (
		rows *sql.Rows
		err  error
		tx   *sql.Tx
	)
	if opts.Atomic {
		if tx, err = db.BeginTx(ctx, nil); err != nil {
			return result.ScriptResult{}, err
		}
		rows, err = tx.QueryContext(ctx, script)
	} else {
		rows, err = db.QueryContext(ctx, script)
	}
	if err != nil {
		if tx != nil {
			_ = tx.Rollback()
		}
		// Whole batch failed before any result set surfaced (Snowflake typical):
		// the failing statement index is not recoverable -> -1.
		return scriptError(-1, err), nil
	}
	defer rows.Close()

	sr := result.ScriptResult{Statements: []result.StatementResult{}}
	idx := 0
	var loopErr error
	for {
		stmt, cerr := collectStatement(rows, idx, limit)
		if cerr != nil {
			loopErr = cerr
			break
		}
		sr.Statements = append(sr.Statements, stmt)
		idx++
		if !rows.NextResultSet() {
			break
		}
	}
	if loopErr == nil {
		loopErr = rows.Err() // driver error stashed across NextResultSet/Scan
	}
	if loopErr != nil {
		if tx != nil {
			_ = tx.Rollback()
			sr.Statements = nil // atomic: all-or-nothing
		}
		// idx == len(Statements) in both break paths == the index of the first
		// statement absent from results; for a streamed mid-batch failure that is
		// the failing statement. Batched drivers abort before any set and take the
		// scriptError(-1) path above instead.
		sr.Errors = []result.StatementError{statementError(idx, loopErr)}
		return sr, nil
	}
	if tx != nil {
		if cerr := tx.Commit(); cerr != nil {
			return result.ScriptResult{}, cerr
		}
	}
	return sr, nil
}

func collectStatement(rows *sql.Rows, index, limit int) (result.StatementResult, error) {
	colNames, err := rows.Columns() // re-read per result set: column count varies
	if err != nil {
		return result.StatementResult{}, err
	}
	colTypes, _ := rows.ColumnTypes()
	columns := make([]result.Column, len(colNames))
	for i, name := range colNames {
		typ := ""
		nullable := false
		if i < len(colTypes) {
			typ = colTypes[i].DatabaseTypeName()
			if n, ok := colTypes[i].Nullable(); ok {
				nullable = n
			}
		}
		columns[i] = result.Column{Name: name, Type: typ, Nullable: nullable}
	}

	outRows := make([][]any, 0, limit)
	truncated := false
	count := 0
	for rows.Next() {
		if count >= limit {
			// Leaves the set un-drained; database/sql discards the remainder on the
			// next NextResultSet (same early-stop pattern as QuerySQL).
			truncated = true
			break
		}
		values := make([]any, len(columns)) // realloc scan buffers to THIS set's width
		scan := make([]any, len(columns))
		for i := range values {
			scan[i] = &values[i]
		}
		if err := rows.Scan(scan...); err != nil {
			return result.StatementResult{}, err
		}
		outRows = append(outRows, normalizeRow(values))
		count++
	}

	// Structural classification: data columns -> "rows"; zero columns (e.g. a
	// MySQL DML OK-packet) -> "exec". Snowflake surfaces non-returning statements
	// as a 1-column status set, which honestly shows as "rows".
	kind := "exec"
	if len(columns) > 0 {
		kind = "rows"
	}
	return result.StatementResult{
		Index:     index,
		Kind:      kind,
		Result:    &result.QueryResult{Columns: columns, Rows: outRows, Truncated: truncated},
		RowCount:  len(outRows),
		Truncated: truncated,
	}, nil
}

func scriptError(index int, err error) result.ScriptResult {
	return result.ScriptResult{
		Statements: []result.StatementResult{},
		Errors:     []result.StatementError{statementError(index, err)},
	}
}

func statementError(index int, err error) result.StatementError {
	return result.StatementError{
		Index:   index,
		Code:    "query.statement_failed",
		Message: config.RedactString(err.Error()),
	}
}
