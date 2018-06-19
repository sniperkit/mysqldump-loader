package main

import (
	"database/sql"
	"flag"
	"log"
	"os"
	"runtime"

	load "github.com/sniperkit/mysqldump-loader/pkg"
)

var (
	concurrency    = flag.Int("concurrency", 0, "Maximum number of concurrent load operations.")
	dataSourceName = flag.String("data-source-name", "", "Data source name for MySQL server to load data into.")
	dumpFile       = flag.String("dump-file", "", "MySQL dump file to load.")
	lowPriority    = flag.Bool("low-priority", false, "Use LOW_PRIORITY when loading data.")
	replaceTable   = flag.Bool("replace-table", false, "Load data into a temporary table and replace the old table with it once load is complete.")
)

func init() {
	flag.Lookup("concurrency").DefValue = "Number of available CPUs"
}

func main() {
	flag.Parse()

	if *concurrency == 0 {
		*concurrency = runtime.NumCPU()
	}

	if *dataSourceName == "" {
		*dataSourceName = os.Getenv("DATA_SOURCE_NAME")
	}

	db, err := sql.Open("mysql", *dataSourceName)
	if err != nil {
		log.Fatal(err)
	}

	r := os.Stdin
	if *dumpFile != "" {
		if r, err = os.Open(*dumpFile); err != nil {
			log.Fatal(err)
		}
	}

	loader := load.NewLoader(db, *concurrency, *lowPriority)
	scanner := load.NewScanner(r)

	var executor *load.Executor
	if *replaceTable {
		executor = load.NewExecutor(db, loader, scanner, load.NewReplacer(db))
	} else {
		executor = load.NewExecutor(db, loader, scanner, nil)
	}

	if err := executor.Execute(); err != nil {
		log.Fatal(err)
	}

}
