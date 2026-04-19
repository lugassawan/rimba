package cmd

import (
	"fmt"

	"github.com/lugassawan/rimba/internal/gh"
	"github.com/lugassawan/rimba/internal/output"
	"github.com/lugassawan/rimba/internal/resolver"
	"github.com/lugassawan/rimba/internal/termcolor"
	"github.com/spf13/cobra"
)

func listRenderEmpty(cmd *cobra.Command, msg string) error {
	if isJSON(cmd) {
		return output.WriteJSON(cmd.OutOrStdout(), version, "list", make([]output.ListItem, 0))
	}
	fmt.Fprintln(cmd.OutOrStdout(), msg)
	return nil
}

func listRenderJSON(cmd *cobra.Command, rows []resolver.WorktreeDetail, prInfos prInfoMap) error {
	items := make([]output.ListItem, len(rows))
	for i, r := range rows {
		items[i] = output.ListItem{
			Task:      r.Task,
			Service:   r.Service,
			Type:      r.Type,
			Branch:    r.Branch,
			Path:      r.Path,
			IsCurrent: r.IsCurrent,
			Status:    r.Status,
		}
		if info, ok := prInfos[r.Branch]; ok {
			items[i].PRNumber = info.number
			if info.status != nil {
				s := string(*info.status)
				items[i].CIStatus = &s
			}
		}
	}
	return output.WriteJSON(cmd.OutOrStdout(), version, "list", items)
}

func listRenderTable(cmd *cobra.Command, rows []resolver.WorktreeDetail, full bool, prInfos prInfoMap, ghWarning string) {
	hasService := resolver.HasService(rows)
	noColor, _ := cmd.Flags().GetBool(flagNoColor)
	p := termcolor.NewPainter(noColor)

	if ghWarning != "" {
		fmt.Fprintln(cmd.OutOrStdout(), p.Paint(ghWarning, termcolor.Yellow))
	}

	tbl := termcolor.NewTable(2)
	tbl.AddRow(listHeader(p, hasService, full)...)

	for _, row := range rows {
		taskCell := "  " + row.Task
		if row.IsCurrent {
			taskCell = "* " + row.Task
			taskCell = p.Paint(taskCell, termcolor.Green, termcolor.Bold)
		}

		typeCell := row.Type
		if c := typeColor(row.Type); c != "" {
			typeCell = p.Paint(typeCell, c)
		}

		statusCell := colorStatus(p, row.Status)
		cells := listRow(taskCell, row, typeCell, statusCell, hasService, full)
		if full {
			info := prInfos[row.Branch]
			cells = append(cells, formatPRCell(info.number, p), formatCICell(info.status, p))
		}
		tbl.AddRow(cells...)
	}

	tbl.Render(cmd.OutOrStdout())
}

func listHeader(p *termcolor.Painter, hasService, full bool) []string {
	h := []string{p.Paint("TASK", termcolor.Bold)}
	if hasService {
		h = append(h, p.Paint("SERVICE", termcolor.Bold))
	}
	h = append(h, p.Paint("TYPE", termcolor.Bold))
	if full {
		h = append(h, p.Paint("BRANCH", termcolor.Bold), p.Paint("PATH", termcolor.Bold))
	}
	h = append(h, p.Paint("STATUS", termcolor.Bold))
	if full {
		h = append(h, p.Paint("PR", termcolor.Bold), p.Paint("CI", termcolor.Bold))
	}
	return h
}

func listRow(taskCell string, row resolver.WorktreeDetail, typeCell, statusCell string, hasService, full bool) []string {
	cells := []string{taskCell}
	if hasService {
		cells = append(cells, row.Service)
	}
	cells = append(cells, typeCell)
	if full {
		cells = append(cells, row.Branch, row.Path)
	}
	cells = append(cells, statusCell)
	return cells
}

func formatPRCell(n *int, p *termcolor.Painter) string {
	if n == nil {
		return p.Paint("–", termcolor.Gray)
	}
	return fmt.Sprintf("#%d", *n)
}

func formatCICell(status *gh.CIStatus, p *termcolor.Painter) string {
	if status == nil {
		return p.Paint("–", termcolor.Gray)
	}
	switch *status {
	case gh.CIStatusSuccess:
		return p.Paint("✓", termcolor.Green)
	case gh.CIStatusPending:
		return p.Paint("●", termcolor.Yellow)
	case gh.CIStatusFailure:
		return p.Paint("✗", termcolor.Red)
	default:
		return p.Paint("–", termcolor.Gray)
	}
}
