package favicon

import (
	"context"
	"encoding/binary"
)

func completeWebP(data []byte) bool {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" ||
		uint64(binary.LittleEndian.Uint32(data[4:]))+8 != uint64(len(data)) {
		return false
	}
	for pos := 12; pos < len(data); {
		if pos+8 > len(data) {
			return false
		}
		size := uint64(binary.LittleEndian.Uint32(data[pos+4:]))
		end := uint64(pos) + 8 + size + size%2
		if end > uint64(len(data)) || size%2 != 0 && data[end-1] != 0 {
			return false
		}
		pos = int(end)
	}
	return true
}

// x/image decodes still WebP frames only. Unwrap bounded ANMF frames as still
// images so adding validation preserves animation support without a new codec.
// Container layout: https://developers.google.com/speed/webp/docs/riff_container
func validAnimatedWebP(ctx context.Context, data []byte, canvasWidth, canvasHeight int) bool {
	frames, pixels := 0, 0
	animationSeen := false
	for pos := 30; pos < len(data) && ctx.Err() == nil; {
		size := int(binary.LittleEndian.Uint32(data[pos+4:]))
		chunk := data[pos+8 : pos+8+size]
		switch string(data[pos : pos+4]) {
		case "VP8X", "VP8 ", "VP8L", "ALPH":
			return false
		case "ANIM":
			if animationSeen || frames != 0 || len(chunk) != 6 {
				return false
			}
			animationSeen = true
		case "ANMF":
			if !animationSeen || len(chunk) < 16 || chunk[15]&0xfc != 0 {
				return false
			}
			x, y := 2*uint24(chunk), 2*uint24(chunk[3:])
			width, height := 1+uint24(chunk[6:]), 1+uint24(chunk[9:])
			frames++
			if !validDimensions(width, height) || frames > maxWebPFrames || x+width > canvasWidth || y+height > canvasHeight {
				return false
			}
			pixels += width * height
			if pixels > maxImagePixels {
				return false
			}
			frame := make([]byte, 30, 30+len(chunk)-16)
			copy(frame, "RIFF")
			copy(frame[8:], "WEBPVP8X")
			binary.LittleEndian.PutUint32(frame[16:], 10)
			if len(chunk) >= 20 && string(chunk[16:20]) == "ALPH" {
				frame[20] = 16
			}
			putUint24(frame[24:], width-1)
			putUint24(frame[27:], height-1)
			frame = append(frame, chunk[16:]...)
			binary.LittleEndian.PutUint32(frame[4:], uint32(len(frame)-8))
			if !validRaster(ctx, frame, "image/webp", width, height) {
				return false
			}
		}
		pos += 8 + size + size%2
	}
	return animationSeen && frames > 0 && ctx.Err() == nil
}

func uint24(data []byte) int {
	return int(data[0]) | int(data[1])<<8 | int(data[2])<<16
}

func putUint24(data []byte, value int) {
	data[0], data[1], data[2] = byte(value), byte(value>>8), byte(value>>16)
}
