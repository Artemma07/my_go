package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func loginHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.ServeFile(w, r, "login.html")
			return
		}

		var creds struct {
			Login    string `json:"login"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		var user struct {
			ID     int
			Rights string
		}
		err := db.QueryRow("SELECT id_user, rights FROM Users WHERE login = ? AND password = ?", creds.Login, creds.Password).
			Scan(&user.ID, &user.Rights)
		if err != nil {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}

		if err := logAction(db, user.ID, "login", nil, nil, "Успешный вход в систему"); err != nil {
			log.Printf("Ошибка логирования входа: %v", err)
		}

		claims := &Claims{
			Login:  creds.Login,
			Rights: user.Rights,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			},
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, err := token.SignedString([]byte(jwtSecret))
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:    "token",
			Value:   tokenString,
			Expires: time.Now().Add(24 * time.Hour),
			Path:    "/",
		})
		w.WriteHeader(http.StatusOK)
	}
}

func logoutHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := r.Context().Value("user").(*Claims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var userID int
		err := db.QueryRow("SELECT id_user FROM Users WHERE login = ?", claims.Login).Scan(&userID)
		if err != nil {
			http.Error(w, "User not found", http.StatusInternalServerError)
			return
		}

		if err := logAction(db, userID, "logout", nil, nil, "Выход из системы"); err != nil {
			log.Printf("Ошибка логирования выхода: %v", err)
		}

		http.SetCookie(w, &http.Cookie{
			Name:    "token",
			Value:   "",
			Expires: time.Unix(0, 0),
			Path:    "/",
		})
		w.WriteHeader(http.StatusOK)
	}
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

func serveAdmin(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "admin.html")
}

func parametersHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT parameter_name, min_threshold, max_threshold FROM Parameters")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var params []map[string]interface{}
	for rows.Next() {
		var p struct {
			Name string
			Min  float64
			Max  float64
		}
		if err := rows.Scan(&p.Name, &p.Min, &p.Max); err != nil {
			continue
		}
		params = append(params, map[string]interface{}{
			"parameter_name": p.Name,
			"min_threshold":  p.Min,
			"max_threshold":  p.Max,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(params)
}

func updateParametersHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value("user").(*Claims)
	if !ok || claims.Rights != "admin" {
		respondWithJSON(w, http.StatusForbidden, map[string]string{"message": "Доступ запрещен"})
		return
	}

	var userID int
	err := db.QueryRow("SELECT id_user FROM Users WHERE login = ?", claims.Login).Scan(&userID)
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, map[string]string{"message": "Ошибка получения ID пользователя"})
		return
	}

	var params []struct {
		ParameterName string  `json:"parameter_name"`
		MinThreshold  float64 `json:"min_threshold"`
		MaxThreshold  float64 `json:"max_threshold"`
	}

	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		respondWithJSON(w, http.StatusBadRequest, map[string]string{"message": "Неверный формат данных"})
		return
	}

	tx, err := db.Begin()
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError, map[string]string{"message": "Ошибка начала транзакции"})
		return
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	var changes []string
	var firstParamID int

	for _, p := range params {
		var oldMin, oldMax float64
		err = tx.QueryRow(
			"SELECT min_threshold, max_threshold FROM Parameters WHERE parameter_name = ?",
			p.ParameterName,
		).Scan(&oldMin, &oldMax)
		if err != nil {
			respondWithJSON(w, http.StatusBadRequest, map[string]string{"message": fmt.Sprintf("Параметр %s не найден", p.ParameterName)})
			return
		}

		_, err = tx.Exec(
			"UPDATE Parameters SET min_threshold = ?, max_threshold = ? WHERE parameter_name = ?",
			p.MinThreshold, p.MaxThreshold, p.ParameterName,
		)
		if err != nil {
			respondWithJSON(w, http.StatusInternalServerError, map[string]string{
				"message": fmt.Sprintf("Ошибка обновления параметра %s: %v", p.ParameterName, err),
			})
			return
		}

		if firstParamID == 0 {
			err = tx.QueryRow(
				"SELECT id_parameter FROM Parameters WHERE parameter_name = ?",
				p.ParameterName,
			).Scan(&firstParamID)
			if err != nil {
				respondWithJSON(w, http.StatusInternalServerError, map[string]string{"message": "Ошибка получения ID параметра"})
				return
			}
		}

		changes = append(changes, fmt.Sprintf(
			"%s: min %.2f→%.2f, max %.2f→%.2f",
			p.ParameterName, oldMin, p.MinThreshold, oldMax, p.MaxThreshold,
		))
	}

	if len(changes) > 0 {
		description := "Изменены параметры: " + strings.Join(changes, "; ")
		if err := logAction(tx, userID, "parameter_update", firstParamID, nil, description); err != nil {
			log.Printf("Ошибка логирования изменений параметров: %v", err)
			respondWithJSON(w, http.StatusInternalServerError, map[string]string{
				"message": "Ошибка логирования изменений",
			})
			return
		}
	}

	if err := tx.Commit(); err != nil {
		respondWithJSON(w, http.StatusInternalServerError, map[string]string{"message": "Ошибка сохранения изменений"})
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"message": "Параметры успешно обновлены"})
}

func generateReportHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	// Проверка авторизации
	claims, ok := r.Context().Value("user").(*Claims)
	if !ok {
		respondWithJSON(w, http.StatusUnauthorized, map[string]string{"message": "Не авторизован"})
		return
	}

	// Получение userID из базы данных по логину
	var userID int
	err := db.QueryRow("SELECT id_user FROM Users WHERE login = ?", claims.Login).Scan(&userID)
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError,
			map[string]string{"message": "Ошибка получения ID пользователя: " + err.Error()})
		return
	}

	// Получение параметров периода
	var request struct {
		DateFrom string `json:"dateFrom"`
		DateTo   string `json:"dateTo"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		respondWithJSON(w, http.StatusBadRequest, map[string]string{"message": "Неверный формат данных"})
		return
	}
	// Валидация дат
	if request.DateFrom == "" || request.DateTo == "" {
		respondWithJSON(w, http.StatusBadRequest, map[string]string{"message": "Необходимо указать период отчета"})
		return
	}

	// Подсчет расфасованных пачек
	totalPackages, err := countPackagedBags(db, request.DateFrom, request.DateTo)
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError,
			map[string]string{"message": "Ошибка подсчета готовых изделмй: " + err.Error()})
		return
	}

	// Подсчет бракованных пачек
	defectivePackages, err := countDefectiveBags(db, request.DateFrom, request.DateTo)
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError,
			map[string]string{"message": "Ошибка подсчета брака: " + err.Error()})
		return
	}

	// Расчет процента брака
	defectivePercent := 0.0
	if totalPackages > 0 {
		defectivePercent = float64(defectivePackages) / float64(totalPackages) * 100
	}

	// Формирование отчета
	reportData := map[string]interface{}{
		"date_from":            request.DateFrom,
		"date_to":              request.DateTo,
		"total_packages":       totalPackages,
		"defective_packages":   defectivePackages,
		"defective_percentage": defectivePercent,
	}

	reportJSON, err := json.Marshal(reportData)
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError,
			map[string]string{"message": "Ошибка формирования отчета: " + err.Error()})
		return
	}

	// Сохранение отчета в БД
	reportName := fmt.Sprintf("Отчет за период %s - %s", request.DateFrom, request.DateTo)
	res, err := db.Exec(`
        INSERT INTO Reports (report_name, File, created_at) 
        VALUES (?, ?, NOW())`,
		reportName,
		string(reportJSON),
	)
	if err != nil {
		respondWithJSON(w, http.StatusInternalServerError,
			map[string]string{"message": "Ошибка сохранения отчета: " + err.Error()})
		return
	}

	reportID, _ := res.LastInsertId()

	// Логирование действия
	if err := logAction(db, userID, "report_generated", nil, reportID,
		fmt.Sprintf("Создан отчет: %d изделий, %d брак (%.2f%%)",
			totalPackages, defectivePackages, defectivePercent)); err != nil {
		log.Printf("Ошибка логирования: %v", err)
	}

	// Ответ клиенту
	respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"message":      "Отчет успешно создан",
		"report_id":    reportID,
		"report_data":  reportData,
		"generated_at": time.Now().Format(time.RFC3339),
	})
}

