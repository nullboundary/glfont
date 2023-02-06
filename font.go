package glfont

import (
	"bytes"
	"fmt"
	"os"

	"github.com/go-gl/gl/all-core/gl"
)

// Direction represents the direction in which strings should be rendered.
type Direction uint8

// Known directions.
const (
	LeftToRight Direction = iota // E.g.: Latin
	RightToLeft                  // E.g.: Arabic
	TopToBottom                  // E.g.: Chinese
)

type color struct {
	r float32
	g float32
	b float32
	a float32
}

// Use default preapration for exported functions like `LoadFont` and `LoadFontFromBytes`
func configureDefaults(windowWidth int, windowHeight int) uint32 {
	// Configure the default font vertex and fragment shaders
	program, err := newProgram(vertexFontShader, fragmentFontShader)
	if err != nil {
		panic(err)
	}

	// Activate corresponding render state
	gl.UseProgram(program)

	// set screen resolution
	resUniform := gl.GetUniformLocation(program, gl.Str("resolution\x00"))
	gl.Uniform2f(resUniform, float32(windowWidth), float32(windowHeight))

	return program
}

// LoadFontBytes loads the specified font bytes at the given scale.
func LoadFontBytes(buf []byte, scale int32, windowWidth int, windowHeight int) (*Font, error) {
	program := configureDefaults(windowWidth, windowHeight)

	fd := bytes.NewReader(buf)
	return LoadTrueTypeFont(program, fd, scale, 32, 127, LeftToRight)
}

// LoadFont loads the specified font at the given scale.
func LoadFont(file string, scale int32, windowWidth int, windowHeight int) (*Font, error) {
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	program := configureDefaults(windowWidth, windowHeight)

	return LoadTrueTypeFont(program, fd, scale, 32, 127, LeftToRight)
}

// SetColor allows you to set the text color to be used when you draw the text
func (f *Font) SetColor(red float32, green float32, blue float32, alpha float32) {
	f.color.r = red
	f.color.g = green
	f.color.b = blue
	f.color.a = alpha
}

// UpdateResolution used to recalibrate fonts for new window size
func (f *Font) UpdateResolution(windowWidth int, windowHeight int) {
	gl.UseProgram(f.program)
	resUniform := gl.GetUniformLocation(f.program, gl.Str("resolution\x00"))
	gl.Uniform2f(resUniform, float32(windowWidth), float32(windowHeight))
	gl.UseProgram(0)
}

// Printf draws a string to the screen, takes a list of arguments like printf
func (f *Font) Printf(x, y float32, scale float32, fs string, argv ...interface{}) error {

	indices := []rune(fmt.Sprintf(fs, argv...))

	if len(indices) == 0 {
		return nil
	}

	// setup blending mode
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)

	// Activate corresponding render state
	gl.UseProgram(f.program)
	// set text color
	gl.Uniform4f(gl.GetUniformLocation(f.program, gl.Str("textColor\x00")), f.color.r, f.color.g, f.color.b, f.color.a)
	// set screen resolution
	// resUniform := gl.GetUniformLocation(f.program, gl.Str("resolution\x00"))
	// gl.Uniform2f(resUniform, float32(2560), float32(1440))

	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindVertexArray(f.vao)

	// Iterate through all characters in string
	for i := range indices {

		// get rune
		runeIndex := indices[i]

		// find rune in fontChar list
		ch, ok := f.fontChar[runeIndex]

		// load missing runes in batches of 32
		if !ok {
			low := runeIndex - (runeIndex % 32)
			f.GenerateGlyphs(low, low+31)
			ch, ok = f.fontChar[runeIndex]
		}

		// skip runes that are not in font chacter range
		if !ok {
			fmt.Printf("%c %d\n", runeIndex, runeIndex)
			continue
		}

		// calculate position and size for current rune
		xpos := x + float32(ch.bearingH)*scale
		ypos := y - float32(ch.height-ch.bearingV)*scale
		w := float32(ch.width) * scale
		h := float32(ch.height) * scale
		vertices := []float32{
			xpos + w, ypos, 1.0, 0.0,
			xpos, ypos, 0.0, 0.0,
			xpos, ypos + h, 0.0, 1.0,

			xpos, ypos + h, 0.0, 1.0,
			xpos + w, ypos + h, 1.0, 1.0,
			xpos + w, ypos, 1.0, 0.0,
		}

		// Render glyph texture over quad
		gl.BindTexture(gl.TEXTURE_2D, ch.textureID)
		// Update content of VBO memory
		gl.BindBuffer(gl.ARRAY_BUFFER, f.vbo)

		// BufferSubData(target Enum, offset int, data []byte)
		gl.BufferSubData(gl.ARRAY_BUFFER, 0, len(vertices)*4, gl.Ptr(vertices)) // Be sure to use glBufferSubData and not glBufferData
		// Render quad
		gl.DrawArrays(gl.TRIANGLES, 0, 16)

		gl.BindBuffer(gl.ARRAY_BUFFER, 0)
		// Now advance cursors for next glyph (note that advance is number of 1/64 pixels)
		x += float32((ch.advance >> 6)) * scale // Bitshift by 6 to get value in pixels (2^6 = 64 (divide amount of 1/64th pixels by 64 to get amount of pixels))

	}

	// clear opengl textures and programs
	gl.BindVertexArray(0)
	gl.BindTexture(gl.TEXTURE_2D, 0)
	gl.UseProgram(0)
	gl.Disable(gl.BLEND)

	return nil
}

// Width returns the width of a piece of text in pixels
func (f *Font) Width(scale float32, fs string, argv ...interface{}) float32 {

	var width float32

	indices := []rune(fmt.Sprintf(fs, argv...))

	if len(indices) == 0 {
		return 0
	}

	// Iterate through all characters in string
	for i := range indices {

		// get rune
		runeIndex := indices[i]

		// find rune in fontChar list
		ch, ok := f.fontChar[runeIndex]

		// load missing runes in batches of 32
		if !ok {
			low := runeIndex & rune(32-1)
			f.GenerateGlyphs(low, low+31)
			ch, ok = f.fontChar[runeIndex]
		}

		// skip runes that are not in font chacter range
		if !ok {
			fmt.Printf("%c %d\n", runeIndex, runeIndex)
			continue
		}

		// Now advance cursors for next glyph (note that advance is number of 1/64 pixels)
		width += float32((ch.advance >> 6)) * scale // Bitshift by 6 to get value in pixels (2^6 = 64 (divide amount of 1/64th pixels by 64 to get amount of pixels))

	}

	return width
}
