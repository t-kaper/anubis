package dtb

import (
	"context"
	"time"
)

func DoWithAttemptsCtx(ctx context.Context, fn func() error, attempts int, delay time.Duration) error {
	var err error

	// Создаём таймер ОДИН раз, а не на каждой итерации
	timer := time.NewTimer(delay)
	defer timer.Stop() // Гарантируем освобождение ресурсов

	for i := 0; i < attempts; i++ {
		// Проверяем, не отменён ли контекст перед попыткой
		select {
		case <-ctx.Done():
			return ctx.Err() // Контекст отменён - выходим
		default:
			// Контекст активен, продолжаем
		}

		// Выполняем операцию
		err = fn()
		if err == nil {
			return nil // Успех! Возвращаем nil
		}

		// Если это была последняя попытка - не ждём, сразу выходим
		if i == attempts-1 {
			break
		}

		// Сбрасываем таймер на новую задержку
		// Важно: если таймер уже сработал, очищаем канал
		if !timer.Stop() {
			select {
			case <-timer.C: // Вычитываем значение, если оно есть
			default: // Если канал пустой - ничего не делаем
			}
		}
		timer.Reset(delay) // Перезапускаем таймер

		// Ждём либо отмену контекста, либо срабатывание таймера
		select {
		case <-ctx.Done():
			return ctx.Err() // Контекст отменён во время ожидания
		case <-timer.C:
			// Таймер сработал - продолжаем следующую попытку
		}
	}

	return err // Все попытки исчерпаны, возвращаем последнюю ошибку
}

func DoWithAttempts(callback func() error, maxAttempts int, delay time.Duration) error {
	// function executes callback several times
	var err error
	for maxAttempts > 0 {
		if err = callback(); err != nil {
			time.Sleep(delay)
			maxAttempts--
			continue
		}
		return nil
	}
	return err
}
