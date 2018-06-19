package mysqldump_loader

import (
	"context"
	"database/sql"
	"sync"
)

type replacer struct {
	db    *sql.DB
	errCh chan error
	wg    sync.WaitGroup
}

func NewReplacer(db *sql.DB) *replacer {
	if db == nil {
		return &replacer{} // ,errCh: make(chan error, 1)}
	}
	return &replacer{db: db, errCh: make(chan error, 1)}
}

func (s *replacer) execute(ctx context.Context, database, new, old string, wg *sync.WaitGroup) error {
	select {
	case err := <-s.errCh:
		return err
	default:
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if wg != nil {
			wg.Wait()
		}
		if err := s.replace(ctx, database, new, old); err != nil {
			s.errCh <- err
		}
	}()
	return nil
}

func (s *replacer) replace(ctx context.Context, database, new, old string) error {
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	if err := disableForeignKeyChecks(ctx, conn); err != nil {
		return err
	}
	if err := dropTableIfExists(ctx, conn, database, old); err != nil {
		return err
	}
	return renameTable(ctx, conn, database, new, old)
}

func (s *replacer) wait() error {
	waitCh := make(chan struct{})
	go func() {
		defer close(waitCh)
		s.wg.Wait()
	}()

	select {
	case err := <-s.errCh:
		return err
	case <-waitCh:
		return nil
	}
}
