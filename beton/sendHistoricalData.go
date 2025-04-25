package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/websocket"
)

func sendHistoricalData(db *sql.DB, conn *websocket.Conn) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := db.QueryContext(ctx, `
		SELECT
			DATE_FORMAT(m.time, '%Y-%m-%d %H:%i:%s.%f') as time,
			p.parameter_name,
			m.value
		FROM Measurement m
		JOIN Parameters p ON m.id_parameter = p.id_parameter
		ORDER BY m.time DESC
		LIMIT 100
	`)
	if err != nil {
		return fmt.Errorf("query error: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var m Measurement
		var paramName string
		if err := rows.Scan(&m.Time, &paramName, &m.Value); err != nil {
			log.Println("Row scan error:", err)
			continue
		}
		m.Parameter = paramName

		if err := conn.WriteJSON(m); err != nil {
			return fmt.Errorf("websocket write error: %v", err)
		}
		count++
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows iteration error: %v", err)
	}

	log.Printf("Sent %d historical records", count)
	return nil
}
