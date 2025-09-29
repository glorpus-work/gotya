package platform

// Package platform provides constants and utilities for handling platform-specific
// information such as operating systems and architectures.

const (
	// OSWindows represents the Windows operating system.
	OSWindows = "windows"
	// OSLinux represents the Linux operating system.
	OSLinux = "linux"
	// OSDarwin represents the macOS operating system.
	OSDarwin = "darwin"
	// OSFreeBSD represents the FreeBSD operating system.
	OSFreeBSD = "freebsd"
	// OSOpenBSD represents the OpenBSD operating system.
	OSOpenBSD = "openbsd"
	// OSNetBSD represents the NetBSD operating system.
	OSNetBSD = "netbsd"
	// AnyOS represents any possible OS
	AnyOS = "any"

	// ArchAMD64 represents the AMD64 (x86_64) architecture.
	ArchAMD64 = "amd64"
	// Arch386 represents the 32-bit x86 architecture.
	Arch386 = "386"
	// ArchARM represents the ARM architecture (32-bit).
	ArchARM = "arm"
	// ArchARM64 represents the ARM64 (AArch64) architecture.
	ArchARM64 = "arm64"
	// AnyArch represents any possible architecture
	AnyArch = "any"
)

// ValidOS returns a list of valid OS values.
func ValidOS() []string {
	return []string{
		OSWindows,
		OSLinux,
		OSDarwin,
		OSFreeBSD,
		OSOpenBSD,
		OSNetBSD,
	}
}

// ValidArch returns a list of valid architecture values.
func ValidArch() []string {
	return []string{
		ArchAMD64,
		Arch386,
		ArchARM,
		ArchARM64,
	}
}
