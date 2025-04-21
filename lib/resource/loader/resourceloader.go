package loader

import (
	"fmt"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"shellworld/lib/resource/imagestamp"
	"shellworld/lib/vmath/vec2"
	"strings"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"go.starlark.net/starlark"
)

type ResourceLoader struct {
	assetDir string

	imageStampLock sync.Mutex
	imageStamps    map[string]*imageStampResource
}

func New(assetDir string) (*ResourceLoader, error) {
	rl := &ResourceLoader{
		assetDir:    assetDir,
		imageStamps: map[string]*imageStampResource{},
	}

	if err := rl.walkAssetDir(); err != nil {
		return nil, fmt.Errorf("while walking asset dir: %w", err)
	}

	return rl, nil
}

func (l *ResourceLoader) walkAssetDir() error {
	err := filepath.WalkDir(l.assetDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.Name() == "RESOURCE" {
			if err := l.runResourceFile(path); err != nil {
				return fmt.Errorf("while running RESOURCE file: %w", err)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (l *ResourceLoader) runResourceFile(path string) error {
	scriptBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("while reading resource file: %w", err)
	}

	thread := &starlark.Thread{
		Name:  "resource_parser",
		Print: func(_ *starlark.Thread, msg string) { fmt.Println(msg) },
	}

	predefined := starlark.StringDict{
		"image_stamp": starlark.NewBuiltin("image_stamp", l.callImageStamp),
	}

	_, err = starlark.ExecFile(thread, path, scriptBytes, predefined)
	if err != nil {
		return fmt.Errorf("while executing resource file: %w", err)
	}

	return nil
}

// Implementation of the image_stamp function.
func (l *ResourceLoader) callImageStamp(t *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name string
	var png string
	var centerInPixels starlark.Tuple
	var facing starlark.Tuple
	var widthInMeters float64
	if err := starlark.UnpackArgs("image_stamp", args, kwargs, "name", &name, "png", &png, "center_in_pixels", &centerInPixels, "facing", &facing, "width_in_meters", &widthInMeters); err != nil {
		return nil, err
	}

	resourceFileName := t.CallFrame(1).Pos.Filename()
	resourceFilePath, err := filepath.Rel(l.assetDir, resourceFileName)
	if err != nil {
		return nil, fmt.Errorf("while getting relative path of resource file: %w", err)
	}
	resourceDir, _ := filepath.Split(resourceFilePath)
	resourceDir = filepath.ToSlash(resourceDir)
	resourceDir = strings.TrimSuffix(resourceDir, "/")
	resourceIdentifier := fmt.Sprintf("//%s:%s", resourceDir, name)

	pngIdentifier := filepath.Join(resourceDir, png)

	centerInPixelsVec, err := vec2FromStarlark(centerInPixels)
	if err != nil {
		return nil, fmt.Errorf("center_in_pixels: %w", err)
	}

	facingVec, err := vec2FromStarlark(facing)
	if err != nil {
		return nil, fmt.Errorf("facing: %w", err)
	}

	handle := &imageStampResource{
		pngPath:        pngIdentifier,
		centerInPixels: centerInPixelsVec,
		facing:         facingVec,
		widthInMeters:  widthInMeters,
	}

	l.imageStampLock.Lock()
	defer l.imageStampLock.Unlock()
	l.imageStamps[resourceIdentifier] = handle

	return starlark.None, nil
}

func (l *ResourceLoader) LoadImage(assetPath string) (*ebiten.Image, error) {
	fullPath := filepath.Join(l.assetDir, assetPath)

	f, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("while opening file: %w", err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("while decoding png: %w", err)
	}

	ebimg := ebiten.NewImageFromImage(img)

	return ebimg, nil
}

func (l *ResourceLoader) LoadImageStamp(resourceID string) (*imagestamp.ImageStamp, error) {
	l.imageStampLock.Lock()
	defer l.imageStampLock.Unlock()

	obj, ok := l.imageStamps[resourceID]
	if !ok {
		return nil, fmt.Errorf("resource not found: %q", resourceID)
	}

	pngPath := filepath.Join(l.assetDir, obj.pngPath)

	f, err := os.Open(pngPath)
	if err != nil {
		return nil, fmt.Errorf("while opening PNG file: %w", err)
	}

	img, err := png.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("while decoding PNG: %w", err)
	}

	ebimg := ebiten.NewImageFromImage(img)

	return imagestamp.New(ebimg, obj.centerInPixels, obj.widthInMeters), nil
}

type imageStampResource struct {
	pngPath        string
	centerInPixels vec2.T
	facing         vec2.T
	widthInMeters  float64
}

func vec2FromStarlark(t starlark.Tuple) (vec2.T, error) {
	if t.Len() != 2 {
		return vec2.T{}, fmt.Errorf("must have length 2")
	}

	x, ok := t.Index(0).(starlark.Float)
	if !ok {
		return vec2.T{}, fmt.Errorf("elem 0 must be a float")
	}

	y, ok := t.Index(1).(starlark.Float)
	if !ok {
		return vec2.T{}, fmt.Errorf("elem 1 must be a float")
	}

	return vec2.T{float64(x), float64(y)}, nil
}

func parseResourcePath(p string) (string, string, error) {
	if !strings.HasPrefix(p, "//") {
		return "", "", fmt.Errorf("resource path must start with //")
	}

	p = strings.TrimPrefix(p, "//")

	splitpoint := strings.LastIndex(p, ":")
	if splitpoint == -1 {
		return "", "", fmt.Errorf("resource path must be in form //a/b/c:resource_name")
	}

	dir := p[:splitpoint]
	name := p[splitpoint+1:]

	return dir, name, nil
}
