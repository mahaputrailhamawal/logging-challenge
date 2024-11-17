package main

import (
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	f, err := os.OpenFile(
		"logs/app.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644,
	)

	if err != nil {
		log.Fatal().Err(err).Msg("Failed to open log file")
	}

	defer f.Close()

	multiLevelWriter := zerolog.MultiLevelWriter(os.Stdout, f)
	log.Logger = zerolog.New(multiLevelWriter).With().Timestamp().Logger()

	log.Debug().
		Int("num_tickets", 10).
		Msg("Booking created")
}