// Дополнительная функция для подсчета бракованных пачек
func countDefectiveBags(db *sql.DB, dateFrom, dateTo string) (int, error) {
	var count int
	err := db.QueryRow(`
        SELECT COUNT(*) 
        FROM Logs 
        WHERE action_type = 'threshold_alert' 
        AND id_parameter = ? 
        AND time BETWEEN ? AND ?`,
		parameterIDs["status"],
		dateFrom,
		dateTo,
	).Scan(&count)
	return count, err
}

func countPackagedBags(db *sql.DB, dateFrom, dateTo string) (int, error) {
	var count int

	query := `
        SELECT COUNT(*) 
        FROM (
            SELECT m1.time, m1.value as current_val,
                   (SELECT m2.value 
                    FROM Measurement m2 
                    WHERE m2.id_parameter = m1.id_parameter 
                    AND m2.time < m1.time 
                    ORDER BY m2.time DESC 
                    LIMIT 1) as prev_val
            FROM Measurement m1
            WHERE m1.id_parameter = ?
            AND m1.time BETWEEN ? AND ?
            AND m1.value = 1
        ) t
        WHERE prev_val = 0 OR prev_val IS NULL`

	err := db.QueryRow(query, parameterIDs["status"], dateFrom, dateTo).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("ошибка запроса: %v", err)
	}
	log.Println(dateFrom, dateTo)
	log.Println("количество пачек = ", count)

	return count, nil
}

