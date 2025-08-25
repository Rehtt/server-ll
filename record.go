package main

import (
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/Rehtt/Kit/util/size"
	"github.com/shirou/gopsutil/v3/net"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func Record(db *gorm.DB) {
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

	includePort := strings.Split(*includePort, ",")
	excludePort := strings.Split(*excludePort, ",")

	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, v := range prev {
			if (len(includePort) > 0 && (!slices.Contains(includePort, v.Name) || slices.Contains(excludePort, v.Name))) ||
				(*excludeDockerPort && strings.HasPrefix(v.Name, "docker") || strings.HasPrefix(v.Name, "br-") || strings.HasPrefix(v.Name, "veth")) {
				continue
			}
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
