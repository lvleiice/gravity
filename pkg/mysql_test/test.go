package mysql_test

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/juju/errors"
	log "github.com/sirupsen/logrus"

	"strings"

	"os"

	"github.com/BurntSushi/toml"

	"strconv"

	"github.com/moiot/gravity/pkg/utils"
)

const TestTableName = "test_table"
const TestTableWithoutTs = "test_table_without_ts"
const DummyTableName = "dummy_table"
const TxnRouteTableName = "drc_routes"
const TestScanColumnTableIdPrimary = "test_scan_column_id_primary"
const TestScanColumnTableMultiPrimary = "test_scan_column_multiple_primary"
const TestScanColumnTableUniqueIndexEmailString = "test_scan_column_unique_index_email_string"
const TestScanColumnTableUniqueIndexTime = "test_scan_column_unique_index_time"
const TestScanColumnTableNoKey = "test_scan_column_no_key"
const deadSignalTable = "dead_signals"

var setupSqls = []string{
	fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
  id int(11) unsigned NOT NULL,
  name varchar(256) DEFAULT NULL,
  email varchar(30) COLLATE utf8mb4_bin NOT NULL DEFAULT 'default_email',
  ts TIMESTAMP,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`, TestTableName),
	fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
  id int(11) unsigned NOT NULL,
  name varchar(256) DEFAULT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`, TestTableWithoutTs),

	fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s (
  id int(11) unsigned NOT NULL,
  name varchar(256) DEFAULT NULL,
  ts TIMESTAMP,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`, DummyTableName),

	fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s(
	id int(11) unsigned NOT NULL,
	region varchar(256) NOT NULL,
	v BIGINT UNSIGNED NOT NULL DEFAULT 0,
	PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`, TxnRouteTableName),

	fmt.Sprintf("REPLACE INTO %s (id, region) VALUES (1, 'eu')", TxnRouteTableName),

	fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s(
  id int(11) unsigned NOT NULL,
  name varchar(256) DEFAULT NULL,
  PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`, TestScanColumnTableIdPrimary),

	fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s(
  id int(11) unsigned NOT NULL,
  name varchar(256) NOT NULL,
  email varchar(30) COLLATE utf8mb4_bin NOT NULL DEFAULT 'default_email',
  ts TIMESTAMP,
  PRIMARY KEY (id, name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`, TestScanColumnTableMultiPrimary),

	fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s(
  id int(11) unsigned NOT NULL,
  name varchar(256) DEFAULT NULL,
  email varchar(30) COLLATE utf8mb4_bin NOT NULL DEFAULT 'default_email',
  ts TIMESTAMP,
  UNIQUE INDEX email_idx (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`, TestScanColumnTableUniqueIndexEmailString),

	fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s(
  id int(11) unsigned NOT NULL,
  ts TIMESTAMP,
  UNIQUE INDEX time_idx (ts)
) ENGINE=InnoDB DEFAULT CHARSET=utf8`, TestScanColumnTableUniqueIndexTime),

	fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s(
  id int(11) unsigned NOT NULL) ENGINE=InnoDB DEFAULT CHARSET=utf8`, TestScanColumnTableNoKey),
}

var deadSignalSQL = fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS drc.%s(
id INT NOT NULL,
v BIGINT UNSIGNED NOT NULL DEFAULT 0,
PRIMARY KEY (id)
)ENGINE=InnoDB DEFAULT CHARSET=utf8
`, deadSignalTable)

const srcDBConfStr = `
host = "source-db"
username = "root"
password = ""
port = 3306
`
const targetDBConfStr = `
host = "target-db"
username = "root"
password = ""
port = 3306
`

const TestDBPrefix = "__test_drc__"

func TestDBName(name string) string {
	return fmt.Sprintf("%s_%s", TestDBPrefix, name)
}

func IsTestDB(schemaName string) bool {
	return strings.Contains(schemaName, TestDBPrefix)
}

func dropDBStatement(testDBName string) string {
	return fmt.Sprintf("DROP DATABASE IF EXISTS %s", testDBName)
}

func createDBStatement(tesetDBName string) string {
	return fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s ", tesetDBName)
}

func dbConfig(configStr string) *utils.DBConfig {
	dbConfig := utils.DBConfig{}
	_, err := toml.Decode(configStr, &dbConfig)
	if err != nil {
		log.Fatalf("failed to decode srcDBConfigStr")
	}
	if dbConfig.Location == "" {
		dbConfig.Location = time.Local.String()
	}
	return &dbConfig

}

func SourceDBConfig() *utils.DBConfig {
	cfg := utils.DBConfig{}

	sourceDBHost, ok := os.LookupEnv("SOURCE_DB_HOST")
	if !ok {
		return dbConfig(srcDBConfStr)
	}
	cfg.Host = sourceDBHost

	sourceDBUser, ok := os.LookupEnv("SOURCE_DB_USER")
	if ok {
		cfg.Username = sourceDBUser
	} else {
		cfg.Username = "root"
	}

	sourceDBPort, ok := os.LookupEnv("SOURCE_DB_PORT")
	if ok {
		p, err := strconv.Atoi(sourceDBPort)
		if err != nil {
			log.Fatalf("invalid port")
		}
		cfg.Port = p
	} else {
		cfg.Port = 3306
	}

	sourceDBPass, ok := os.LookupEnv("SOURCE_DB_PASSWORD")
	if ok {
		cfg.Password = sourceDBPass
	} else {
		cfg.Password = ""
	}

	return &cfg
}

