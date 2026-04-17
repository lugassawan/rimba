package operations

// DiskFootprint is the total disk usage of a rimba repo.
// MainErr != nil means MainBytes could not be computed and is zero.
type DiskFootprint struct {
	TotalBytes     int64
	MainBytes      int64
	MainErr        error
	WorktreesBytes int64
}

// BuildDiskFootprint sums worktreeSizes (nils are skipped) plus mainSize.
// When mainErr != nil, MainBytes stays zero and mainErr is preserved.
func BuildDiskFootprint(worktreeSizes []*int64, mainSize int64, mainErr error) DiskFootprint {
	fp := DiskFootprint{MainErr: mainErr}
	if mainErr == nil {
		fp.MainBytes = mainSize
	}
	for _, s := range worktreeSizes {
		if s != nil {
			fp.WorktreesBytes += *s
		}
	}
	fp.TotalBytes = fp.MainBytes + fp.WorktreesBytes
	return fp
}
