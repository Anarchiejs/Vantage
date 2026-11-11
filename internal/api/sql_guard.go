package api

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"sync"

	"modernc.org/libc"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

// Maximum size of a value produced by the console (SQLITE_LIMIT_LENGTH).
const maxConsoleValue = 4 << 20

// PRAGMAs the console may read. The first group takes a table or index name;
// the second group is allowed only as a read ("PRAGMA user_version", without "=").
var (
	pragmasWithArgument = map[string]bool{
		"table_info": true, "table_xinfo": true, "table_list": true, "index_list": true,
		"index_info": true, "index_xinfo": true, "foreign_key_list": true,
	}
	pragmasWithoutArgument = map[string]bool{
		"database_list": true, "compile_options": true, "function_list": true, "pragma_list": true,
		"collation_list": true, "module_list": true, "user_version": true, "schema_version": true,
		"application_id": true, "page_count": true, "page_size": true, "freelist_count": true,
		"encoding": true, "foreign_keys": true, "query_only": true, "journal_mode": true,
	}
)

// allowConsole is the console's real barrier: SQLite calls it for every compiled statement,
// including comments and multiple statements. Only reads pass; ATTACH, PRAGMAs that change
// the connection, transactions, and every write are refused.
func allowConsole(action sqlite.AuthorizerActionCode, arg1, arg2, _, _ string) sqlite.AuthorizerReturnCode {
	switch action {
	case sqlite.AuthSelect, sqlite.AuthRead, sqlite.AuthFunction, sqlite.AuthRecursive:
		return sqlite.AuthorizerOK
	case sqlite.AuthPragma:
		nom := strings.ToLower(arg1)
		if pragmasWithArgument[nom] || (arg2 == "" && pragmasWithoutArgument[nom]) {
			return sqlite.AuthorizerOK
		}
	}
	return sqlite.AuthorizerDeny
}

// protectConnection installs the console policy on this physical connection and watches ctx.
// When ctx expires, the query is interrupted even while scanning rows, which the driver
// only does during QueryContext. The returned function stops the watch; call it before
// returning the connection to the pool.
func protectConnection(ctx context.Context, conn *sql.Conn) (func(), error) {
	var handle uintptr
	err := conn.Raw(func(driver any) error {
		reg, ok := driver.(sqlite.AuthorizerRegisterer)
		if !ok {
			return errors.New("SQLite driver has no authorizer")
		}
		if err := reg.RegisterAuthorizer(allowConsole); err != nil {
			return err
		}
		// The driver does not export sqlite3_interrupt: read the connection handle (field "db").
		v := reflect.ValueOf(driver)
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		if v.Kind() == reflect.Struct {
			if f := v.FieldByName("db"); f.IsValid() && f.Kind() == reflect.Uintptr {
				handle = uintptr(f.Uint())
			}
		}
		if handle == 0 {
			return errors.New("SQLite handle not found: cannot interrupt")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if _, err := sqlite.Limit(conn, sqlite3.SQLITE_LIMIT_LENGTH, maxConsoleValue); err != nil {
		return nil, err
	}

	stop := make(chan struct{})
	var done sync.WaitGroup
	done.Add(1)
	go func() {
		defer done.Done()
		select {
		case <-ctx.Done():
			tls := libc.NewTLS()
			sqlite3.Xsqlite3_interrupt(tls, handle)
			tls.Close()
		case <-stop:
		}
	}()
	// An interrupt that arrives after the query ends is cleared by SQLite at the start of the next one
	// (no active statement): it cannot affect the connection's next user.
	return func() { close(stop); done.Wait() }, nil
}
