package git

// RemoteExists reports whether a remote with the given name is configured.
func RemoteExists(r Runner, name string) bool {
	_, err := r.Run("remote", "get-url", name)
	return err == nil
}

// AddRemote adds a new remote with the given name and URL.
func AddRemote(r Runner, name, url string) error {
	_, err := r.Run("remote", "add", name, url)
	return err
}
