package domain

// FileEntry is one item of a directory listing.
type FileEntry struct {
	Name  string
	IsDir bool
}
