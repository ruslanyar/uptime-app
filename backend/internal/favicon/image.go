package favicon

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/xml"
	"image"
	"image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"math/bits"
	"net/http"
	"strconv"
	"strings"

	_ "golang.org/x/image/webp"
)

const (
	maxImagePixels = 1_048_576
	maxImageSide   = 2048
	maxICOFrames   = 64
	maxGIFFrames   = 64
	maxWebPFrames  = 64
	maxXMLDepth    = 128
	maxXMLTokens   = 16_384
	maxNamespaces  = 64
)

// Decoders run synchronously: bounded images cannot leave decoding goroutines
// behind after cancellation. Check the budget on reads and again after decoding.
type imageReader struct {
	ctx context.Context
	*bytes.Reader
}

func (r imageReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}

func (r imageReader) ReadByte() (byte, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.ReadByte()
}

func imageType(ctx context.Context, body []byte) string {
	if ctx.Err() != nil || len(body) == 0 || len(body) > maxBody {
		return ""
	}
	kind := http.DetectContentType(body)
	switch kind {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		if validRaster(ctx, body, kind, 0, 0) {
			return kind
		}
	case "image/x-icon":
		if validICO(ctx, body) {
			return kind
		}
	default:
		if validSVG(ctx, body) {
			return "image/svg+xml"
		}
	}
	return ""
}

func validDimensions(width, height int) bool {
	return width > 0 && height > 0 && width <= maxImageSide && height <= maxImageSide &&
		width*height <= maxImagePixels
}

func validRaster(ctx context.Context, body []byte, kind string, width, height int) bool {
	reader := func() imageReader { return imageReader{ctx, bytes.NewReader(body)} }
	config, format, err := image.DecodeConfig(reader())
	expected := map[string]string{"image/png": "png", "image/jpeg": "jpeg", "image/gif": "gif", "image/webp": "webp"}[kind]
	if err != nil || format != expected || !validDimensions(config.Width, config.Height) || ctx.Err() != nil {
		return false
	}
	if width != 0 && (config.Width != width || config.Height != height) {
		return false
	}
	if format == "webp" {
		if !completeWebP(body) {
			return false
		}
		if len(body) >= 30 && string(body[12:16]) == "VP8X" && body[20]&2 != 0 {
			return validAnimatedWebP(ctx, body, config.Width, config.Height)
		}
	}
	if format == "gif" {
		// DecodeAll retains every frame. Preflight the frame count and total pixels
		// before allowing that allocation, then validate the complete animation.
		if !boundedGIF(ctx, body) {
			return false
		}
		_, err := gif.DecodeAll(reader())
		return err == nil && ctx.Err() == nil
	}
	decoded, _, err := image.Decode(reader())
	return err == nil && ctx.Err() == nil && validDimensions(decoded.Bounds().Dx(), decoded.Bounds().Dy())
}

func boundedGIF(ctx context.Context, data []byte) bool {
	if len(data) < 13 {
		return false
	}
	pos := 13
	if data[10]&0x80 != 0 {
		pos += 3 * (2 << (data[10] & 7))
	}
	frames, pixels := 0, 0
	for pos < len(data) && ctx.Err() == nil {
		block := data[pos]
		pos++
		switch block {
		case 0x3b:
			return frames > 0
		case 0x21:
			pos++ // Extension label precedes the data sub-blocks.
		case 0x2c:
			if pos+9 > len(data) {
				return false
			}
			width, height := int(binary.LittleEndian.Uint16(data[pos+4:])), int(binary.LittleEndian.Uint16(data[pos+6:]))
			frames++
			if !validDimensions(width, height) || frames > maxGIFFrames {
				return false
			}
			pixels += width * height
			if pixels > maxImagePixels {
				return false
			}
			packed := data[pos+8]
			pos += 9
			if packed&0x80 != 0 {
				pos += 3 * (2 << (packed & 7))
			}
			pos++ // LZW minimum code size precedes the data sub-blocks.
		default:
			return false
		}
		for {
			if pos >= len(data) {
				return false
			}
			size := int(data[pos])
			pos++
			if size == 0 {
				break
			}
			pos += size
		}
	}
	return false
}

