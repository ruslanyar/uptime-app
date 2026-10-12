package favicon

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func rasterFixtures(t *testing.T) map[string][]byte {
	t.Helper()
	picture := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	picture.Set(0, 0, color.NRGBA{R: 255, A: 255})
	fixtures := make(map[string][]byte)
	for kind, encode := range map[string]func(io.Writer) error{
		"image/png":  func(w io.Writer) error { return png.Encode(w, picture) },
		"image/jpeg": func(w io.Writer) error { return jpeg.Encode(w, picture, nil) },
		"image/gif":  func(w io.Writer) error { return gif.Encode(w, picture, nil) },
	} {
		var buffer bytes.Buffer
		if err := encode(&buffer); err != nil {
			t.Fatal(err)
		}
		fixtures[kind] = buffer.Bytes()
	}
	// A complete 1x1 VP8 image; WebP has no encoder in x/image.
	webp, err := base64.StdEncoding.DecodeString("UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA")
	if err != nil {
		t.Fatal(err)
	}
	fixtures["image/webp"] = webp
	return fixtures
}

func pngICO(payload []byte, width, height byte) []byte {
	data := make([]byte, 22)
	binary.LittleEndian.PutUint16(data[2:], 1)
	binary.LittleEndian.PutUint16(data[4:], 1)
	data[6], data[7] = width, height
	binary.LittleEndian.PutUint16(data[10:], 1)
	binary.LittleEndian.PutUint16(data[12:], 32)
	binary.LittleEndian.PutUint32(data[14:], uint32(len(payload)))
	binary.LittleEndian.PutUint32(data[18:], 22)
	return append(data, payload...)
}

func dibICO(bpp int, bitfields bool) []byte {
	colors := 0
	if bpp <= 8 {
		colors = 1 << bpp
	}
	maskBytes := 0
	if bitfields {
		maskBytes = 12
	}
	// Two 2-pixel rows, each DWORD-aligned, followed by two mask rows.
	rowSize := ((2*bpp + 31) / 32) * 4
	payload := make([]byte, 40+maskBytes+colors*4+2*rowSize+8)
	binary.LittleEndian.PutUint32(payload, 40)
	binary.LittleEndian.PutUint32(payload[4:], 2)
	binary.LittleEndian.PutUint32(payload[8:], 4)
	binary.LittleEndian.PutUint16(payload[12:], 1)
	binary.LittleEndian.PutUint16(payload[14:], uint16(bpp))
	if bitfields {
		binary.LittleEndian.PutUint32(payload[16:], 3)
		masks := []uint32{0xff0000, 0xff00, 0xff}
		if bpp == 16 {
			masks = []uint32{0x7c00, 0x03e0, 0x001f}
		}
		for i, mask := range masks {
			binary.LittleEndian.PutUint32(payload[40+i*4:], mask)
		}
	}
	if colors != 0 {
		paletteStart := 40 + maskBytes
		payload[paletteStart+4] = 255
		payload[paletteStart+colors*4] = 1 << (8 - bpp)
	} else {
		payload[40+maskBytes] = 255
	}
	data := pngICO(payload, 2, 2)
	binary.LittleEndian.PutUint16(data[12:], uint16(bpp))
	return data
}

func repeatedICO(data []byte, count int) []byte {
	directory := make([]byte, 6+16*count)
	copy(directory, data[:6])
	binary.LittleEndian.PutUint16(directory[4:], uint16(count))
	for i := 0; i < count; i++ {
		entry := directory[6+i*16 : 6+(i+1)*16]
		copy(entry, data[6:22])
		binary.LittleEndian.PutUint32(entry[12:], uint32(len(directory)))
	}
	return append(directory, data[22:]...)
}

