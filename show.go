package main

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

func Show(db *gorm.DB) {
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
