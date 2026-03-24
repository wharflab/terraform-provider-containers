package buildcontext

// ContextType represents the source type for a build context.
type ContextType string

const (
	ContextTypeLocal   ContextType = "local_directory"
	ContextTypeGit     ContextType = "git"
	ContextTypeS3      ContextType = "s3"
	ContextTypeTarball ContextType = "tarball"
)

// ValidContextTypes returns all valid context type values.
func ValidContextTypes() []string {
	return []string{
		string(ContextTypeLocal),
		string(ContextTypeGit),
		string(ContextTypeS3),
		string(ContextTypeTarball),
	}
}

// PreparedContext holds the result of context preparation, ready for upload.
type PreparedContext struct {
	Hash        string
	ArchivePath string
	Size        int64
}
