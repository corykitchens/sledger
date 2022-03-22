package sledger

import (
	"database/sql"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v2"
)

func SayHello() string {
	return fmt.Sprintf("Hello World")
}
func Run() {
	path := flag.String("ledger", "sledger.yaml", "path within git repository to sledger file")
	database := flag.String("database", "postgresql://localhost", "URL of database to update")
	flag.Parse()

	fmt.Println("==> Sledger")

	fmt.Println("==> Parameters")
	fmt.Println("  Ledger:", *path)
	fmt.Println("Database:", *database)

	fmt.Println("==> Setup")

	fmt.Println("Connecting to database...")
	db, err := sql.Open("postgres", *database)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	verifySledgerVersion(db)
	verifySledgerTable(db)
	sledger := loadSledgerYaml(*path)

	fmt.Println("==> Synchorization")
	sync(db, sledger)

	fmt.Println("==> Complete")
}

func verifySledgerVersion(db *sql.DB) {
	createSledgerVersion(db)

	version := getSledgerVersion(db)

	if version == "" {
		insertSledgerVersion(db)
		version = SLEDGER_VERSION
	}

	if version != SLEDGER_VERSION {
		panic("Unsupported sledger version. Please use the correct version of sledger.")
	}
}

func createSledgerVersion(db *sql.DB) {
	fmt.Println("Ensuring sledger_version exists...")
	rows, err := db.Query("create table if not exists sledger_version (sledger_version text)")
	if err != nil {
		panic(err)
	}
	rows.Close()
}

func getSledgerVersion(db *sql.DB) string {
	fmt.Println("Getting sledger_version...")
	rows, err := db.Query("select sledger_version from sledger_version limit 1")
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	if rows.Next() {
		var version string

		if err := rows.Scan(&version); err != nil {
			panic(err)
		}

		return version
	}

	return ""
}

func insertSledgerVersion(db *sql.DB) {
	fmt.Println("Inserting sledger_version...")
	rows, err := db.Query("insert into sledger_version values ($1)", SLEDGER_VERSION)
	if err != nil {
		panic(err)
	}
	rows.Close()
}

func verifySledgerTable(db *sql.DB) {
	fmt.Println("Creating sledger table if it does not exist...")
	rows, err := db.Query("create table if not exists sledger (index bigint not null, forward text not null, backward text not null, timestamp timestamp not null default now())")
	if err != nil {
		panic(err)
	}
	rows.Close()
}

func loadSledgerYaml(path string) sledger {
	fmt.Println("Loading sledger...")

	data, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}

	sledger := sledger{}

	if err := yaml.Unmarshal(data, &sledger); err != nil {
		panic(err)
	}

	return sledger
}

func sync(db *sql.DB, sledger sledger) {
	rows, err := db.Query("select forward, backward from sledger order by index")
	if err != nil {
		panic(err)
	}
	defer rows.Close()

	index := 0

	var rollbacks []rollback

	for rows.Next() {
		var dbForward, dbBackward string

		if err := rows.Scan(&dbForward, &dbBackward); err != nil {
			panic(err)
		}

		if index >= len(sledger.Sledger) {
			rollbacks = append(rollbacks, rollback{index, dbBackward})
		} else {
			yamlForward := sledger.Sledger[index].Forward

			if dbForward != yamlForward {
				fmt.Printf("ERROR     Database does not match YAML\nDatabase: %s\n    YAML: %s\n", dbForward, yamlForward)
				os.Exit(1)
			}

			fmt.Printf("SKIP      %s\n", yamlForward)
		}

		index++
	}

	for index < len(sledger.Sledger) {
		doForward(db, index, sledger.Sledger[index].Forward, sledger.Sledger[index].Backward)

		index++
	}

	for index = len(rollbacks) - 1; index >= 0; index-- {
		doRollback(db, rollbacks[index].index, rollbacks[index].dbBackward)
	}
}

func doRollback(db *sql.DB, index int, dbBackward string) {
	if dbBackward == "" {
		fmt.Println("ERROR     Missing rollback command, cannot rollback.")
		os.Exit(1)
	} else {
		fmt.Printf("ROLLBACK  %s\n", dbBackward)

		tx, err := db.Begin()
		if err != nil {
			panic(err)
		}
		defer tx.Rollback()

		rows, err := tx.Query(dbBackward)
		if err != nil {
			panic(err)
		}
		rows.Close()

		rows, err = tx.Query("delete from sledger where index = $1", index)
		if err != nil {
			panic(err)
		}
		rows.Close()

		if err := tx.Commit(); err != nil {
			panic(err)
		}
	}
}

func doForward(db *sql.DB, index int, yamlForward string, yamlBackward string) {
	fmt.Printf("FORWARD   %s\n", yamlForward)

	tx, err := db.Begin()
	if err != nil {
		panic(err)
	}
	defer tx.Rollback()

	rows, err := db.Query(yamlForward)
	if err != nil {
		panic(err)
	}
	rows.Close()

	rows, err = db.Query("insert into sledger (index, forward, backward) values ($1, $2, $3)", index, yamlForward, yamlBackward)
	if err != nil {
		panic(err)
	}
	rows.Close()

	if err := tx.Commit(); err != nil {
		panic(err)
	}
}
