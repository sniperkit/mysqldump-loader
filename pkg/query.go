package mysqldump_loader

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
)

type query struct {
	line int
	s    string
}

func (q *query) isAlterTableStatement() bool {
	return strings.HasPrefix(q.s, "/*!40000 ALTER TABLE ")
}

func (q *query) isCreateTableStatement() bool {
	return strings.HasPrefix(q.s, "CREATE TABLE ")
}

func (q *query) isDropTableStatement() bool {
	return strings.HasPrefix(q.s, "DROP TABLE ")
}

func (q *query) isInsertStatement() bool {
	return strings.HasPrefix(q.s, "INSERT ")
}

func (q *query) isLockTablesStatement() bool {
	return strings.HasPrefix(q.s, "LOCK TABLES ")
}

func (q *query) isReplaceStatement() bool {
	return strings.HasPrefix(q.s, "REPLACE ")
}

func (q *query) isSetNamesStatement() bool {
	return strings.HasPrefix(q.s, " SET NAMES ") || strings.HasPrefix(q.s, "/*!40101 SET NAMES ")
}

func (q *query) isUnlockTablesStatement() bool {
	return strings.HasPrefix(q.s, "UNLOCK TABLES ")
}

func (q *query) isUseStatement() bool {
	return strings.HasPrefix(q.s, "USE ")
}

func parseSetNamesStatement(q *query) (charset string, err error) {
	if strings.HasPrefix(q.s, "/*!40101 SET NAMES ") {
		charset, _, err = parseIdentifier(q.s, len("/*!40101 SET NAMES "), " ")
	} else {
		charset, _, err = parseIdentifier(q.s, len(" SET NAMES "), " ")
	}
	return
}

func parseUseStatement(q *query) (database string, err error) {
	database, _, err = parseIdentifier(q.s, len("USE "), ";")
	return
}

func disableForeignKeyChecks(ctx context.Context, conn *sql.Conn) error {
	_, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS=0")
	return err
}

func setCharacterSet(ctx context.Context, conn *sql.Conn, charset string) error {
	_, err := conn.ExecContext(ctx, fmt.Sprintf("SET NAMES %s", charset))
	return err
}

func convert(q *query) (*insertion, error) {
	var replace, ignore bool
	var i int
	if strings.HasPrefix(q.s, "INSERT ") {
		i = len("INSERT ")
	} else if strings.HasPrefix(q.s, "REPLACE ") {
		replace = true
		i = len("REPLACE ")
	} else {
		return nil, fmt.Errorf("unsupported statement. line=%d", q.line)
	}

	if strings.HasPrefix(q.s[i:], "IGNORE ") {
		ignore = true
		i += len("IGNORE ")
	}

	if strings.HasPrefix(q.s[i:], "INTO ") {
		i += len("INTO ")
	} else {
		return nil, fmt.Errorf("unsupported statement. line=%d", q.line)
	}

	table, i, err := parseIdentifier(q.s, i, " ")
	if err != nil {
		return nil, fmt.Errorf("failed to parse table name. err=%s, line=%d", err, q.line)
	}
	i++

	if q.s[i] == '(' {
		i++
		for {
			_, i, err = parseIdentifier(q.s, i, ",)")
			if err != nil {
				return nil, fmt.Errorf("failed to parse column name. err=%s, line=%d", err, q.line)
			}
			if q.s[i] == ')' {
				i++
				break
			}
		}
		if q.s[i] != ' ' {
			return nil, fmt.Errorf("no space character after a list of colunm names. line=%d", q.line)
		}
		i++
	}

	if strings.HasPrefix(q.s[i:], "VALUES ") {
		i += len("VALUES ")
	} else {
		return nil, fmt.Errorf("unsupported statement. line=%d", q.line)
	}

	var buf bytes.Buffer
	for {
		for {
			if q.s[i] == '(' {
				i++
			}
			if q.s[i] == '\'' {
				i++
				for {
					// TODO: NO_BACKSLASH_ESCAPES
					j := strings.IndexAny(q.s[i:], "\\\t'")
					if j == -1 {
						return nil, fmt.Errorf("column value is not enclosed. line=%d", q.line)
					}
					buf.WriteString(q.s[i : i+j])
					i += j
					if q.s[i] == '\\' {
						buf.WriteString(q.s[i : i+2])
						i += 2
					} else if q.s[i] == '\t' {
						buf.WriteString(`\t`)
						i++
					} else if strings.IndexByte(",)", q.s[i+1]) != -1 {
						i++
						break
					} else {
						return nil, fmt.Errorf("unescaped single quote. line=%d", q.line)
					}
				}
			} else if strings.HasPrefix(q.s[i:], "0x") {
				j := strings.IndexAny(q.s[i+2:], ",)")
				if j == -1 {
					return nil, fmt.Errorf("hex blob is not terminated. line=%d", q.line)
				}
				if _, err := buf.ReadFrom(hex.NewDecoder(strings.NewReader(q.s[i+2 : i+2+j]))); err != nil {
					return nil, fmt.Errorf("failed to decode hex blob. err=%s, line=%d", err, q.line)
				}
				i += 2 + j
			} else {
				j := strings.IndexAny(q.s[i:], ",)")
				if j == -1 {
					return nil, fmt.Errorf("column value is not terminated. line=%d", q.line)
				}
				s := q.s[i : i+j]
				if s == "NULL" {
					buf.WriteString(`\N`)
				} else {
					buf.WriteString(s)
				}
				i += j
			}
			if q.s[i] == ',' {
				buf.WriteByte('\t')
				i++
			} else {
				buf.WriteByte('\n')
				i++
				break
			}
		}
		if q.s[i] == ',' {
			i++
		} else if q.s[i] == ';' {
			i++
			break
		} else {
			return nil, fmt.Errorf("unexpected character '%c'. line=%d", q.s[i], q.line)
		}
	}

	return &insertion{ignore: ignore, r: &buf, replace: replace, table: table}, nil
}

func quoteName(name string) []byte {
	var i int
	buf := make([]byte, len(name)*2+2)

	buf[i] = '`'
	i++
	for j := 0; j < len(name); j++ {
		if name[j] == '`' {
			buf[i] = '`'
			i++
		}
		buf[i] = name[j]
		i++
	}
	buf[i] = '`'
	i++

	return buf[:i]
}
