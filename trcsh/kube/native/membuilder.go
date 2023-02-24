package native

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dsnet/golib/memfile"
	"github.com/jonboulle/clockwork"
	eUtils "github.com/trimble-oss/tierceron/utils"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/kubectl/pkg/cmd/apply"
	"k8s.io/kubectl/pkg/util/openapi"
)

const (
	maxPatchRetry                        = 5
	warningNoLastAppliedConfigAnnotation = "Warning: resource %[1]s is missing the %[2]s annotation which is required by %[3]s apply. %[3]s apply should only be used on resources created declaratively by either %[3]s create --save-config or %[3]s apply. The missing annotation will be patched automatically.\n"
)

type MemBuilder struct {
	resource.Builder
	schema resource.ContentValidator
	paths  []resource.Visitor
	errs   []error
	config *eUtils.DriverConfig
}

func newPatcher(o *apply.ApplyOptions, info *resource.Info, helper *resource.Helper) (*apply.Patcher, error) {
	var openapiSchema openapi.Resources
	if o.OpenAPIPatch {
		openapiSchema = o.OpenAPISchema
	}

	return &apply.Patcher{
		Mapping:           info.Mapping,
		Helper:            helper,
		Overwrite:         o.Overwrite,
		BackOff:           clockwork.NewRealClock(),
		Force:             o.DeleteOptions.ForceDeletion,
		CascadingStrategy: o.DeleteOptions.CascadingStrategy,
		Timeout:           o.DeleteOptions.Timeout,
		GracePeriod:       o.DeleteOptions.GracePeriod,
		OpenapiSchema:     openapiSchema,
		Retries:           maxPatchRetry,
	}, nil
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

func (b *MemBuilder) MemSchema(schema resource.ContentValidator) *MemBuilder {
	b.schema = schema
	b.Schema(schema)
	return b
}

func (b *MemBuilder) MemFilenameParam(enforceNamespace bool, filenameOptions *resource.FilenameOptions) *MemBuilder {
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
