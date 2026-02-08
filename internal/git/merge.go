package git

// Merge runs `git merge [--no-ff] <branch>` inside the given directory.
func Merge(r Runner, dir, branch string, noFF bool) error {
	args := []string{"merge"}
	if noFF {
		args = append(args, "--no-ff")
	}
	args = append(args, branch)
	_, err := r.RunInDir(dir, args...)
	return err
}
