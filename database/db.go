package database

import (
	"fmt"
	"log"
	"mapmarker/backend/config"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var Connection *gorm.DB

func Connect(host, port, db, username, password string) error {
	var err error
	connectionString := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Hong_Kong",
		host, username, password, db, port)

	Connection, err = gorm.Open(postgres.Open(connectionString))

	if err != nil {
		return err
	}

	return nil
}

func Init() {
	err := Connect(
		config.Data.DB.Host,
		config.Data.DB.Port,
		config.Data.DB.Name,
		config.Data.DB.Username,
		config.Data.DB.Password,
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("connected to postgresql")
}