func validICO(ctx context.Context, body []byte) bool {
	if len(body) < 6 || binary.LittleEndian.Uint16(body) != 0 || binary.LittleEndian.Uint16(body[2:]) != 1 {
		return false
	}
	count := int(binary.LittleEndian.Uint16(body[4:]))
	directoryEnd := 6 + count*16
	if count == 0 || count > maxICOFrames || directoryEnd > len(body) {
		return false
	}
	pixels := 0
	// Validate every representation: a browser may choose any ICO frame.
	for i := 0; i < count; i++ {
		if ctx.Err() != nil {
			return false
		}
		entry := body[6+i*16 : 6+(i+1)*16]
		width, height := int(entry[0]), int(entry[1])
		if width == 0 {
			width = 256
		}
		if height == 0 {
			height = 256
		}
		pixels += width * height
		offset, size := uint64(binary.LittleEndian.Uint32(entry[12:])), uint64(binary.LittleEndian.Uint32(entry[8:]))
		if entry[3] != 0 || binary.LittleEndian.Uint16(entry[4:]) > 1 || pixels > maxImagePixels ||
			offset < uint64(directoryEnd) || size == 0 || offset+size > uint64(len(body)) {
			return false
		}
		payload := body[offset : offset+size]
		if bytes.HasPrefix(payload, []byte("\x89PNG\r\n\x1a\n")) {
			if !validRaster(ctx, payload, "image/png", width, height) {
				return false
			}
		} else if !validIconDIB(ctx, payload, width, height, binary.LittleEndian.Uint16(entry[6:])) {
			return false
		}
	}
	return ctx.Err() == nil
}

// ICO DIBs are uncompressed pixel rows and a one-bit transparency mask, not BMP
// files. Validate both planes and palette indices without allocating a bitmap.
func validIconDIB(ctx context.Context, data []byte, width, height int, entryBits uint16) bool {
	if len(data) < 40 {
		return false
	}
	headerSize := int(binary.LittleEndian.Uint32(data))
	if (headerSize != 40 && headerSize != 108 && headerSize != 124) || headerSize > len(data) ||
		int32(binary.LittleEndian.Uint32(data[4:])) != int32(width) ||
		int32(binary.LittleEndian.Uint32(data[8:])) != int32(height*2) || binary.LittleEndian.Uint16(data[12:]) != 1 {
		return false
	}
	bpp := int(binary.LittleEndian.Uint16(data[14:]))
	if (bpp != 1 && bpp != 4 && bpp != 8 && bpp != 16 && bpp != 24 && bpp != 32) ||
		(entryBits != 0 && int(entryBits) != bpp) {
		return false
	}
	compression := binary.LittleEndian.Uint32(data[16:])
	pixelOffset := headerSize
	if compression == 3 && (bpp == 16 || bpp == 32) {
		maskOffset := 40
		if headerSize == 40 {
			pixelOffset += 12
		}
		if pixelOffset > len(data) {
			return false
		}
		var used uint32
		for i := 0; i < 3; i++ {
			mask := binary.LittleEndian.Uint32(data[maskOffset+i*4:])
			shifted := mask >> bits.TrailingZeros32(mask)
			if mask == 0 || mask&used != 0 || (bpp == 16 && mask>>16 != 0) || shifted&(shifted+1) != 0 {
				return false
			}
			used |= mask
		}
	} else if compression != 0 {
		return false
	}
	colorCount := binary.LittleEndian.Uint32(data[32:])
	if colorCount > 256 {
		return false
	}
	colors := int(colorCount)
	if bpp <= 8 && colors == 0 {
		colors = 1 << bpp
	}
	if bpp <= 8 && colors > 1<<bpp {
		return false
	}
	pixelOffset += colors * 4
	// Each color/mask scanline is padded to a 32-bit boundary. ICO stores the
	// two planes consecutively, hence the doubled height in the DIB header.
	rowSize := ((width*bpp + 31) / 32) * 4
	maskRowSize := ((width + 31) / 32) * 4
	if pixelOffset+(rowSize+maskRowSize)*height > len(data) {
		return false
	}
	for y := 0; y < height; y++ {
		if ctx.Err() != nil {
			return false
		}
		if bpp <= 8 {
			row := data[pixelOffset+y*rowSize:]
			for x := 0; x < width; x++ {
				index := (int(row[x*bpp/8]) >> (8 - bpp - x*bpp%8)) & ((1 << bpp) - 1)
				if index >= colors {
					return false
				}
			}
		}
	}
	return true
}

