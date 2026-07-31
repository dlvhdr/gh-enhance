package tui

import (
	"math"
	"strconv"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/dlvhdr/gh-enhance/internal/data"
)

func bucketToIcon(bucket data.CheckBucket, initialStyle lipgloss.Style, styles styles) string {
	switch bucket {
	case data.CheckBucketPass:
		return styles.successGlyph.Inherit(initialStyle).Render()
	case data.CheckBucketFail:
		return styles.failureGlyph.Inherit(initialStyle).Render()
	case data.CheckBucketNeutral:
		return styles.neutralGlyph.Inherit(initialStyle).Render()
	case data.CheckBucketSkipping:
		return styles.skippedGlyph.Inherit(initialStyle).Render()
	case data.CheckBucketCancel:
		return styles.canceledGlyph.Inherit(initialStyle).Render()
	case data.CheckBucketActionRequired:
		return styles.warningGlyph.Inherit(initialStyle).Render()
	default:
		return styles.pendingGlyph.Inherit(initialStyle).Render()
	}
}

const (
	ApproxDaysInYear  = 365
	ApproxDaysInMonth = 28
	DaysInWeek        = 7
)

func TimeElapsed(then time.Time) string {
	var parts []string
	var text string

	now := time.Now()
	diff := now.Sub(then)
	day := math.Round(diff.Hours() / 24)
	year := math.Round(day / ApproxDaysInYear)
	month := math.Round(day / ApproxDaysInMonth)
	week := math.Round(day / DaysInWeek)
	hour := math.Round(math.Abs(diff.Hours()))
	minute := math.Round(math.Abs(diff.Minutes()))
	second := math.Round(math.Abs(diff.Seconds()))

	if year > 0 {
		parts = append(parts, strconv.Itoa(int(year))+"y")
	}

	if month > 0 {
		parts = append(parts, strconv.Itoa(int(month))+"mo")
	}

	if week > 0 {
		parts = append(parts, strconv.Itoa(int(week))+"w")
	}

	if day > 0 {
		parts = append(parts, strconv.Itoa(int(day))+"d")
	}

	if hour > 0 {
		parts = append(parts, strconv.Itoa(int(hour))+"h")
	}

	if minute > 0 {
		parts = append(parts, strconv.Itoa(int(minute))+"m")
	}

	if second > 0 {
		parts = append(parts, strconv.Itoa(int(second))+"s")
	}

	if len(parts) == 0 {
		return "now"
	}

	return parts[0] + text
}
