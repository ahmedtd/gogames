// Command spritexml2pb turns takes a Kenney sprite sheet XML descriptor
// and outputs a spritesheet textproto
package main

import (
	"bytes"
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"
)

var (
	outFileName  = flag.String("out", "", "How to name the generated file.")
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
		Atlas: xmlAtlas,
	}

	rawSource := &bytes.Buffer{}
	if err := tmpl.Execute(rawSource, input); err != nil {
		return fmt.Errorf("while executing template: %w", err)
	}

	if err := os.WriteFile(*outFileName, rawSource.Bytes(), 0o644); err != nil {
		return fmt.Errorf("while writing output file: %w", err)
	}

	return nil
}

var fileTmpl = `image_path: "{{.Atlas.ImagePath}}"
{{range .Atlas.SubTextures}}
sprites {
  name: "{{.Name}}"
  x: {{.X}}
  y: {{.Y}}
  width: {{.Width}}
  height: {{.Height}}
  center_x: {{.CenterX}}
  center_y: {{.CenterY}}
}
{{end}}
`

type templateInput struct {
	Atlas xmlTextureAtlas
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
