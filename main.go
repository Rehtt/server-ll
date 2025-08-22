package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Rehtt/Kit/util/size"
	"github.com/glebarez/sqlite"
	"github.com/shirou/gopsutil/v3/net"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

var (
	defaultDBPath = func() string {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, ".local", "var", "server-ll", "db")
		}
		return "./db"
	}()
	dbFile            = flag.String("f", defaultDBPath, "db file")
	locStr            = flag.String("l", "auto", "show time location. eg: auto,local,utc,Asia/Shanghai")
	showMode          = flag.String("s", "", "show mode: y,m,d")
	includePort       = flag.String("i", "", "include intercase")
	excludePort       = flag.String("e", "", "exclude ports")
	excludeDockerPort = flag.Bool("exclude-docker", false, "exclude docker ports")
	pruneDBDockerPort = flag.Bool("prune-docker", false, "prune db docker ports")
)

func main() {
	flag.Parse()
	// Ensure DB directory exists
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
	if *pruneDBDockerPort {
		if err := IncludeDocker(db).Delete(&DB{}).Error; err != nil {
			panic(err)
		}
	}

	if *showMode != "" {
		show(db)
		return
	}

	historicalRecord := make(map[string]HistoricalRecord)
	var kv KeyValue
	if err := db.Where("key = ?", "historical_record").First(&kv).Error; err != nil && err != gorm.ErrRecordNotFound {
		panic(err)
	}
	if kv.Value != "" {
		if err := json.Unmarshal([]byte(kv.Value), &historicalRecord); err != nil {
			// 如果反序列化失败，忽略旧基线，视为首次运行
			historicalRecord = make(map[string]HistoricalRecord)
		}
	}

	prev, err := net.IOCounters(true)
	if err != nil {
		panic(err)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, v := range prev {
			h, ok := historicalRecord[v.Name]
			if ok {
				data := DB{Name: v.Name}
				currRecv := size.ByteSize(v.BytesRecv)
				currSent := size.ByteSize(v.BytesSent)
				// 如果累计计数器回绕或重置（任一方向变小），视为新基线：当次以当前读数入库
				if currRecv < h.BytesRecv || currSent < h.BytesSent {
					data.Recv = currRecv
					data.Sent = currSent
				} else {
					data.Recv = currRecv - h.BytesRecv
					data.Sent = currSent - h.BytesSent
				}
				if err := tx.Create(&data).Error; err != nil {
					return err
				}
			}
			// 更新新基线
			h.Time = time.Now()
			h.BytesSent = size.ByteSize(v.BytesSent)
			h.BytesRecv = size.ByteSize(v.BytesRecv)
			historicalRecord[v.Name] = h
		}
		b, err := json.Marshal(historicalRecord)
		if err != nil {
			return err
		}
		kv.Value = string(b)
		kv.Key = "historical_record"
		// Upsert 基线
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "key"}},
			DoUpdates: clause.AssignmentColumns([]string{"value"}),
		}).Create(&kv).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		panic(err)
	}
}

func show(db *gorm.DB) {
	var tmp []struct {
		DB
		Strtime string `gorm:"column:strtime"`
	}

	var timestr string
	switch *showMode {
	case "y":
		timestr = "%Y"
	case "m":
		timestr = "%Y-%m"
	case "d":
		timestr = "%Y-%m-%d"
	default:
		timestr = "%Y-%m-%d"
	}

	sql := db.Model(&DB{}).Select(`
        strftime(?,time) as strtime,
        name,
        sum(recv) as recv,
        sum(sent) as sent
    `, timestr).Group("strtime,name").Order("strtime")
	if *includePort != "" {
		sql = sql.Where("name in (?)", strings.Split(*includePort, ","))
	}
	if *excludePort != "" {
		sql = sql.Where("name not in (?)", strings.Split(*excludePort, ","))
	}
	if *excludeDockerPort {
		sql = ExcludeDocker(sql)
	}
	if err := sql.Find(&tmp).Error; err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%-12s%-20s%12s%12s\n", "time", "name", "recv", "sent")
	fmt.Printf("%-12s%-20s%12s%12s\n", "----", "----", "----", "----")
	var t string

	for _, v := range tmp {
		if t != "" && t != v.Strtime {
			fmt.Println("---")
		}
		t = v.Strtime

		recv := v.Recv.GB()
		sent := v.Sent.GB()
		if recv.Size < 1 {
			recv = v.Recv.MB()
			if recv.Size < 1 {
				recv = v.Recv.KB()
			}
		}
		if sent.Size < 1 {
			sent = v.Sent.MB()
			if sent.Size < 1 {
				sent = v.Sent.KB()
			}
		}

		fmt.Printf("%-12s%-20s%12s%12s\n", v.Strtime, v.Name, recv.String(), sent.String())
	}
}

func ExcludeDocker(db *gorm.DB) *gorm.DB {
	return db.Where("name NOT LIKE ? AND name NOT LIKE ? AND name NOT LIKE ?",
		"docker%", "br-%", "veth%")
}

func IncludeDocker(db *gorm.DB) *gorm.DB {
	return db.Where("name LIKE ? OR name LIKE ? OR name LIKE ?",
		"docker%", "br-%", "veth%")
}
