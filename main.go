package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Rehtt/Kit/cli"
	"github.com/Rehtt/Kit/util/size"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type DB struct {
	Time time.Time     `gorm:"column:time;type:timestamp;not null;autoCreateTime"`
	Name string        `gorm:"column:name;index"`
	Recv size.ByteSize `gorm:"column:recv;type:bigint"`
	Sent size.ByteSize `gorm:"column:sent;type:bigint"`
}

type KeyValue struct {
	Key   string `gorm:"column:key;type:text;unique"`
	Value string `gorm:"column:value;type:text"`
}
type HistoricalRecord struct {
	Time      time.Time     `json:"time"`
	BytesRecv size.ByteSize `json:"bytes_recv"`
	BytesSent size.ByteSize `json:"bytes_sent"`
}

type SetExcludePortConfig struct {
	Ports         []string `json:"ports"`
	ExcludeDocker bool     `json:"exclude_docker"`
}

var defaultDBPath = func() string {
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".local", "var", "server-ll", "db")
	}
	return "./db"
}()

var (
	dbFile = cli.String("f", defaultDBPath, "db file")
	locStr = cli.String("l", "auto", "show time location. eg: auto,local,utc,Asia/Shanghai")

	showCommand  = cli.NewCLI("show", "show historical traffic", flag.ExitOnError)
	pruneCommand = cli.NewCLI("prune", "prune all db docker ports", flag.ExitOnError)
)

var (
	showMode          = showCommand.String("s", "d", "show mode: y,m,d")
	includePort       = showCommand.String("i", "", "include intercase")
	excludePort       = showCommand.String("e", "", "exclude ports")
	excludeDockerPort = showCommand.Bool("exclude-docker", false, "exclude docker ports")
)

func main() {
	showCommand.CommandFunc = func(args []string) error {
		db := openDB()
		Show(db)
		return nil
	}
	pruneCommand.CommandFunc = func(args []string) error {
		db := openDB()
		Prune(db)
		return nil
	}

	cli.CommandLine.CommandFunc = func(args []string) error {
		if len(args) > 0 {
			cli.CommandLine.Help()
			return nil
		}
		db := openDB()
		Record(db)
		return nil
	}

	cli.AddCommand(showCommand, pruneCommand)
	cli.Parse()
}

func ExcludeDocker(db *gorm.DB) *gorm.DB {
	return db.Where("name NOT LIKE ? AND name NOT LIKE ? AND name NOT LIKE ?",
		"docker%", "br-%", "veth%")
}

func IncludeDocker(db *gorm.DB) *gorm.DB {
	return db.Where("name LIKE ? OR name LIKE ? OR name LIKE ?",
		"docker%", "br-%", "veth%")
}

func openDB() *gorm.DB {
	if err := os.MkdirAll(filepath.Dir(*dbFile), 0o755); err != nil {
		panic(err)
	}
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?_loc=%s", *dbFile, *locStr)))
	if err != nil {
		panic(err)
	}
	if err := db.AutoMigrate(&DB{}, &KeyValue{}); err != nil {
		panic(err)
	}
	return db
}
