package mysqldump_loader

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"strconv"
	"sync"

	"github.com/go-sql-driver/mysql"
)

type loader struct {
	db          *sql.DB
	errCh       chan error
	guardCh     chan struct{}
	lowPriority bool
	wg          sync.WaitGroup
	wgs         map[string]*sync.WaitGroup
}

func NewLoader(db *sql.DB, concurrency int, lowPriority bool) *loader {
	return &loader{
		db:          db,
		errCh:       make(chan error, concurrency),
		guardCh:     make(chan struct{}, concurrency),
		lowPriority: lowPriority,
		wgs:         make(map[string]*sync.WaitGroup),
	}
}

func (l *loader) execute(ctx context.Context, q *query, charset, database, table string) error {
	select {
	case err := <-l.errCh:
		return err
	default:
	}

	wg, ok := l.wgs[table]
	if !ok {
		wg = &sync.WaitGroup{}
		l.wgs[table] = wg
	}

	l.guardCh <- struct{}{}
	wg.Add(1)
	l.wg.Add(1)
	go func() {
		defer func() { <-l.guardCh }()
		defer wg.Done()
		defer l.wg.Done()
		if err := l.load(ctx, q, charset, database, table); err != nil {
			l.errCh <- err
		}
	}()
	return nil
}

func (l *loader) load(ctx context.Context, q *query, charset, database, table string) error {
	i, err := convert(q)
	if err != nil {
		return err
	}

	var query bytes.Buffer
	query.WriteString("LOAD DATA ")
	if l.lowPriority {
		query.WriteString("LOW_PRIORITY ")
	}
	query.WriteString(fmt.Sprintf("LOCAL INFILE 'Reader::%d' ", q.line))
	if i.replace {
		query.WriteString("REPLACE ")
	} else if i.ignore {
		query.WriteString("IGNORE ")
	}
	query.WriteString("INTO TABLE ")
	if database != "" {
		query.Write(quoteName(database))
		query.WriteByte('.')
	}
	if table != "" {
		query.Write(quoteName(table))
	} else {
		query.Write(quoteName(i.table))
	}
	if charset != "" {
		query.WriteString(" CHARACTER SET ")
		query.WriteString(charset)
	}

	mysql.RegisterReaderHandler(strconv.Itoa(q.line), func() io.Reader { return i.r })
	defer mysql.DeregisterReaderHandler(strconv.Itoa(q.line))

	conn, err := l.db.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()

	if charset != "" {
		if err := setCharacterSet(ctx, conn, charset); err != nil {
			return err
		}
	}
	if err := disableForeignKeyChecks(ctx, conn); err != nil {
		return err
	}
	if _, err := conn.ExecContext(ctx, query.String()); err != nil {
		return err
	}

	return nil
}

func (l *loader) wait() error {
	waitCh := make(chan struct{})
	go func() {
		defer close(waitCh)
		l.wg.Wait()
	}()

	select {
	case err := <-l.errCh:
		return err
	case <-waitCh:
		return nil
	}
}

func (l *loader) waitGroup(table string) *sync.WaitGroup {
	return l.wgs[table]
}
