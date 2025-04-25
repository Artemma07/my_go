package main

import (
	"testing"
)

func TestReadData_RealServer(t *testing.T) {
	endpoint := "opc.tcp://localhost:4840" // Укажите ваш реальный endpoint
	timechek := 1.0

	speed, weight, status, him, humidity, err := ReadData(endpoint, timechek)
	if err != nil {
		t.Fatalf("Ошибка при чтении данных с OPC UA сервера: %v", err)
	}

	t.Logf("Полученные данные: speed=%d, weight=%d, status=%t, him=%f, humidity=%f",
		speed, weight, status, him, humidity)
}

func TestReadData_InvalidEndpoint(t *testing.T) {
	endpoint := "opc.tcp://invalid:4840" // Несуществующий сервер
	timechek := 1.0

	_, _, _, _, _, err := ReadData(endpoint, timechek)
	if err == nil {
		t.Fatal("Ожидалась ошибка подключения к неверному endpoint, но её не произошло")
	}
}

func TestReadData_LongWait(t *testing.T) {
	endpoint := "opc.tcp://localhost:4840" // Укажите ваш реальный endpoint
	timechek := 5.0                        // Увеличенное время ожидания

	speed, weight, status, him, humidity, err := ReadData(endpoint, timechek)
	if err != nil {
		t.Fatalf("Ошибка при чтении данных с долгим ожиданием: %v", err)
	}

	t.Logf("Полученные данные после долгого ожидания: speed=%d, weight=%d, status=%t, him=%f, humidity=%f",
		speed, weight, status, him, humidity)
}

func TestReadData_CheckValuesRange(t *testing.T) {
	endpoint := "opc.tcp://localhost:4840" // Укажите ваш реальный endpoint
	timechek := 1.0

	speed, weight, _, him, humidity, err := ReadData(endpoint, timechek)
	if err != nil {
		t.Fatalf("Ошибка при чтении данных: %v", err)
	}

	if speed < 0 || weight < 0 || him < 0.0 || humidity < 0.0 {
		t.Errorf("Некоторые значения находятся вне допустимого диапазона: speed=%d, weight=%d, him=%f, humidity=%f",
			speed, weight, him, humidity)
	}
}