func logsHandler(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	actionType := r.URL.Query().Get("actionType")
	dateFrom := r.URL.Query().Get("dateFrom")
	dateTo := r.URL.Query().Get("dateTo")

	query := `
        SELECT 
            l.time, 
            u.login as user_login, 
            l.action_type, 
            l.description 
        FROM Logs l
        LEFT JOIN Users u ON l.id_user = u.id_user
        WHERE 1=1
    `

	var args []interface{}
	var conditions []string

	if actionType != "" {
		conditions = append(conditions, "l.action_type = ?")
		args = append(args, actionType)
	}
	if dateFrom != "" {
		conditions = append(conditions, "l.time >= ?")
		args = append(args, dateFrom)
	}
	if dateTo != "" {
		conditions = append(conditions, "l.time <= ?")
		args = append(args, dateTo)
	}

	if len(conditions) > 0 {
		query += " AND " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY l.time DESC LIMIT 100"

	rows, err := db.Query(query, args...)
	if err != nil {
		log.Printf("Ошибка запроса логов: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var logs []map[string]interface{}
	for rows.Next() {
		var timeBytes []byte
		var userLogin sql.NullString
		var actionType string
		var description sql.NullString

		if err := rows.Scan(&timeBytes, &userLogin, &actionType, &description); err != nil {
			log.Printf("Ошибка сканирования лога: %v", err)
			continue
		}

		timeStr := string(timeBytes)
		loc, _ := time.LoadLocation("Local")
		parsedTime, err := time.ParseInLocation("2006-01-02 15:04:05", timeStr, loc)
		if err != nil {
			log.Printf("Ошибка парсинга времени: %v", err)
			continue
		}
		logs = append(logs, map[string]interface{}{
			"time":        parsedTime.Format(time.RFC3339),
			"user_login":  userLogin.String,
			"action_type": actionType,
			"description": description.String,
		})
	}

	if err := rows.Err(); err != nil {
		log.Printf("Ошибка после обработки логов: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if len(logs) == 0 {
		logs = []map[string]interface{}{}
	}

	if err := json.NewEncoder(w).Encode(logs); err != nil {
		log.Printf("Ошибка кодирования JSON: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

type DB interface {
	QueryRow(query string, args ...interface{}) *sql.Row
	Exec(query string, args ...interface{}) (sql.Result, error)
	Query(query string, args ...interface{}) (*sql.Rows, error)
}

func checkAndLogThresholdViolation(db DB, parameterName string, value float64) error {
	// Логируем только для параметра status
	if parameterName != "status" {
		return nil
	}

	// Проверяем изменение с 0 на 1
	if value != 1 {
		return nil
	}

	// Получаем границы weight и текущее значение
	var weightMin, weightMax, currentWeight float64
	err := db.QueryRow(`
        SELECT p.min_threshold, p.max_threshold, m.value 
        FROM Parameters p
        JOIN (
            SELECT value FROM Measurement 
            WHERE id_parameter = ? 
            ORDER BY time DESC LIMIT 1
        ) m
        WHERE p.parameter_name = 'weight'`,
		parameterIDs["weight"],
	).Scan(&weightMin, &weightMax, &currentWeight)
	if err != nil {
		return fmt.Errorf("failed to get weight data: %v", err)
	}

	// Проверяем выход weight за границы
	if currentWeight >= weightMin && currentWeight <= weightMax {
		return nil
	}

	// Проверяем предыдущее значение status
	var prevStatus float64
	err = db.QueryRow(`
        SELECT value FROM Measurement 
        WHERE id_parameter = ? 
        ORDER BY time DESC LIMIT 1,1`,
		parameterIDs["status"],
	).Scan(&prevStatus)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get previous status value: %v", err)
	}

	// Логируем только если было изменение 0→1
	if prevStatus != 0 {
		return nil
	}
	// Добавить проверку влажности

	var currentHumidity float64
	err = db.QueryRow(`
    SELECT value FROM Measurement 
    WHERE id_parameter = ? 
    ORDER BY time DESC LIMIT 1`,
		parameterIDs["humidity"],
	).Scan(&currentHumidity)

	if currentWeight < weightMin || currentWeight > weightMax || currentHumidity > 15.0 {
		// Логировать брак
	}

	// Получаем ID системного пользователя
	var systemUserID int
	err = db.QueryRow("SELECT id_user FROM Users WHERE login = 'system'").Scan(&systemUserID)
	if err != nil {
		return fmt.Errorf("failed to get system user ID: %v", err)
	}

	// Формируем сообщение с реальными значениями
	description := fmt.Sprintf(
		"Выход weight за пределы значений (min: %.2f, max: %.2f, текущее: %.2f)",
		weightMin,
		weightMax,
		currentWeight,
	)

	// Создаем запись в логах
	_, err = db.Exec(`
        INSERT INTO Logs (id_user, time, action_type, id_parameter, description)
        VALUES (?, NOW(), ?, ?, ?)`,
		systemUserID,
		"threshold_alert",
		parameterIDs["status"],
		description,
	)

	if err != nil {
		return fmt.Errorf("failed to log violation: %v", err)
	}

	return nil
}
