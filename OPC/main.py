import time
import csv
from opcua import ua, Server


def main():
    # Создаем OPC UA сервер и задаем конечную точку
    server = Server()
    server.set_endpoint("opc.tcp://localhost:4840")

    # Регистрируем пространство имен
    namespace = "http://examples.freeopcua.github.io"
    idx = server.register_namespace(namespace)

    # Получаем корневой узел объектов
    objects = server.get_objects_node()

    # Создаем объект для виртуальных датчиков
    sensors = objects.add_object(idx, "VirtualSensors")

    # Добавляем переменные (датчики) с начальными значениями
    sensor_pod = sensors.add_variable(idx, "Скорость подачи", 0)
    sensor_weight = sensors.add_variable(idx, "Вес смеси", 0)
    sensor_status = sensors.add_variable(idx, "Статус заполнения", False)
    sensor_him = sensors.add_variable(idx, "Уровень химикатов", 0)
    sensor_humidity = sensors.add_variable(idx, "Влажность", 0)

    # Разрешаем запись значений для переменных
    sensor_pod.set_writable()
    sensor_weight.set_writable()
    sensor_status.set_writable()
    sensor_him.set_writable()
    sensor_humidity.set_writable()

    # Запускаем сервер
    server.start()
    print("OPC UA сервер запущен на {}".format(server.endpoint))

    try:
        # Читаем данные из файла data.csv
        with open("data.csv", "r", encoding="utf-8") as csvfile:
            reader = csv.DictReader(csvfile)
            for row in reader:
                try:
                    # Извлекаем и преобразуем данные из CSV
                    pos = int(row["screw_position"])
                    weight = int(row["weight"])

                    # Преобразование для булевого значения
                    package_str = row["package_availability"].strip().lower()
                    if package_str in ["true", "1", "yes"]:
                        package = True
                    else:
                        package = False

                    # Здесь значения могут быть вещественными, поэтому преобразуем в float
                    bunker = float(row["salt_level"])
                    humidity = float(row["moisture"])
                except (ValueError, KeyError) as e:
                    print("Ошибка при чтении строки:", row, "Ошибка:", e)
                    continue

                # Устанавливаем значения переменных
                sensor_pod.set_value(pos)
                sensor_weight.set_value(weight)
                sensor_status.set_value(package)
                sensor_him.set_value(bunker)
                sensor_humidity.set_value(humidity)

                # Выводим значения в консоль
                print(
                    f"[Обновление] Скорость: {pos}, Вес: {weight}, Статус заполнения: {package}, Уровень химикатов: {bunker}, Влажность: {humidity}")
                print("Сервер запущен, доступные переменные:")
                print(f"  Скорость: {sensor_pod.nodeid}")
                print(f"  Вес смеси: {sensor_weight.nodeid}")
                print(f"  Статус заполнения: {sensor_status.nodeid}")
                print(f"  Уровень химикатов: {sensor_him.nodeid}")
                print(f"  Влажность: {sensor_humidity.nodeid}")

                # Пауза 1 секунда между обновлениями
                time.sleep(0.05)
    finally:
        server.stop()
        print("OPC UA сервер остановлен")


if __name__ == "__main__":
    main()
