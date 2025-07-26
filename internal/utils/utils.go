package utils

import (
	"fmt"
	"time"

	"github.com/charmbracelet/log/v2"
)

func TimeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Debug(fmt.Sprintf("🕐 %s took %s", name, elapsed))
}
