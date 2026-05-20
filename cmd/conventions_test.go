package cmd

import (
	"testing"
	"unicode"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// allCommands returns cmd and all its descendants recursively.
func allCommands(c *cobra.Command) []*cobra.Command {
	result := make([]*cobra.Command, 0, 1+len(c.Commands()))
	result = append(result, c)
	for _, sub := range c.Commands() {
		result = append(result, allCommands(sub)...)
	}
	return result
}

func TestFlagUsageIsLowercase(t *testing.T) {
	for _, c := range allCommands(rootCmd) {
		// LocalFlags covers this command's own Flags() and PersistentFlags(),
		// but not flags inherited from ancestor commands — those are validated
		// when the ancestor command is processed in the outer loop.
		c.LocalFlags().VisitAll(func(f *pflag.Flag) {
			if f.Hidden || f.Usage == "" {
				return
			}
			first := rune(f.Usage[0])
			if unicode.IsUpper(first) {
				t.Errorf("command %q flag --%s: Usage starts with uppercase %q (must be lowercase): %q",
					c.CommandPath(), f.Name, string(first), f.Usage)
			}
		})
	}
}

func TestExamplePresent(t *testing.T) {
	for _, c := range allCommands(rootCmd) {
		if !c.Runnable() {
			continue
		}
		if c.Example == "" {
			t.Errorf("runnable command %q has no Example field", c.CommandPath())
		}
	}
}
