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

// Taille maximale d'une valeur produite par la console (SQLITE_LIMIT_LENGTH).
const maxValeurConsole = 4 << 20

// PRAGMA lisibles depuis la console. Les premiers prennent un nom de table ou d'index en argument ;
// les seconds ne sont autorisés qu'en lecture (« PRAGMA user_version », sans « = »).
var (
	pragmasAvecArgument = map[string]bool{
		"table_info": true, "table_xinfo": true, "table_list": true, "index_list": true,
		"index_info": true, "index_xinfo": true, "foreign_key_list": true,
	}
	pragmasSansArgument = map[string]bool{
		"database_list": true, "compile_options": true, "function_list": true, "pragma_list": true,
		"collation_list": true, "module_list": true, "user_version": true, "schema_version": true,
		"application_id": true, "page_count": true, "page_size": true, "freelist_count": true,
		"encoding": true, "foreign_keys": true, "query_only": true, "journal_mode": true,
	}
)

// autoriserConsole est la vraie barrière de la console : SQLite l'appelle pour chaque instruction
// compilée, commentaires et instructions multiples compris. Seules les lectures passent ; ATTACH,
// les PRAGMA qui modifient la connexion, les transactions et toute écriture sont refusés.
func autoriserConsole(action sqlite.AuthorizerActionCode, arg1, arg2, _, _ string) sqlite.AuthorizerReturnCode {
	switch action {
	case sqlite.AuthSelect, sqlite.AuthRead, sqlite.AuthFunction, sqlite.AuthRecursive:
		return sqlite.AuthorizerOK
	case sqlite.AuthPragma:
		nom := strings.ToLower(arg1)
		if pragmasAvecArgument[nom] || (arg2 == "" && pragmasSansArgument[nom]) {
			return sqlite.AuthorizerOK
		}
	}
	return sqlite.AuthorizerDeny
}

// protegerConnexion installe la politique de la console sur cette connexion physique et surveille ctx :
// à son expiration, la requête est interrompue même pendant le parcours des lignes, ce que le pilote
// ne fait que pendant QueryContext. La fonction renvoyée arrête la surveillance ; l'appeler avant de
// rendre la connexion au pool.
func protegerConnexion(ctx context.Context, conn *sql.Conn) (func(), error) {
	var poignee uintptr
	err := conn.Raw(func(pilote any) error {
		reg, ok := pilote.(sqlite.AuthorizerRegisterer)
		if !ok {
			return errors.New("pilote SQLite sans authorizer")
		}
		if err := reg.RegisterAuthorizer(autoriserConsole); err != nil {
			return err
		}
		// Le pilote n'exporte pas sqlite3_interrupt : on lit la poignée de la connexion (champ « db »).
		v := reflect.ValueOf(pilote)
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		if v.Kind() == reflect.Struct {
			if f := v.FieldByName("db"); f.IsValid() && f.Kind() == reflect.Uintptr {
				poignee = uintptr(f.Uint())
			}
		}
		if poignee == 0 {
			return errors.New("poignée SQLite introuvable : interruption impossible")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if _, err := sqlite.Limit(conn, sqlite3.SQLITE_LIMIT_LENGTH, maxValeurConsole); err != nil {
		return nil, err
	}

	arret := make(chan struct{})
	var fini sync.WaitGroup
	fini.Add(1)
	go func() {
		defer fini.Done()
		select {
		case <-ctx.Done():
			tls := libc.NewTLS()
			sqlite3.Xsqlite3_interrupt(tls, poignee)
			tls.Close()
		case <-arret:
		}
	}()
	// Une interruption arrivée après la fin de la requête est effacée par SQLite au début de la suivante
	// (aucune instruction active) : elle ne peut pas toucher l'utilisateur suivant de la connexion.
	return func() { close(arret); fini.Wait() }, nil
}
