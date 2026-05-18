package operations

// Plan records planned steps and gates their execution under dry-run mode.
type Plan struct {
	DryRun bool
	Steps  []string
}

// Do appends desc to Steps. When DryRun is false, action is also called and
// its error returned. When DryRun is true, action is skipped and nil returned.
func (p *Plan) Do(desc string, action func() error) error {
	p.Steps = append(p.Steps, desc)
	if p.DryRun {
		return nil
	}
	return action()
}