func TestImageValidation(t *testing.T) {
	fixtures := rasterFixtures(t)
	fixtures["image/svg+xml"] = []byte(svg)
	fixtures["image/x-icon"] = pngICO(fixtures["image/png"], 2, 2)
	for kind, data := range fixtures {
		t.Run(kind, func(t *testing.T) {
			if got := imageType(context.Background(), data); got != kind {
				t.Fatalf("valid image type = %q; want %q", got, kind)
			}
			if got := imageType(context.Background(), data[:len(data)/2]); got != "" {
				t.Fatalf("truncated image accepted as %q", got)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if got := imageType(ctx, data); got != "" {
				t.Fatalf("cancelled validation returned %q", got)
			}
		})
	}
	for _, bpp := range []int{1, 4, 8, 16, 24, 32} {
		for _, bitfields := range []bool{false, true} {
			if bitfields && bpp != 16 && bpp != 32 {
				continue
			}
			if got := imageType(context.Background(), dibICO(bpp, bitfields)); got != "image/x-icon" {
				t.Errorf("DIB %d-bit, bitfields=%v rejected", bpp, bitfields)
			}
		}
	}
}

func TestCorruptCandidateFallback(t *testing.T) {
	validPNG := rasterFixtures(t)["image/png"]
	for name, corrupt := range map[string][]byte{
		"PNG signature": []byte("\x89PNG\r\n\x1a\n"),
		"SVG XML":       []byte(`<svg xmlns="http://www.w3.org/2000/svg"><path></svg>`),
		"ICO pixels":    pngICO([]byte("\x89PNG\r\n\x1a\n"), 2, 2),
	} {
		for _, target := range []string{"second", "fallback", "unavailable"} {
			t.Run(name+"/"+target, func(t *testing.T) {
				var requests []string
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests = append(requests, r.URL.Path)
					data := corrupt
					switch r.URL.Path {
					case "/":
						data = []byte(`<link rel="icon" href="/first"><link rel="icon" href="/second">`)
					case "/second":
						if target == "second" {
							data = validPNG
						}
					case "/favicon.ico":
						if target == "fallback" {
							data = validPNG
						}
					}
					_, _ = w.Write(data)
				}))
				defer server.Close()
				icon, err := (&Resolver{client: server.Client()}).Resolve(context.Background(), server.URL)
				wantRequests := "/,/first,/second"
				if target != "second" {
					wantRequests += ",/favicon.ico"
				}
				if strings.Join(requests, ",") != wantRequests {
					t.Fatalf("requests = %v; want %s", requests, wantRequests)
				}
				if target == "unavailable" {
					if err != errUnavailable || len(icon.Data) != 0 {
						t.Fatalf("all corrupt: icon=%v error=%v", icon, err)
					}
				} else if err != nil || icon.Type != "image/png" || !bytes.Equal(icon.Data, validPNG) {
					t.Fatalf("alternative not returned: icon=%v error=%v", icon, err)
				}
			})
		}
	}
}