func validSVG(ctx context.Context, body []byte) bool {
	decoder := xml.NewDecoder(imageReader{ctx, bytes.NewReader(body)})
	depth, tokens := 0, 0
	rootSeen := false
	bomSeen := false
	namespaces := []map[string]string{{"": "", "xml": "http://www.w3.org/XML/1998/namespace"}}
	var elements []xml.Name
	for {
		// Token accepts undefined namespace prefixes. RawToken lets us reject
		// those explicitly and check matching element names ourselves.
		token, err := decoder.RawToken()
		if err == io.EOF {
			return rootSeen && depth == 0 && ctx.Err() == nil
		}
		tokens++
		if err != nil || tokens > maxXMLTokens || ctx.Err() != nil {
			return false
		}
		switch element := token.(type) {
		case xml.StartElement:
			bindings := make(map[string]string, len(namespaces[depth]))
			for prefix, namespace := range namespaces[depth] {
				bindings[prefix] = namespace
			}
			for _, attr := range element.Attr {
				if attr.Name.Space == "xmlns" || attr.Name.Space == "" && attr.Name.Local == "xmlns" {
					prefix := attr.Name.Local
					if attr.Name.Space == "" {
						prefix = ""
					}
					if prefix == "xmlns" || prefix != "" && attr.Value == "" ||
						attr.Value == "http://www.w3.org/2000/xmlns/" ||
						(prefix == "xml") != (attr.Value == "http://www.w3.org/XML/1998/namespace") {
						return false
					}
					bindings[prefix] = attr.Value
				}
			}
			if len(bindings) > maxNamespaces {
				return false
			}
			attributes := make(map[xml.Name]bool, len(element.Attr))
			for _, attr := range element.Attr {
				name := attr.Name
				if name.Space != "" && name.Space != "xmlns" {
					var ok bool
					name.Space, ok = bindings[name.Space]
					if !ok {
						return false
					}
				}
				if attributes[name] {
					return false
				}
				attributes[name] = true
			}
			namespace, ok := bindings[element.Name.Space]
			if !ok || element.Name.Space == "xmlns" {
				return false
			}
			if depth == 0 {
				if rootSeen || element.Name.Local != "svg" || namespace != "http://www.w3.org/2000/svg" || !validSVGSize(element.Attr) {
					return false
				}
				rootSeen = true
			}
			depth++
			if depth > maxXMLDepth {
				return false
			}
			namespaces = append(namespaces, bindings)
			elements = append(elements, element.Name)
		case xml.EndElement:
			if depth == 0 || elements[depth-1] != element.Name {
				return false
			}
			depth--
			namespaces = namespaces[:depth+1]
			elements = elements[:depth]
		case xml.CharData:
			text := string(element)
			if tokens == 1 {
				bomSeen = text == "\ufeff"
				text = strings.TrimPrefix(text, "\ufeff")
			}
			if depth == 0 && strings.TrimSpace(text) != "" {
				return false
			}
		case xml.Directive:
			// DTDs and custom entities are unnecessary for icons; no external
			// definitions should influence validation or browser decoding.
			return false
		case xml.ProcInst:
			if strings.EqualFold(element.Target, "xml") && (element.Target != "xml" || tokens != 1 && !(tokens == 2 && bomSeen)) {
				return false
			}
		}
	}
}

func validSVGSize(attrs []xml.Attr) bool {
	width, height := 300.0, 150.0
	for _, attr := range attrs {
		if attr.Name.Space != "" {
			continue
		}
		switch attr.Name.Local {
		case "width":
			width = svgLength(attr.Value, 300)
		case "height":
			height = svgLength(attr.Value, 150)
		}
	}
	return width > 0 && height > 0 && width <= maxImageSide && height <= maxImageSide && width*height <= maxImagePixels
}

func svgLength(value string, reference float64) float64 {
	value = strings.TrimSpace(value)
	scale := 1.0
	for _, unit := range []struct {
		suffix string
		scale  float64
	}{{"px", 1}, {"pt", 96.0 / 72}, {"pc", 16}, {"mm", 96.0 / 25.4}, {"cm", 96.0 / 2.54}, {"in", 96}, {"em", 16}, {"ex", 8}, {"%", reference / 100}} {
		if strings.HasSuffix(value, unit.suffix) {
			value = strings.TrimSuffix(value, unit.suffix)
			scale = unit.scale
			break
		}
	}
	length, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(length) || math.IsInf(length, 0) {
		return 0
	}
	return length * scale
}
