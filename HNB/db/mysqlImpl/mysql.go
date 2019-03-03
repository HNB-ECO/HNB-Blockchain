package mysqlImpl

import (
	"github.com/HNB-ECO/HNB-Blockchain/HNB/db/common"
	"database/sql"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/MySQL"
	"strings"
	"time"
)

type blkStore struct {
	db *sql.DB
}

func NewMySQLStore(ipport, username, passwd string) (*blkStore, error) {
	libName := "hnblib"
	ip := strings.Split(ipport, ":")
	c := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8\n", username, passwd, ip[0], ip[1], "mysql")
	db, err := sql.Open("mysql", c)
	if err != nil {
		panic(err)
	}
	if _, err = db.Exec("create database if not exists " + libName); err != nil {
		panic(err)
	}
	if err = db.Close(); err != nil {
		panic(err)
	}

	c = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8\n", username, passwd, ip[0], ip[1], libName)
	db, err = sql.Open("mysql", c)
	if err != nil {
		panic(err)
	}

	err = checkConnection(db)
	if err != nil {
		panic(err)
	}

	return &blkStore{db}, nil
}

func checkConnection(db *sql.DB) error {
	if db == nil {
		return errors.New("db is nil")
	}
	recv := make(chan error)

	dbCom := "select now()"

	go func() {
		_, err := db.Exec(dbCom)
		select {
		case recv <- err:
		default:

		}
	}()

	select {
	case <-time.After(10 * time.Second):
		return errors.New("connection timeout")
	case err := <-recv:
		return err
	}

	return nil
}

const (
	CREATETABLE = "create table if not exists `block` (" +
		"`blkNum` BIGINT UNSIGNED NOT NULL," +
		"`body` TEXT NOT NULL," +
		"PRIMARY KEY (`blkNum`)" +
		") DEFAULT CHARSET=utf8;"

	INSERTTABLE = "insert block(blkNum, body) values(%d, '%s')"

	QUERYTABLE = "select body from block where blkNum=%d"

	DELETETABLE = "delete from block where blkNum = %d"
)

func (bs *blkStore) NewBlockTable() error {
	_, err := bs.db.Exec(CREATETABLE)
	return err
}

func (bs *blkStore) Put(key []byte, value []byte) error {

	blkNum := binary.BigEndian.Uint64(key)
	valueString := base64.StdEncoding.EncodeToString(value)

	sqlInsert := fmt.Sprintf(INSERTTABLE, blkNum, valueString)
	_, err := bs.db.Exec(sqlInsert)
	return err
}
func (bs *blkStore) Get(key []byte) ([]byte, error) {
	blkNum := binary.BigEndian.Uint64(key)
	sqlSelect := fmt.Sprintf(QUERYTABLE, blkNum)
	rows, err := bs.db.Query(sqlSelect)
	if err != nil {
		return nil, err
	}

	defer func() {
		rows.Close()
	}()

	var body string
	exist := rows.Next()

	if exist == false {
		return nil, nil
	}

	err = rows.Scan(&body)
	if err != nil {
		return nil, err
	}

	decodeBytes, err := base64.StdEncoding.DecodeString(body)

	return decodeBytes, err
}

func (bs *blkStore) Delete(key []byte) error {
	blkNum := binary.BigEndian.Uint64(key)
	sqlDelete := fmt.Sprintf(DELETETABLE, blkNum)
	_, err := bs.db.Exec(sqlDelete)
	return err
}

func (bs *blkStore) Close() error {
	panic("sql not implement Close")
}

func (bs *blkStore) Has(key []byte) (bool, error) {
	panic("sql not implement Has")
}

func (bs *blkStore) NewBatch() {
	panic("sql not implement NewBatch")
}
func (bs *blkStore) BatchPut(key []byte, value []byte) {
	panic("sql not implement BatchPut")
}
func (bs *blkStore) BatchDelete(key []byte) {
	panic("sql not implement BatchDelete")
}
func (bs *blkStore) BatchCommit() error {
	panic("sql not implement BatchCommit")
}

func (bs *blkStore) NewIterator(prefix []byte) common.Iterator {
	panic("sql not implement NewIterator")
}
