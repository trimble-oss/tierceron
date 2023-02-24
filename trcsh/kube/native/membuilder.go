package native

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dsnet/golib/memfile"
	eUtils "github.com/trimble-oss/tierceron/utils"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"k8s.io/cli-runtime/pkg/resource"
)

type MemBuilder struct {
	resource.Builder
	paths  []resource.Visitor
	errs   []error
	config *eUtils.DriverConfig
}

func (b *MemBuilder) Path(recursive bool, paths ...string) *MemBuilder {
	for _, p := range paths {
		_, err := os.Stat(p)
		if os.IsNotExist(err) {
			b.errs = append(b.errs, fmt.Errorf("the path %q does not exist", p))
			continue
		}
		if err != nil {
			b.errs = append(b.errs, fmt.Errorf("the path %q cannot be accessed: %v", p, err))
			continue
		}

		visitors, err := b.ExpandPathsToMemFileVisitors(p, recursive, resource.FileExtensions)
		if err != nil {
			b.errs = append(b.errs, fmt.Errorf("error reading %q: %v", p, err))
		}

		b.paths = append(b.paths, visitors...)
	}
	if len(b.paths) == 0 && len(b.errs) == 0 {
		b.errs = append(b.errs, fmt.Errorf("error reading %v: recognized file extensions are %v", paths, resource.FileExtensions))
	}
	return b
}

func (b *MemBuilder) MemFilenameParam(enforceNamespace bool, filenameOptions *resource.FilenameOptions) *MemBuilder {
	if errs := filenameOptions.validate(); len(errs) > 0 {
		b.errs = append(b.errs, errs...)
		return b
	}
	recursive := filenameOptions.Recursive
	paths := filenameOptions.Filenames
	for _, s := range paths {
		b.Path(recursive, s)
	}

	if enforceNamespace {
		b.RequireNamespace()
	}

	return b
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

// FileVisitor is wrapping around a StreamVisitor, to handle open/close files
type MemFileVisitor struct {
	MemFile *memfile.File
	*resource.StreamVisitor
}

// Visit in a FileVisitor is just taking care of opening/closing files
func (v *MemFileVisitor) Visit(fn resource.VisitorFunc) error {
	// TODO: Consider adding a flag to force to UTF16, apparently some
	// Windows tools don't write the BOM
	utf16bom := unicode.BOMOverride(unicode.UTF8.NewDecoder())
	v.StreamVisitor.Reader = transform.NewReader(v.MemFile, utf16bom)

	return v.StreamVisitor.Visit(fn)
}

func (b *MemBuilder) ExpandPathsToMemFileVisitors(paths string, recursive bool, extensions []string) ([]resource.Visitor, error) {
	var visitors []resource.Visitor
	err := filepath.Walk(paths, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if fi.IsDir() {
			if path != paths && !recursive {
				return filepath.SkipDir
			}
			return nil
		}
		// Don't check extension if the filepath was passed explicitly
		if path != paths && ignoreFile(path, extensions) {
			return nil
		}

		visitor := &MemFileVisitor{
			MemFile:       b.config.MemCache[path],
			StreamVisitor: resource.NewStreamVisitor(nil, b.Mapper(), path, b.schema),
		}

		visitors = append(visitors, visitor)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return visitors, nil
}
