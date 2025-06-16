// Command kenneyspritesheet turns takes a Kenney sprite sheet XML descriptor
// and outputs Go code for sprite definitions.
package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"strings"
	"text/template"
)

var (
	genFileName  = flag.String("gen-file-name", "", "How to name the generated file.")
	packageName  = flag.String("package-name", "", "The package name to use in the generated file.")
	atlasXMLFile = flag.String("atlas-xml", "", "The sprite sheet to process")
)

func main() {
	flag.Parse()

	if err := do(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func do() error {
	xmlBytes, err := os.ReadFile(*atlasXMLFile)
	if err != nil {
		return fmt.Errorf("while reading texture atlas XML: %w", err)
	}

	xmlAtlas := xmlTextureAtlas{}
	if err := xml.Unmarshal(xmlBytes, &xmlAtlas); err != nil {
		return fmt.Errorf("while unmarshaling texture atlas XML: %w", err)
	}

	for i := range xmlAtlas.SubTextures {
		name := xmlAtlas.SubTextures[i].Name
		name = strings.TrimSuffix(name, ".png")
		name = strings.ToTitle(name)
		xmlAtlas.SubTextures[i].CodeName = name
	}

	tmpl, err := template.New("test").Parse(fileTmpl)
	if err != nil {
		return fmt.Errorf("while parsing template: %w", err)
	}

	input := templateInput{
		PackageName: *packageName,
		Atlas:       xmlAtlas,
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
	"shellworld/lib/resource/spritesheet"

	"github.com/hajimehoshi/ebiten/v2"
)

{{range .Atlas.SubTextures}}
var {{.CodeName}} = &spritesheet.AtlasEntry{
	X:      {{.X}},
	Y:      {{.Y}},
	Width:  {{.Width}},
	Height: {{.Height}},
	CenterX: {{.CenterX}},
	CenterY: {{.CenterY}},
}
{{end}}

//go:embed {{.Atlas.ImagePath}}
var texturePNG []byte

var Sheet *spritesheet.T
func Load() error {
	img, err := png.Decode(bytes.NewReader(texturePNG))
	if err != nil {
		return fmt.Errorf("while decoding sprite PNG: %w", err)
	}

	Sheet = &spritesheet.T{
		Image: ebiten.NewImageFromImage(img),
	}

	return nil
}
`

type templateInput struct {
	PackageName string
	Atlas       xmlTextureAtlas
}

type xmlTextureAtlas struct {
	ImagePath   string          `xml:"imagePath,attr"`
	SubTextures []xmlSubTexture `xml:"SubTexture"`
}

type xmlSubTexture struct {
	CodeName string
	Name     string  `xml:"name,attr"`
	X        int     `xml:"x,attr"`
	Y        int     `xml:"y,attr"`
	Width    int     `xml:"width,attr"`
	Height   int     `xml:"height,attr"`
	CenterX  float32 `xml:"centerx,attr"`
	CenterY  float32 `xml:"centery,attr"`
}
