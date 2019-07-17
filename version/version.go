package version

var (
	version = ""
	commit  = ""
	date    = ""
)

func Version() string {
	return version
}

func Commit() string {
	return commit
}

func Date() string {
	return date
}

func String() string {
	if version != "" {
		return version
	}
	if commit != "" {
		return commit
	}
	return "(unknown)"
}
