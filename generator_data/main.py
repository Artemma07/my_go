import csv
import random

# Параметры генерации данных
PERIOD_DURATION = 6  # Длительность периода в секундах
UPDATE_INTERVAL = 0.05  # Интервал обновления данных в секундах
STEPS_PER_PERIOD = int(PERIOD_DURATION / UPDATE_INTERVAL)  # Шагов в периоде (120)
TOTAL_PERIODS = 1000  # Общее количество периодов
TOTAL_STEPS = TOTAL_PERIODS * STEPS_PER_PERIOD  # Общее количество шагов (120,000)

# Открытие CSV файла для записи
with open('data.csv', 'w', newline='') as csvfile:
    csv_writer = csv.writer(csvfile)

    # Запись заголовков
    csv_writer.writerow([
        'time',
        'speed',
        'weight',
        'status',
        'him_level',
        'moisture'
    ])

    # Инициализация переменных
    current_weight = random.randint(960, 1025)


    for period in range(TOTAL_PERIODS):
        # Обновляем вес каждый период
        if period > 0:
            current_weight = random.randint(960, 1025)

        for step in range(STEPS_PER_PERIOD):
            # Расчет текущего времени
            current_step = period * STEPS_PER_PERIOD + step
            time = round(current_step * UPDATE_INTERVAL, 2)

            #изменения скорости
            if step < 40:  # Первые 2 секунды (40 шагов)
                speed = 0
            else:  # Линейный рост 0→720 за 80 шагов
                speed = (step - 40) * 9


            status = step >= 40

            # Уровень соли и влажности (линейные изменения за 1000 периодов)
            progress = current_step / TOTAL_STEPS
            him_level = 90 - (5 * progress)
            moisture = 20 + (70 * progress)

            # Запись данных
            csv_writer.writerow([
                time,
                speed,
                current_weight,
                status,
                round(him_level, 2),
                round(moisture, 2)
            ])