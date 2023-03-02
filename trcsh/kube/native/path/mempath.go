package path

import (
	"errors"
	"path/filepath"

	"github.com/dsnet/golib/memfile"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"k8s.io/cli-runtime/pkg/resource"
)

// MemoryFileVisitor is wrapping around a StreamVisitor, to handle open/close files
type MemoryFileVisitor struct {
	MemFile *memfile.File
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
	MemCache map[string]*memfile.File // Where to send output.
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
	var memFile *memfile.File
	memFileOk := false
	if memFile, memFileOk = mpv.MemCache[memPath]; !memFileOk {
		memPath = "./" + memPath
		if memFile, memFileOk = mpv.MemCache[memPath]; !memFileOk {
			return nil, errors.New("Unsupported file")
		}
	}

	visitor := &MemoryFileVisitor{
		MemFile:       memFile,
		StreamVisitor: resource.NewStreamVisitor(nil, mapper, path, schema),
	}

	visitors = append(visitors, visitor)
	return visitors, nil
}
