package main

import (
	"fmt"
	"strings"

	"gorm.io/gorm"
)

func Prune(db *gorm.DB) {
	var names []string
	if err := IncludeDocker(db).Model(&DB{}).Group("name").Order("name").Pluck("name", &names).Error; err != nil {
		panic(err)
	}
	fmt.Println("Prune DB all Docker ports:")
	for _, name := range names {
		fmt.Printf(" - %s\n", name)
	}
	fmt.Print("[y/N]:")
	var reply string
	fmt.Scanln(&reply)

	if strings.ToLower(reply) != "y" {
		return
	}
	if err := db.Where("name in (?)", names).Delete(&DB{}).Error; err != nil {
		panic(err)
	}
	if err := db.Exec("VACUUM;").Error; err != nil {
		panic(err)
	}
	fmt.Println("Prune DB all Docker ports done")
}