func TargetDBConfig() *utils.DBConfig {
	cfg := utils.DBConfig{}

	targetDBHost, ok := os.LookupEnv("TARGET_DB_HOST")
	if !ok {
		return dbConfig(targetDBConfStr)
	}
	cfg.Host = targetDBHost

	targetDBUser, ok := os.LookupEnv("TARGET_DB_USER")
	if ok {
		cfg.Username = targetDBUser
	} else {
		cfg.Username = "root"
	}

	targetDBPort, ok := os.LookupEnv("TARGET_DB_PORT")
	if ok {
		p, err := strconv.Atoi(targetDBPort)
		if err != nil {
			log.Fatalf("invalid port")
		}
		cfg.Port = p
	} else {
		cfg.Port = 3306
	}

	targetDBPass, ok := os.LookupEnv("TARGET_DB_PASSWORD")
	if ok {
		cfg.Password = targetDBPass
	} else {
		cfg.Password = ""
	}
	return &cfg
}

func createConnection(confStr string) (*sql.DB, error) {
	cfg := dbConfig(confStr)
	db, err := utils.CreateDBConnection(cfg)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return db, nil
}

func IsDeadSignal(schema string, table string) bool {
	return schema == "drc" && table == deadSignalTable
}

func SendDeadSignal(db *sql.DB, gravityID uint32) error {
	_, err := db.Exec(fmt.Sprintf("insert into %s.%s(id, v) values (%d, 1) on duplicate key update v = v+1", "drc", deadSignalTable, gravityID))
	return errors.Trace(err)
}

func CountTestTable(db *sql.DB, testDBName string, testTableName string) (int, error) {
	var count int

	query := fmt.Sprintf("SELECT COUNT(1) FROM `%s`.`%s`", testDBName, testTableName)
	row := db.QueryRow(query)

	err := row.Scan(&count)
	if err != nil {
		return 0, errors.Trace(err)
	}
	return count, nil
}

func testInsertSQL(testDBName string, testTableName string, args map[string]interface{}) (string, []interface{}) {
	placeHolders := make([]string, len(args))
	for i := 0; i < len(placeHolders); i++ {
		placeHolders[i] = "?"
	}

	argNames := make([]string, len(args))
	argValues := make([]interface{}, len(args))

	i := 0
	for name, value := range args {
		argNames[i] = name
		argValues[i] = value
		i++
	}

	argNameString := fmt.Sprintf("%s", strings.Join(argNames, ","))
	placeHolderString := fmt.Sprintf("%s", strings.Join(placeHolders, ","))

	return fmt.Sprintf("INSERT INTO `%s`.`%s` (%s) VALUES (%s)", testDBName, testTableName, argNameString, placeHolderString), argValues

}

func InsertIntoTestTable(db *sql.DB, testDBName string, testTableName string, args map[string]interface{}) error {
	statement, argValues := testInsertSQL(testDBName, testTableName, args)
	log.Infof(statement)
	_, err := db.Exec(statement, argValues...)
	return err
}

func InsertIntoTestTableWithTxnRoute(db *sql.DB, testDBName string, testTableName string, args map[string]interface{}, region string) error {
	txn, err := db.Begin()
	if err != nil {
		return errors.Trace(err)
	}

	statement, argValues := testInsertSQL(testDBName, testTableName, args)

	log.Info(statement)
	_, err = txn.Exec(statement, argValues)
	if err != nil {
		return errors.Trace(err)
	}

	routeStatement := fmt.Sprintf("UPDATE %s.%s SET region = '%s', v = v + 1 WHERE id = 1", testDBName, TxnRouteTableName, region)
	_, err = txn.Exec(routeStatement)
	if err != nil {
		return errors.Trace(err)
	}
	return errors.Trace(txn.Commit())
}

func UpdateTestTableWithMultiRows(db *sql.DB, testDBName string, testTableName string, whereClause string) error {
	txn, err := db.Begin()
	if err != nil {
		return errors.Trace(err)
	}

	if _, err := txn.Exec(fmt.Sprintf("USE %s", testDBName)); err != nil {
		return errors.Trace(err)
	}

	updateStatment := fmt.Sprintf("UPDATE %s SET name = '%s' WHERE %s", testTableName, fmt.Sprintf("%v", time.Now()), whereClause)
	log.Debugf("[source_db] updateStatment: %s", updateStatment)
	if _, err := txn.Exec(updateStatment); err != nil {
		return errors.Trace(err)
	}
	return txn.Commit()
}

