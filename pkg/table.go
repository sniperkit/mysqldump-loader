package mysqldump_loader

import (
	"bytes"
	"context"
	"database/sql"
)

func dropTableIfExists(ctx context.Context, conn *sql.Conn, database, table string) error {
	var query bytes.Buffer
	query.WriteString("DROP TABLE IF EXISTS ")
	if database != "" {
		query.Write(quoteName(database))
		query.WriteByte('.')
	}
	query.Write(quoteName(table))
	_, err := conn.ExecContext(ctx, query.String())
	return err
}

func renameTable(ctx context.Context, conn *sql.Conn, database, old, new string) error {
	var query bytes.Buffer
	query.WriteString("RENAME TABLE ")
	if database != "" {
		query.Write(quoteName(database))
		query.WriteByte('.')
	}
	query.Write(quoteName(old))
	query.WriteString(" TO ")
	if database != "" {
		query.Write(quoteName(database))
		query.WriteByte('.')
	}
	query.Write(quoteName(new))
	_, err := conn.ExecContext(ctx, query.String())
	return err
}

func createTable(ctx context.Context, conn *sql.Conn, database, table, s string) error {
	var query bytes.Buffer
	query.WriteString("CREATE TABLE ")
	if database != "" {
		query.Write(quoteName(database))
		query.WriteByte('.')
	}
	query.Write(quoteName(table))
	query.WriteString(s)
	_, err := conn.ExecContext(ctx, query.String())
	return err
}

func createTableLike(ctx context.Context, conn *sql.Conn, database, new, orig string) error {
	var query bytes.Buffer
	query.WriteString("CREATE TABLE ")
	if database != "" {
		query.Write(quoteName(database))
		query.WriteByte('.')
	}
	query.Write(quoteName(new))
	query.WriteString(" LIKE ")
	if database != "" {
		query.Write(quoteName(database))
		query.WriteByte('.')
	}
	query.Write(quoteName(orig))
	_, err := conn.ExecContext(ctx, query.String())
	return err
}
