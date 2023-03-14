package path

import (
	"errors"
	"path/filepath"

	"github.com/go-git/go-billy/v5"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
)

// MemoryFileVisitor is wrapping around a StreamVisitor, to handle open/close files
type MemoryFileVisitor struct {
	MemFile billy.File
	*resource.StreamVisitor
}

// Visit in a FileVisitor is just taking care of opening/closing files
func (v *MemoryFileVisitor) Visit(fn resource.VisitorFunc) error {
	// TODO: Consider adding a flag to force to UTF16, apparently some
	// Windows tools don't write the BOM
	utf16bom := unicode.BOMOverride(unicode.UTF8.NewDecoder())
	v.StreamVisitor.Reader = transform.NewReader(v.MemFile, utf16bom)

	return v.StreamVisitor.Visit(fn)
}

func ignoreFile(path string, extensions []string) bool {
	if len(extensions) == 0 {
		return false
	}
	ext := filepath.Ext(path)
	for _, s := range extensions {
		if s == ext {
			return false
		}
	}
	return true
}

type MemPathVisitor struct {
	MemFs     billy.Filesystem // Where to send output.
	Iostreams genericclioptions.IOStreams
}

func (mpv *MemPathVisitor) FileVisitorForSTDIN(builder *resource.Builder) resource.Visitor {
	return &MemoryFileVisitor{
		MemFile:       mpv.Iostreams.In.(billy.File),
		StreamVisitor: builder.NewStreamVisitorHelper(resource.NewStreamVisitor, mpv.Iostreams.In),
	}
}

// ExpandPathsToFileVisitors will return a slice of FileVisitors that will handle files from the provided path.
// After FileVisitors open the files, they will pass an io.Reader to a StreamVisitor to do the reading. (stdin
// is also taken care of). Paths argument also accepts a single file, and will return a single visitor

// ExpandPathsToFileVisitors
func (mpv *MemPathVisitor) ExpandPathsToFileVisitors(mapper resource.InfoMapper, path string, recursive bool, extensions []string, schema resource.ContentValidator) ([]resource.Visitor, error) {
	var visitors []resource.Visitor

	// Don't check extension if the filepath was passed explicitly
	if ignoreFile(path, extensions) {
		return nil, errors.New("Unsupported extension")
	}
	memPath := path
	var memFile billy.File
	var memFileErr error

	if memFile, memFileErr = mpv.MemFs.Open(memPath); memFileErr != nil {
		return nil, errors.New("Unsupported file")
	}

	visitor := &MemoryFileVisitor{
		MemFile:       memFile,
		StreamVisitor: resource.NewStreamVisitor(nil, mapper, path, schema),
	}

	visitors = append(visitors, visitor)
	return visitors, nil
}
