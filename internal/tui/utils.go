package tui

import (
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
	default:
		return styles.pendingGlyph.Inherit(initialStyle).Render()
	}
}
