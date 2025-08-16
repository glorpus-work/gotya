package platform

// GetValidOS returns a list of valid OS values
func GetValidOS() []string {
	return []string{
		"windows",
		"linux",
		"darwin",
		"freebsd",
		"openbsd",
		"netbsd",
		"dragonfly",
		"solaris",
	}
}

// GetValidArch returns a list of valid architecture values
func GetValidArch() []string {
	return []string{
		"386",
		"amd64",
		"arm",
		"arm64",
		"ppc64",
		"ppc64le",
		"mips",
		"mipsle",
		"mips64",
		"mips64le",
	}
}
