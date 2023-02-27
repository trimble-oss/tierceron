package resource

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dsnet/golib/memfile"
	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/cli-runtime/pkg/resource"
)

// MemoryFileVisitor is wrapping around a StreamVisitor, to handle open/close files
type MemoryFileVisitor struct {
	MemFile *memfile.File
	*StreamVisitor
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

func (b *Builder) ExpandPathsToFileVisitors(paths string, recursive bool, extensions []string) ([]resource.Visitor, error) {
	var visitors []resource.Visitor
	err := filepath.Walk(paths, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Don't check extension if the filepath was passed explicitly
		if path != paths && ignoreFile(path, extensions) {
			return nil
		}
		memPath := path
		var memFile *memfile.File
		memFileOk := false
		if memFile, memFileOk = b.MemCache[memPath]; !memFileOk {
			if memFile, memFileOk = b.MemCache["./"+path]; !memFileOk {
				return nil
			}
		}

		visitor := &MemoryFileVisitor{
			MemFile:       memFile,
			StreamVisitor: NewStreamVisitor(nil, b.mapper, path, b.schema),
		}

		visitors = append(visitors, visitor)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return visitors, nil
}

// StreamVisitor reads objects from an io.Reader and walks them. A stream visitor can only be
// visited once.
// TODO: depends on objects being in JSON format before being passed to decode - need to implement
// a stream decoder method on runtime.Codec to properly handle this.
type StreamVisitor struct {
	io.Reader
	*mapper

	Source string
	Schema resource.ContentValidator
}

// NewStreamVisitor is a helper function that is useful when we want to change the fields of the struct but keep calls the same.
func NewStreamVisitor(r io.Reader, mapper *mapper, source string, schema resource.ContentValidator) *StreamVisitor {
	return &StreamVisitor{
		Reader: r,
		mapper: mapper,
		Source: source,
		Schema: schema,
	}
}

// Visit implements Visitor over a stream. StreamVisitor is able to distinct multiple resources in one stream.
func (v *StreamVisitor) Visit(fn resource.VisitorFunc) error {
	d := yaml.NewYAMLOrJSONDecoder(v.Reader, 4096)
	for {
		ext := runtime.RawExtension{}
		if err := d.Decode(&ext); err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("error parsing %s: %v", v.Source, err)
		}
		// TODO: This needs to be able to handle object in other encodings and schemas.
		ext.Raw = bytes.TrimSpace(ext.Raw)
		if len(ext.Raw) == 0 || bytes.Equal(ext.Raw, []byte("null")) {
			continue
		}
		if err := resource.ValidateSchema(ext.Raw, v.Schema); err != nil {
			return fmt.Errorf("error validating %q: %v", v.Source, err)
		}
		info, err := v.infoForData(ext.Raw, v.Source)
		if err != nil {
			if fnErr := fn(info, err); fnErr != nil {
				return fnErr
			}
			continue
		}
		if err := fn(info, nil); err != nil {
			return err
		}
	}
}

// FlattenListVisitor flattens any objects that runtime.ExtractList recognizes as a list
// - has an "Items" public field that is a slice of runtime.Objects or objects satisfying
// that interface - into multiple Infos. Returns nil in the case of no errors.
// When an error is hit on sub items (for instance, if a List contains an object that does
// not have a registered client or resource), returns an aggregate error.
type FlattenListVisitor struct {
	visitor resource.Visitor
	typer   runtime.ObjectTyper
	mapper  *mapper
}

// NewFlattenListVisitor creates a visitor that will expand list style runtime.Objects
// into individual items and then visit them individually.
func NewFlattenListVisitor(v resource.Visitor, typer runtime.ObjectTyper, mapper *mapper) resource.Visitor {
	return FlattenListVisitor{v, typer, mapper}
}

func (v FlattenListVisitor) Visit(fn resource.VisitorFunc) error {
	return v.visitor.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		if info.Object == nil {
			return fn(info, nil)
		}
		if !meta.IsListType(info.Object) {
			return fn(info, nil)
		}

		items := []runtime.Object{}
		itemsToProcess := []runtime.Object{info.Object}

		for i := 0; i < len(itemsToProcess); i++ {
			currObj := itemsToProcess[i]
			if !meta.IsListType(currObj) {
				items = append(items, currObj)
				continue
			}

			currItems, err := meta.ExtractList(currObj)
			if err != nil {
				return err
			}
			if errs := runtime.DecodeList(currItems, v.mapper.decoder); len(errs) > 0 {
				return utilerrors.NewAggregate(errs)
			}
			itemsToProcess = append(itemsToProcess, currItems...)
		}

		// If we have a GroupVersionKind on the list, prioritize that when asking for info on the objects contained in the list
		var preferredGVKs []schema.GroupVersionKind
		if info.Mapping != nil && !info.Mapping.GroupVersionKind.Empty() {
			preferredGVKs = append(preferredGVKs, info.Mapping.GroupVersionKind)
		}
		errs := []error{}
		for i := range items {
			item, err := v.mapper.infoForObject(items[i], v.typer, preferredGVKs)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			if len(info.ResourceVersion) != 0 {
				item.ResourceVersion = info.ResourceVersion
			}
			if err := fn(item, nil); err != nil {
				errs = append(errs, err)
			}
		}
		return utilerrors.NewAggregate(errs)

	})
}