func TestSVGDocumentValidation(t *testing.T) {
	for _, data := range []string{
		`<svg xmlns="http://www.w3.org/2000/svg">`,
		`<svg xmlns="http://www.w3.org/2000/svg">&undefined;</svg>`,
		svg + svg,
		svg + "trailing text",
		`<svg xmlns="http://www.w3.org/2000/svg" width="32" width="64"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><unknown:path/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><g></svg></g>`,
		`<svg xmlns="http://www.w3.org/2000/svg" xmlns:x="urn:test" xmlns:y="urn:test" x:a="1" y:a="2"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="1000000"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="2048" height="2048"/>`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="NaN"/>`,
		`<!DOCTYPE svg><svg xmlns="http://www.w3.org/2000/svg"/>`,
		`<?xml version="1.0"?><?xml version="1.0"?>` + svg,
		`<svg xmlns="http://www.w3.org/2000/svg">` + strings.Repeat("<g>", maxXMLDepth) + strings.Repeat("</g>", maxXMLDepth) + "</svg>",
		`<svg xmlns="http://www.w3.org/2000/svg">` + strings.Repeat("<g/>", maxXMLTokens) + "</svg>",
	} {
		if got := imageType(context.Background(), []byte(data)); got != "" {
			t.Errorf("invalid SVG accepted: %.120s", data)
		}
	}
	for _, data := range []string{
		svg,
		"\ufeff" + svg,
		`<s:svg xmlns:s="http://www.w3.org/2000/svg"><s:path/></s:svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><use xlink:href="#icon"/></svg>`,
		`<?xml version="1.0"?><!-- icon --><svg xmlns="http://www.w3.org/2000/svg" width="100%" height="32px"><path d="M0 0h1v1z"/></svg><!-- end -->`,
		`<svg xmlns="http://www.w3.org/2000/svg" width="1in" height="12pt"/>`,
	} {
		if got := imageType(context.Background(), []byte(data)); got != "image/svg+xml" {
			t.Errorf("valid SVG rejected: %s", data)
		}
	}
}

func TestImageResourceLimits(t *testing.T) {
	fixtures := rasterFixtures(t)
	oversizedPNG := bytes.Clone(fixtures["image/png"])
	binary.BigEndian.PutUint32(oversizedPNG[16:], maxImageSide+1)
	binary.BigEndian.PutUint32(oversizedPNG[29:], crc32.ChecksumIEEE(oversizedPNG[12:29]))
	badICOOffset := pngICO(fixtures["image/png"], 2, 2)
	binary.LittleEndian.PutUint32(badICOOffset[18:], ^uint32(0))
	badICOPalette := dibICO(4, false)
	binary.LittleEndian.PutUint32(badICOPalette[22+32:], 1)
	badICOPalette[22+40+4] = 0xf0
	badICOMasks := dibICO(16, true)
	binary.LittleEndian.PutUint32(badICOMasks[22+44:], 0x7c00)
	for name, data := range map[string][]byte{
		"pixel dimensions": oversizedPNG,
		"body limit":       bytes.Repeat([]byte("x"), maxBody+1),
		"ICO offset":       badICOOffset,
		"ICO dimensions":   pngICO(fixtures["image/png"], 16, 16),
		"ICO mask missing": dibICO(32, false)[:22+40+16],
		"ICO palette":      badICOPalette,
		"ICO masks":        badICOMasks,
	} {
		t.Run(name, func(t *testing.T) {
			if got := imageType(context.Background(), data); got != "" {
				t.Fatalf("invalid image accepted as %q", got)
			}
		})
	}
}

func TestGIFAnimationValidation(t *testing.T) {
	frame := image.NewPaletted(image.Rect(0, 0, 2, 2), color.Palette{color.Black, color.White})
	for _, count := range []int{2, maxGIFFrames + 1} {
		animation := &gif.GIF{}
		for i := 0; i < count; i++ {
			animation.Image = append(animation.Image, frame)
			animation.Delay = append(animation.Delay, 1)
		}
		var data bytes.Buffer
		if err := gif.EncodeAll(&data, animation); err != nil {
			t.Fatal(err)
		}
		got := imageType(context.Background(), data.Bytes())
		if count == 2 && got != "image/gif" || count > maxGIFFrames && got != "" {
			t.Errorf("%d-frame GIF type = %q", count, got)
		}
		if got := imageType(context.Background(), data.Bytes()[:data.Len()-3]); got != "" {
			t.Error("GIF with damaged later frame accepted")
		}
	}
	largeFrame := image.NewPaletted(image.Rect(0, 0, 1024, 512), color.Palette{color.Black, color.White})
	var data bytes.Buffer
	if err := gif.EncodeAll(&data, &gif.GIF{Image: []*image.Paletted{largeFrame, largeFrame, largeFrame}, Delay: []int{1, 1, 1}}); err != nil {
		t.Fatal(err)
	}
	if got := imageType(context.Background(), data.Bytes()); got != "" {
		t.Error("GIF exceeding total pixel limit accepted")
	}
}

func TestICOFrameLimits(t *testing.T) {
	var pngData bytes.Buffer
	if err := png.Encode(&pngData, image.NewNRGBA(image.Rect(0, 0, 256, 256))); err != nil {
		t.Fatal(err)
	}
	for _, count := range []int{2, 16, 17, maxICOFrames + 1} {
		data := repeatedICO(pngICO(pngData.Bytes(), 0, 0), count)
		got := imageType(context.Background(), data)
		if count <= 16 && got != "image/x-icon" || count > 16 && got != "" {
			t.Errorf("%d-frame ICO type = %q", count, got)
		}
	}
	data := repeatedICO(dibICO(32, false), 2)
	binary.LittleEndian.PutUint32(data[6+16+8:], 8)
	if got := imageType(context.Background(), data); got != "" {
		t.Error("ICO with damaged second frame accepted")
	}
}

func TestImageReaderCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	reader := imageReader{ctx, bytes.NewReader([]byte("image"))}
	cancel()
	if _, err := reader.Read(make([]byte, 1)); err != context.Canceled {
		t.Fatal(err)
	}
	if _, err := reader.ReadByte(); err != context.Canceled {
		t.Fatal(err)
	}
}

func animatedWebP(still []byte, count int) []byte {
	data := make([]byte, 30)
	copy(data, "RIFF")
	copy(data[8:], "WEBPVP8X")
	binary.LittleEndian.PutUint32(data[16:], 10)
	data[20] = 2
	data = append(data, []byte("ANIM\x06\x00\x00\x00\x00\x00\x00\x00\x00\x00")...)
	for i := 0; i < count; i++ {
		chunk := make([]byte, 24)
		copy(chunk, "ANMF")
		binary.LittleEndian.PutUint32(chunk[4:], uint32(16+len(still)-12))
		chunk[20] = 10
		data = append(data, chunk...)
		data = append(data, still[12:]...)
	}
	binary.LittleEndian.PutUint32(data[4:], uint32(len(data)-8))
	return data
}

func TestAnimatedWebPValidation(t *testing.T) {
	still := rasterFixtures(t)["image/webp"]
	for _, count := range []int{1, 2, maxWebPFrames + 1} {
		data := animatedWebP(still, count)
		got := imageType(context.Background(), data)
		if count <= maxWebPFrames && got != "image/webp" || count > maxWebPFrames && got != "" {
			t.Errorf("%d-frame WebP type = %q", count, got)
		}
	}
	badFrame := animatedWebP(still, 2)
	badFrame[len(badFrame)-10] ^= 0xff
	badPosition := animatedWebP(still, 1)
	badPosition[44+8] = 1
	badChunkSize := animatedWebP(still, 1)
	binary.LittleEndian.PutUint32(badChunkSize[44+4:], ^uint32(0))
	for _, data := range [][]byte{badFrame, badPosition, badChunkSize, animatedWebP(still, 0)} {
		if got := imageType(context.Background(), data); got != "" {
			t.Fatal("damaged animated WebP accepted")
		}
	}
}

func FuzzImageValidation(f *testing.F) {
	for _, data := range [][]byte{
		[]byte(svg), []byte("\x89PNG\r\n\x1a\n"), []byte("GIF89a"),
		[]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), dibICO(4, false), dibICO(32, true),
		animatedWebP([]byte("RIFF\x0c\x00\x00\x00WEBPVP8 \x00\x00\x00\x00"), 1),
	} {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxBody {
			return
		}
		kind := imageType(context.Background(), data)
		switch kind {
		case "", "image/png", "image/jpeg", "image/gif", "image/webp", "image/x-icon", "image/svg+xml":
		default:
			t.Fatalf("unexpected image type %q", kind)
		}
	})
}
