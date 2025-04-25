package main

import (
	"database/sql"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func serveWs(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("token")
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	claims := &Claims{}
	token, err := jwt.ParseWithClaims(
		cookie.Value,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		},
		jwt.WithLeeway(5*time.Second),
	)

	if err != nil || !token.Valid {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer conn.Close()

	log.Println("New WebSocket connection from:", claims.Login)

	//if err := sendHistoricalData(db, conn); err != nil {
	//	log.Println("Error sending historical data:", err)
	//}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rows, err := db.Query(`
				SELECT 
					DATE_FORMAT(m.time, '%Y-%m-%d %H:%i:%s.%f') as time,
					p.parameter_name, 
					m.value 
				FROM Measurement m
				JOIN Parameters p ON m.id_parameter = p.id_parameter
				ORDER BY m.time DESC
				LIMIT 5
			`)
			if err != nil {
				log.Println("Query error:", err)
				continue
			}

			var newData []Measurement
			for rows.Next() {
				var m Measurement
				var paramName string
				if err := rows.Scan(&m.Time, &paramName, &m.Value); err != nil {
					log.Println("Row scan error:", err)
					continue
				}
				m.Parameter = paramName
				newData = append(newData, m)
			}
			rows.Close()

			for _, m := range newData {
				if err := conn.WriteJSON(m); err != nil {
					log.Println("WebSocket write error:", err)
					return
				}
			}

		case <-r.Context().Done():
			log.Println("Connection closed by client")
			return
		}
	}
}
