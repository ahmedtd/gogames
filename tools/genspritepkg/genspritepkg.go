// Command genspritepkg turns takes a textproto SpriteSheet descriptor and
// outputs generated go code for using the sprite package.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/ahmedtd/gogames/lib/resource/spritesheet/spritesheetpb"
	"google.golang.org/protobuf/encoding/prototext"
)

var (
	genFileName          = flag.String("gen-file-name", "", "How to name the generated file.")
	packageName          = flag.String("package-name", "", "The package name to use in the generated file.")
	spriteSheetProtoFile = flag.String("sprite-sheet-proto", "", "The sprite sheet to process")
)

func main() {
	flag.Parse()

	if err := do(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func do() error {
	textprotoBytes, err := os.ReadFile(*spriteSheetProtoFile)
	if err != nil {
		return fmt.Errorf("while reading texture atlas XML: %w", err)
	}

	sheet := &spritesheetpb.SpriteSheet{}
	if err := prototext.Unmarshal(textprotoBytes, sheet); err != nil {
		return fmt.Errorf("while unmarshaling spritesheet from textproto: %w", err)
	}

	tmpl, err := template.New("test").Parse(fileTmpl)
	if err != nil {
		return fmt.Errorf("while parsing template: %w", err)
	}

	input := templateInput{
		PackageName:             *packageName,
		RelativePathToTextProto: filepath.Base(*spriteSheetProtoFile),
		SpriteSheet: templateSpriteSheet{
			ImagePath: sheet.GetImagePath(),
		},
	}

	for i, sprite := range sheet.GetSprites() {
		input.SpriteSheet.Sprites = append(input.SpriteSheet.Sprites, templateSprite{
			Index:    i,
			CodeName: strings.ToTitle(sprite.GetName()),
			Name:     sprite.GetName(),
			X:        int(sprite.GetX()),
			Y:        int(sprite.GetY()),
			Width:    int(sprite.GetWidth()),
			Height:   int(sprite.GetHeight()),
			CenterX:  sprite.GetCenterX(),
			CenterY:  sprite.GetCenterY(),
		})
	}

	rawSource := &bytes.Buffer{}
	if err := tmpl.Execute(rawSource, input); err != nil {
		return fmt.Errorf("while executing template: %w", err)
	}

	formattedSource, err := format.Source(rawSource.Bytes())
	if err != nil {
		log.Printf("raw source:\n%s", rawSource)
		return fmt.Errorf("while formatting raw source: %w", err)
	}

	if err := os.WriteFile(*genFileName, formattedSource, 0o644); err != nil {
		return fmt.Errorf("while writing output file: %w", err)
	}

	return nil
}

var fileTmpl = `package {{.PackageName}}

import (
	"bytes"
	_ "embed"
	"fmt"
	"image/png"

	"github.com/ahmedtd/gogames/lib/resource/spritesheet/spritesheetpb"
	"github.com/ahmedtd/gogames/lib/resource/spritesheet"
	"github.com/hajimehoshi/ebiten/v2"
	"google.golang.org/protobuf/encoding/prototext"
)

//go:embed {{.RelativePathToTextProto}}
var textprotoBytes []byte

//go:embed {{.SpriteSheet.ImagePath}}
var texturePNG []byte

{{range .SpriteSheet.Sprites}}
const {{.CodeName}} int = {{.Index}}
{{end}}

var Sheet *spritesheet.T
func Load() error {
	sheetProto := &spritesheetpb.SpriteSheet{}
	if err := prototext.Unmarshal(textprotoBytes, sheetProto); err != nil {
		return fmt.Errorf("while unmarshaling spritesheet from textproto: %w", err)
	}

	img, err := png.Decode(bytes.NewReader(texturePNG))
	if err != nil {
		return fmt.Errorf("while decoding sprite PNG: %w", err)
	}

	Sheet = spritesheet.New(sheetProto, ebiten.NewImageFromImage(img))

	return nil
}
`

type templateInput struct {
	PackageName             string
	RelativePathToTextProto string
	SpriteSheet             templateSpriteSheet
}

type templateSpriteSheet struct {
	ImagePath string
	Sprites   []templateSprite
}

type templateSprite struct {
	Index    int
	CodeName string
	Name     string
	X        int
	Y        int
	Width    int
	Height   int
	CenterX  float64
	CenterY  float64
}
