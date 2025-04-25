package main

import (
	"context"
	"fmt"

	//"log"
	"time"

	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

// ReadData подключается к OPC UA серверу по заданному endpoint,
// ждёт timechek секунд, выполняет чтение заданных переменных и возвращает их значения.
func ReadData(endpoint string, timechek float64) (speed int, weight int, status bool, him float64, humidity float64, err error) {
	ctx := context.Background()

	// Создаем клиента
	c, err := opcua.NewClient(endpoint, opcua.SecurityMode(ua.MessageSecurityModeNone))
	if err != nil {
		return 0, 0, false, 0.0, 0.0, fmt.Errorf("не удалось создать клиент: %v", err)
	}

	// Подключаемся к серверу
	if err := c.Connect(ctx); err != nil {
		return 0, 0, false, 0.0, 0.0, fmt.Errorf("ошибка подключения: %v", err)
	}
	defer c.Close(ctx)

	// Задаем NodeID для переменных (Numeric NodeID)
	nodeIDs := map[string]string{
		"speed":    "ns=2;i=2",
		"weight":   "ns=2;i=3",
		"status":   "ns=2;i=4",
		"him":      "ns=2;i=5",
		"humidity": "ns=2;i=6",
	}

	nodes := make(map[string]*ua.NodeID)
	for name, id := range nodeIDs {
		nid, err := ua.ParseNodeID(id)
		if err != nil {
			return 0, 0, false, 0.0, 0.0, fmt.Errorf("ошибка парсинга NodeID %s: %v", id, err)
		}
		nodes[name] = nid
		//log.Printf("Зарегистрирован NodeID для %s: %v", name, nid)
	}

	// Ждем указанное время перед считыванием
	time.Sleep(time.Duration(timechek) * time.Second)

	// Формируем запрос на чтение
	req := &ua.ReadRequest{
		NodesToRead: []*ua.ReadValueID{
			{NodeID: nodes["speed"]},
			{NodeID: nodes["weight"]},
			{NodeID: nodes["status"]},
			{NodeID: nodes["him"]},
			{NodeID: nodes["humidity"]},
		},
	}

	resp, err := c.Read(ctx, req)
	if err != nil {
		return 0, 0, false, 0.0, 0.0, fmt.Errorf("ошибка чтения: %v", err)
	}

	if len(resp.Results) < 5 {
		return 0, 0, false, 0.0, 0.0, fmt.Errorf("недостаточное количество результатов")
	}

	// Извлекаем значение для "position"
	var ok bool
	speed, ok = resp.Results[0].Value.Value().(int)
	if !ok {
		// Может быть int32 или int64
		switch v := resp.Results[0].Value.Value().(type) {
		case int32:
			speed = int(v)
		case int64:
			speed = int(v)
		default:
			return 0, 0, false, 0.0, 0.0, fmt.Errorf("неверный тип для speed")
		}
	}

	// Извлекаем значение для "weight"
	weight, ok = resp.Results[1].Value.Value().(int)
	if !ok {
		switch v := resp.Results[1].Value.Value().(type) {
		case int32:
			weight = int(v)
		case int64:
			weight = int(v)
		default:
			return 0, 0, false, 0.0, 0.0, fmt.Errorf("неверный тип для weight")
		}
	}

	// Извлекаем значение для "status"
	status, ok = resp.Results[2].Value.Value().(bool)
	if !ok {
		return 0, 0, false, 0.0, 0.0, fmt.Errorf("неверный тип для status")
	}

	// Извлекаем значение для "bunker"
	if him, ok = resp.Results[3].Value.Value().(float64); !ok {
		switch v := resp.Results[3].Value.Value().(type) {
		case float32:
			him = float64(v)
		default:
			return 0, 0, false, 0.0, 0.0, fmt.Errorf("неверный тип для bunker")
		}
	}

	// Извлекаем значение для "humidity"
	if humidity, ok = resp.Results[4].Value.Value().(float64); !ok {
		switch v := resp.Results[4].Value.Value().(type) {
		case float32:
			humidity = float64(v)
		default:
			return 0, 0, false, 0.0, 0.0, fmt.Errorf("неверный тип для humidity")
		}
	}

	return speed, weight, status, him, humidity, nil
}
