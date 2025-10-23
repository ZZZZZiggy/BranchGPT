package logging

import (
	"log/slog"
	"os"
)

var Logger *slog.Logger

func Init() {
	env := os.Getenv("APP_ENV")
	if env == "prod" {
		Logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	} else {
		Logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}

}
