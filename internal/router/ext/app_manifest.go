package ext

// ApplicationExtensionManifestEntry declares one generated required application extension.
type ApplicationExtensionManifestEntry struct {
	Name string
}

// DeclaredApplicationExtensions defines the ordered required application extension declarations.
var DeclaredApplicationExtensions = []ApplicationExtensionManifestEntry{}
