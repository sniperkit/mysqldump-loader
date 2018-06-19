package mysqldump_loader

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type Executor struct {
	db       *sql.DB
	loader   *loader
	replacer *replacer
	scanner  *scanner
}

func NewExecutor(db *sql.DB, loader *loader, scanner *scanner, replacer *replacer) *Executor {
	return &Executor{db: db, loader: loader, scanner: scanner, replacer: replacer}
}

func (e *Executor) Execute() error {
	var charset, database, table, tempTable string

	conn, err := e.db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()

	for e.scanner.scan() {
		q := e.scanner.query()
		if e.replacer != nil && q.isDropTableStatement() {
			continue
		} else if e.replacer != nil && q.isCreateTableStatement() {
			t, i, err := parseIdentifier(q.s, len("CREATE TABLE "), " ")
			if err != nil {
				return err
			}

			if table != "" {
				wg := e.loader.waitGroup(tempTable)
				if err := e.replacer.execute(context.Background(), database, tempTable, table, wg); err != nil {
					return err
				}
			}
			table = t
			tempTable = "_" + t + "_temp"
			if err := createTable(context.Background(), conn, database, tempTable, q.s[i:]); err != nil {
				return err
			}
		} else if q.isAlterTableStatement() || q.isLockTablesStatement() || q.isUnlockTablesStatement() {
			continue
		} else if e.replacer != nil && q.isInsertStatement() || q.isReplaceStatement() {
			i := strings.Index(q.s, "INTO ")
			if i == -1 {
				return fmt.Errorf("unsupported statement. line=%d", q.line)
			}
			t, _, err := parseIdentifier(q.s, i+5, " ")
			if err != nil {
				return err
			}

			if table != t {
				if table != "" {
					wg := e.loader.waitGroup(tempTable)
					if err := e.replacer.execute(context.Background(), database, tempTable, table, wg); err != nil {
						return err
					}
				}
				table = t
				tempTable = "_" + t + "_temp"
				if err := createTableLike(context.Background(), conn, database, tempTable, table); err != nil {
					return err
				}
			}
			if err := e.loader.execute(context.Background(), q, charset, database, tempTable); err != nil {
				return err
			}
		} else if q.isInsertStatement() || q.isReplaceStatement() {
			if err := e.loader.execute(context.Background(), q, charset, database, ""); err != nil {
				return err
			}
		} else {
			if _, err := conn.ExecContext(context.Background(), q.s); err != nil {
				return err
			}
			if q.isSetNamesStatement() {
				if charset, err = parseSetNamesStatement(q); err != nil {
					return err
				}
			}
			if q.isUseStatement() {
				if database, err = parseUseStatement(q); err != nil {
					return err
				}
			}
		}
	}

	if e.replacer != nil && table != "" {
		wg := e.loader.waitGroup(tempTable)
		if err := e.replacer.execute(context.Background(), database, tempTable, table, wg); err != nil {
			return err
		}
	}

	if err := e.scanner.err(); err != nil {
		return err
	}

	if err := e.loader.wait(); err != nil {
		return err
	}

	if e.replacer != nil {
		if err := e.replacer.wait(); err != nil {
			return err
		}
	}

	return nil
}
