package db

import (
	"errors"
	"log/slog"
	"time"
)

func InitDBWithRetry(name string, initFunc func() error) error {
	const maxRetries = 25
	const retryInterval = 30 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := initFunc()
		if err == nil {
			return nil
		}
		if attempt == maxRetries {
			slog.Error("Error connecting to "+name+" DB, final attempt failed",
				slog.Any("error", err),
				slog.Int("attempt", attempt),
				slog.Int("max_retries", maxRetries))
		} else {
			slog.Error("Error connecting to "+name+" DB, retrying in 30 seconds...",
				slog.Any("error", err),
				slog.Int("attempt", attempt),
				slog.Int("max_retries", maxRetries))
			time.Sleep(retryInterval)
		}
	}
	return errors.New("failed to connect to " + name + " DB after all retries")
}