func UpdateTestTable(db *sql.DB, testDBName string, testTableName string, id int, newName string) error {
	txn, err := db.Begin()
	if err != nil {
		return errors.Trace(err)
	}

	if _, err := txn.Exec(fmt.Sprintf("USE %s", testDBName)); err != nil {
		return errors.Trace(err)
	}

	if _, err := txn.Exec(fmt.Sprintf("UPDATE %s SET name = '%s' WHERE id = %v", testTableName, newName, id)); err != nil {
		return errors.Trace(err)
	}
	return txn.Commit()
}

func DeleteTestTable(db *sql.DB, testDBName string, testTableName string, id int) error {
	txn, err := db.Begin()
	if err != nil {
		return errors.Trace(err)
	}

	if _, err := txn.Exec(fmt.Sprintf("USE %s", testDBName)); err != nil {
		return errors.Trace(err)
	}

	log.Info(fmt.Sprintf("DELETE from %s WHERE id = %v", testTableName, id))
	if _, err := txn.Exec(fmt.Sprintf("DELETE from %s WHERE id = %v", testTableName, id)); err != nil {
		return errors.Trace(err)
	}
	return txn.Commit()
}

func QueryTestTable(db *sql.DB, testDBName string, testTableName string, id int) (string, error) {
	var resultId int
	var resultName string
	statement := fmt.Sprintf("SELECT id, name from %s.%s WHERE id = %v", testDBName, testTableName, id)
	err := db.QueryRow(statement).Scan(&resultId, &resultName)
	return resultName, err
}

func setupTestDB(db *sql.DB, dbName string) error {

	// setup test tableNames
	if _, err := db.Exec(dropDBStatement(dbName)); err != nil {
		return errors.Trace(err)
	}

	if _, err := db.Exec(createDBStatement(dbName)); err != nil {
		return errors.Trace(err)
	}

	for _, statement := range setupSqls {
		txn, err := db.Begin()
		if err != nil {
			return errors.Trace(err)
		}

		txn.Exec(fmt.Sprintf("USE %s", dbName))
		_, err = txn.Exec(statement)
		if err != nil {
			return errors.Trace(err)
		}

		err = txn.Commit()
		if err != nil {
			return errors.Trace(err)
		}
	}

	// setup internal db and tableNames
	if _, err := db.Exec(createDBStatement("drc")); err != nil {
		return errors.Trace(err)
	}

	_, err := db.Exec(deadSignalSQL)
	return errors.Trace(err)
}

func MustCreateSourceDBConn() *sql.DB {
	var db *sql.DB
	var err error

	// If we can find env def for source db, use it first
	testUrl, ok := os.LookupEnv("SOURCE_DB_URL")
	if ok {
		log.Infof("[MustSetupSourceDB] with SOURCE_DB_URL: %v", testUrl)
		db, err = sql.Open("mysql", testUrl)
		if err != nil {
			log.Fatalf("failed to setup testUrl: %v", testUrl)
		}
	} else {
		db, err = createConnection(srcDBConfStr)
		if err != nil {
			log.Fatalf("failed to create source db conn: %v", errors.ErrorStack(err))
		}
	}
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(0)

	_, err = db.Exec("SET global max_connections = 500")
	if err != nil {
		log.Fatalf("failed to set max_connections, err: %v", err.Error())
	}

	return db
}

// MustSetupSourceDB setup a test db, so that we can use different db in different test cases
func MustSetupSourceDB(dbName string) *sql.DB {
	db := MustCreateSourceDBConn()
	err := setupTestDB(db, dbName)
	if err != nil {
		log.Fatalf("failed to setup source db err: %v", errors.ErrorStack(err))
	}

	SetMySQLGlobalVars(db)

	db.SetMaxIdleConns(150)
	db.SetMaxOpenConns(150)

	return db
}

func MustCreateTargetDBConn() *sql.DB {
	var db *sql.DB
	var err error

	// If we can find env def for target db, use it first
	testUrl, ok := os.LookupEnv("TARGET_DB_URL")
	if ok {
		db, err = sql.Open("mysql", testUrl)
		if err != nil {
			log.Fatalf("failed to setup testUrl: %v", testUrl)
		}
	} else {
		db, err = createConnection(targetDBConfStr)
		if err != nil {
			log.Fatalf("failed to create source db conn: %v", errors.ErrorStack(err))
		}
	}
	return db
}

func MustSetupTargetDB(dbName string) *sql.DB {
	db := MustCreateTargetDBConn()

	err := setupTestDB(db, dbName)
	if err != nil {
		log.Fatalf("failed to setup source db1 err: %v", errors.ErrorStack(err))
	}

	SetMySQLGlobalVars(db)

	return db
}

var MaxConn = 2048

func SetMySQLGlobalVars(db *sql.DB) {
	globalSettings := []string{
		fmt.Sprintf("SET global max_connections = %d", MaxConn),
		"SET global connect_timeout = 3600",
		"SET global net_read_timeout = 3600",
		"SET global net_write_timeout = 3600",
		"SET global max_allowed_packet = 1073741824",
	}

	for _, s := range globalSettings {
		_, err := db.Exec(s)
		if err != nil {
			log.Fatalf("failed to set global settings, s: %v, err: %v", s, err.Error())
		}
	}
}
