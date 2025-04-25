package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
)

const (
	jwtSecret = "your-secret-key-here"
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	parameterIDs = map[string]int{
		"speed":    1,
		"weight":   2,
		"status":   3,
		"him":      4,
		"humidity": 5,
	}
)

func main() {
	log.Println("Starting application...")

	db, err := sql.Open("mysql", "root:X47z56f32@tcp(localhost:3306)/beton")
	if err != nil {
		log.Fatal("DB connection error:", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatal("DB ping error:", err)
	}
	log.Println("DB connection established")

	// Основные обработчики маршрутов
	http.HandleFunc("/login", loginHandler(db))
	http.Handle("/logout", corsMiddleware(authMiddleware(http.HandlerFunc(logoutHandler(db)))))
	http.Handle("/", corsMiddleware(authMiddleware(http.HandlerFunc(serveHome))))
	http.Handle("/admin", corsMiddleware(authMiddleware(adminMiddleware(http.HandlerFunc(serveAdmin)))))
	http.Handle("/ws", corsMiddleware(authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serveWs(db, w, r)
	}))))
	http.Handle("/parameters", corsMiddleware(authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parametersHandler(db, w, r)
	}))))
	http.Handle("/update-parameters", corsMiddleware(authMiddleware(adminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		updateParametersHandler(db, w, r)
	})))))

	// Обработчики для работы с отчетами
	http.Handle("/generate-report", corsMiddleware(authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			generateReportHandler(db, w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	http.Handle("/api/logs", corsMiddleware(authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logsHandler(db, w, r)
	}))))

	// Обработчик для HTML-страницы логов (возвращает HTML)
	http.Handle("/view-logs", corsMiddleware(authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "logs.html")
	}))))
	http.Handle("/handleDeleteRepor/logs", corsMiddleware(authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logsHandler(db, w, r)
	}))))
	http.Handle("/reports", corsMiddleware(authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "reports.html")
	}))))

	// API для работы с отчетами
	http.Handle("/api/reports", corsMiddleware(authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Получение списка отчетов
			rows, err := db.Query("SELECT id_report, report_name, created_at FROM Reports ORDER BY created_at DESC")
			if err != nil {
				respondWithJSON(w, http.StatusInternalServerError, map[string]string{"message": "Ошибка получения отчетов: " + err.Error()})
				return
			}
			defer rows.Close()

			var reports []map[string]interface{}
			for rows.Next() {
				var id int
				var name string
				var createdAt []byte // Сканируем как []byte вместо time.Time

				if err := rows.Scan(&id, &name, &createdAt); err != nil {
					log.Printf("Ошибка сканирования отчета: %v", err)
					continue
				}

				// Парсим дату из строки
				createTime, err := time.Parse("2006-01-02 15:04:05", string(createdAt))
				if err != nil {
					log.Printf("Ошибка парсинга даты: %v", err)
					createTime = time.Now() // Используем текущее время как fallback
				}

				reports = append(reports, map[string]interface{}{
					"id_report":   id,
					"report_name": name,
					"created_at":  createTime.Format(time.RFC3339),
				})
			}

			if err := rows.Err(); err != nil {
				respondWithJSON(w, http.StatusInternalServerError, map[string]string{"message": "Ошибка обработки отчетов: " + err.Error()})
				return
			}

			respondWithJSON(w, http.StatusOK, reports)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))

	// Добавьте в функцию main() перед запуском сервера
	http.Handle("/api/users", corsMiddleware(authMiddleware(adminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Получение списка пользователей
			rows, err := db.Query("SELECT id_user, login, rights FROM Users ORDER BY login")
			if err != nil {
				respondWithJSON(w, http.StatusInternalServerError, map[string]string{"message": "Ошибка получения пользователей: " + err.Error()})
				return
			}
			defer rows.Close()

			var users []User
			for rows.Next() {
				var u User
				if err := rows.Scan(&u.ID, &u.Login, &u.Rights); err != nil {
					log.Printf("Ошибка сканирования пользователя: %v", err)
					continue
				}
				users = append(users, u)
			}

			if err := rows.Err(); err != nil {
				respondWithJSON(w, http.StatusInternalServerError, map[string]string{"message": "Ошибка обработки пользователей: " + err.Error()})
				return
			}

			respondWithJSON(w, http.StatusOK, users)

		case http.MethodPost:
			// Создание нового пользователя
			var newUser struct {
				Login    string `json:"login"`
				Password string `json:"password"`
				Rights   string `json:"rights"`
			}

			if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
				respondWithJSON(w, http.StatusBadRequest, map[string]string{"message": "Неверный формат данных"})
				return
			}

			if newUser.Login == "" || newUser.Password == "" {
				respondWithJSON(w, http.StatusBadRequest, map[string]string{"message": "Логин и пароль обязательны"})
				return
			}

			if newUser.Rights != "admin" && newUser.Rights != "operator" {
				respondWithJSON(w, http.StatusBadRequest, map[string]string{"message": "Недопустимые права пользователя"})
				return
			}

			_, err := db.Exec("INSERT INTO Users (login, password, rights) VALUES (?, ?, ?)",
				newUser.Login, newUser.Password, newUser.Rights)
			if err != nil {
				respondWithJSON(w, http.StatusInternalServerError, map[string]string{"message": "Ошибка создания пользователя: " + err.Error()})
				return
			}

			respondWithJSON(w, http.StatusCreated, map[string]string{"message": "Пользователь создан"})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))))

	http.Handle("/api/users/", corsMiddleware(authMiddleware(adminMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/users/")
		if id == "" {
			respondWithJSON(w, http.StatusBadRequest, map[string]string{"message": "Не указан ID пользователя"})
			return
		}

		switch r.Method {
		case http.MethodPut:
			// Обновление пользователя
			var update struct {
				Rights string `json:"rights"`
			}

			if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
				respondWithJSON(w, http.StatusBadRequest, map[string]string{"message": "Неверный формат данных"})
				return
			}

			if update.Rights != "admin" && update.Rights != "operator" {
				respondWithJSON(w, http.StatusBadRequest, map[string]string{"message": "Недопустимые права пользователя"})
				return
			}

			// Проверяем, что пользователь не system
			var login string
			err := db.QueryRow("SELECT login FROM Users WHERE id_user = ?", id).Scan(&login)
			if err != nil {
				respondWithJSON(w, http.StatusNotFound, map[string]string{"message": "Пользователь не найден"})
				return
			}

			if login == "system" {
				respondWithJSON(w, http.StatusForbidden, map[string]string{"message": "Нельзя изменить системного пользователя"})
				return
			}

			_, err = db.Exec("UPDATE Users SET rights = ? WHERE id_user = ?", update.Rights, id)
			if err != nil {
				respondWithJSON(w, http.StatusInternalServerError, map[string]string{"message": "Ошибка обновления пользователя: " + err.Error()})
				return
			}

			respondWithJSON(w, http.StatusOK, map[string]string{"message": "Права пользователя обновлены"})

		case http.MethodDelete:
			// Удаление пользователя
			// Проверяем, что пользователь не system
			var login string
			err := db.QueryRow("SELECT login FROM Users WHERE id_user = ?", id).Scan(&login)
			if err != nil {
				respondWithJSON(w, http.StatusNotFound, map[string]string{"message": "Пользователь не найден"})
				return
			}

			if login == "system" {
				respondWithJSON(w, http.StatusForbidden, map[string]string{"message": "Нельзя удалить системного пользователя"})
				return
			}

			_, err = db.Exec("DELETE FROM Users WHERE id_user = ?", id)
			if err != nil {
				respondWithJSON(w, http.StatusInternalServerError, map[string]string{"message": "Ошибка удаления пользователя: " + err.Error()})
				return
			}

			respondWithJSON(w, http.StatusOK, map[string]string{"message": "Пользователь удален"})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})))))
	http.Handle("/api/reports/", corsMiddleware(authMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/reports/")
		if id == "" {
			respondWithJSON(w, http.StatusBadRequest, map[string]string{"message": "Не указан ID отчета"})
			return
		}

		switch r.Method {
		case http.MethodGet:
			if strings.HasSuffix(id, "/download") {
				id = strings.TrimSuffix(id, "/download")
				var reportData []byte
				err := db.QueryRow("SELECT File FROM Reports WHERE id_report = ?", id).Scan(&reportData)
				if err != nil {
					respondWithJSON(w, http.StatusNotFound, map[string]string{"message": "Отчет не найден"})
					return
				}

				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=report_%s.json", id))
				w.Write(reportData)
				return
			}

			var reportData []byte
			err := db.QueryRow("SELECT File FROM Reports WHERE id_report = ?", id).Scan(&reportData)
			if err != nil {
				respondWithJSON(w, http.StatusNotFound, map[string]string{"message": "Отчет не найден"})
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write(reportData)

		case http.MethodDelete:
			claims, ok := r.Context().Value("user").(*Claims)
			if !ok || claims.Rights != "admin" {
				respondWithJSON(w, http.StatusForbidden, map[string]string{"message": "Доступ запрещен"})
				return
			}

			// Проверяем существование отчета перед удалением
			var exists bool
			err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM Reports WHERE id_report = ?)", id).Scan(&exists)
			if err != nil {
				respondWithJSON(w, http.StatusInternalServerError, map[string]string{"message": "Ошибка проверки отчета"})
				return
			}

			if !exists {
				respondWithJSON(w, http.StatusNotFound, map[string]string{"message": "Отчет не найден"})
				return
			}

			// Удаляем связанные записи в логах (если есть)
			_, err = db.Exec("DELETE FROM Logs WHERE id_report = ?", id)
			if err != nil {
				log.Printf("Ошибка удаления связанных логов: %v", err)
			}

			// Удаляем сам отчет
			result, err := db.Exec("DELETE FROM Reports WHERE id_report = ?", id)
			if err != nil {
				respondWithJSON(w, http.StatusInternalServerError, map[string]string{"message": "Ошибка удаления отчета: " + err.Error()})
				return
			}

			// Проверяем, была ли удалена хотя бы одна запись
			rowsAffected, _ := result.RowsAffected()
			if rowsAffected == 0 {
				respondWithJSON(w, http.StatusNotFound, map[string]string{"message": "Отчет не найден"})
				return
			}

			respondWithJSON(w, http.StatusOK, map[string]string{"message": "Отчет успешно удален"})

		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))))
	// Запуск сервера
	go func() {
		log.Println("Server starting on :8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	// Основной цикл сбора данных
	endpoint := "opc.tcp://localhost:4840"
	timeCheck := 1.0

	for {
		start := time.Now()

		speed, weight, status, him, humidity, err := ReadData(endpoint, timeCheck)
		if err != nil {
			log.Printf("Ошибка чтения: %v", err)
			time.Sleep(time.Duration(timeCheck) * time.Second)
			continue
		}

		location, err := time.LoadLocation("Europe/Moscow")
		if err != nil {
			log.Printf("Ошибка загрузки временной зоны: %v", err)
			location = time.UTC // fallback на UTC если не удалось загрузить зону
		}

		currentTime := time.Now().In(location).Format("2006-01-02 15:04:05.000")
		measurements := []Measurement{
			{currentTime, "speed", float64(speed)},
			{currentTime, "weight", float64(weight)},
			{currentTime, "status", boolToFloat(status)},
			{currentTime, "him", him},
			{currentTime, "humidity", humidity},
		}

		tx, err := db.Begin()
		if err != nil {
			log.Printf("Ошибка транзакции: %v", err)
			continue
		}

		for _, m := range measurements {
			_, err = tx.Exec(
				"INSERT INTO Measurement (time, id_parameter, value) VALUES (?, ?, ?)",
				currentTime, parameterIDs[m.Parameter], m.Value,
			)
			if err != nil {
				log.Printf("Ошибка записи: %v", err)
				continue
			}

			if err := checkAndLogThresholdViolation(tx, m.Parameter, m.Value); err != nil {
				log.Printf("Ошибка проверки границ параметра %s: %v", m.Parameter, err)
			}
		}

		if err := tx.Commit(); err != nil {
			log.Printf("Ошибка коммита транзакции: %v", err)
		}

		elapsed := time.Since(start)
		if remaining := time.Duration(timeCheck*1000)*time.Millisecond - elapsed; remaining > 0 {
			time.Sleep(remaining)
		}
	}
}

// Вспомогательные функции

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

func logAction(db DB, userID int, actionType string, paramID interface{}, reportID interface{}, description string) error {
	query := `
        INSERT INTO Logs (id_user, time, action_type, id_parameter, id_report, description)
        VALUES (?, NOW(), ?, ?, ?, ?)
    `

	var pID, rID interface{}
	if paramID != nil {
		pID = paramID
	}
	if reportID != nil {
		rID = reportID
	}

	_, err := db.Exec(query, userID, actionType, pID, rID, description)
	return err
}

func respondWithJSON(w http.ResponseWriter, statusCode int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("Ошибка кодирования JSON: %v", err)
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			next.ServeHTTP(w, r)
			return
		}

		cookie, err := r.Cookie("token")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(cookie.Value, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		ctx := context.WithValue(r.Context(), "user", claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func adminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value("user").(*Claims)
		if !ok || claims.Rights != "admin" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
