package main

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"

	api "github.com/etesami/air-quality-monitoring/api"
	pb "github.com/etesami/air-quality-monitoring/pkg/protoc"
	internal "github.com/etesami/air-quality-monitoring/svc-local-storage/internal"

	_ "github.com/mattn/go-sqlite3"
	"google.golang.org/grpc"
)

func createTables(db *sql.DB) error {
	// Create your tables here
	query := `
			CREATE TABLE IF NOT EXISTS air_quality (
					id SERIAL PRIMARY KEY,
					aqi TEXT,
  				attributions TEXT,
  				city TEXT,
  				debug TEXT,
  				dominentpol TEXT,
  				forecast TEXT,
  				iaqi TEXT,
  				idx TEXT,
  				time TEXT,
					status TEXT
			);`
	_, err := db.Exec(query)
	return err
}

func main() {

	svcAddress := os.Getenv("SVC_LOCAL_ADD")
	svcPort := os.Getenv("SVC_LOCAL_PORT")
	thisSvc := &api.Service{
		Address: svcAddress,
		Port:    svcPort,
	}

	db, err := sql.Open("sqlite3", "./data.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	err = createTables(db)
	if err != nil {
		log.Fatal(err)
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%s", thisSvc.Port))
	if err != nil {
		log.Fatal(err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterAirQualityMonitoringServer(grpcServer, &internal.Server{Db: db})
	log.Printf("gRPC server is running on port :%s\n", thisSvc.Port)
	if err := grpcServer.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
