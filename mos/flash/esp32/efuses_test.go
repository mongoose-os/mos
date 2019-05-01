package esp32

import (
	"encoding/hex"
	"fmt"
	"testing"
)

func printBlock(vv []uint32) string {
	res := "["
	for i, v := range vv {
		if i > 0 {
			res += ", "
		}
		res += fmt.Sprintf("0x%08x", v)
	}
	res += "]"
	return res
}

func TestKeyEncoding(t *testing.T) {
	cases := []struct {
		cs  KeyEncodingScheme
		in  string
		out []uint32
	}{
		// None
		{
			KeyEncodingSchemeNone,
			"0000000000000000000000000000000000000000000000000000000000000000",
			[]uint32{0, 0, 0, 0, 0, 0, 0, 0},
		}, {
			KeyEncodingSchemeNone,
			"0000000011111111222222223333333344444444555555556666666677777777",
			[]uint32{0x77777777, 0x66666666, 0x55555555, 0x44444444, 0x33333333, 0x22222222, 0x11111111, 0x00000000},
		}, {
			KeyEncodingSchemeNone,
			"0123456789abcdeff1e2d3c4b5a69788796a5b4c3d2e1f551122334455667788",
			[]uint32{0x55667788, 0x11223344, 0x3d2e1f55, 0x796a5b4c, 0xb5a69788, 0xf1e2d3c4, 0x89abcdef, 0x01234567},
		}, {
			KeyEncodingSchemeNone,
			"00000000111111112222222233333333444444445555555566666666777777",
			nil,
		}, {
			KeyEncodingSchemeNone,
			"000000001111111122222222333333334444444455555555666666667777777788",
			nil,
		},
		// 3/4
		{
			KeyEncodingScheme34,
			"000000000000000000000000000000000000000000000000",
			[]uint32{0, 0, 0, 0, 0, 0, 0, 0},
		}, {
			KeyEncodingScheme34,
			"a0a1a2a3b0b1b2b3c0c1c2c3d0d1d2d3e0e1e2e3f0f1f2f3",
			[]uint32{0xf0f1f2f3, 0x6001e2e3, 0xd2d3e0e1, 0x4f01d0d1, 0xc0c1c2c3, 0x4c01b2b3, 0xa2a3b0b1, 0x3d01a0a1},
		}, {
			KeyEncodingScheme34,
			"111111111111111111111111111111111111111111111111",
			[]uint32{0x11111111, 0x2a001111, 0x11111111, 0x2a001111, 0x11111111, 0x2a001111, 0x11111111, 0x2a001111},
		}, {
			KeyEncodingScheme34,
			"000000001111111122222222333333334444444455555555",
			[]uint32{0x55555555, 0x3e004444, 0x33334444, 0x4e003333, 0x22222222, 0x2a001111, 0x00001111, 0x06000000},
		}, {
			KeyEncodingScheme34,
			"0123456789abcdeff1e2d3c4b5a69788796a5b4c3d2e1f55",
			[]uint32{0x3d2e1f55, 0x5b4e5b4c, 0x9788796a, 0x5a1fb5a6, 0xf1e2d3c4, 0x6e26cdef, 0x456789ab, 0x3b220123},
		},
	}
	var fd fuseDescriptor
	for _, fd = range fuseDescriptors {
		if fd.name == "flash_encryption_key" {
			break
		}
	}
	for i, c := range cases {
		in, _ := hex.DecodeString(c.in)
		f := Fuse{d: fd, blocks: []*FuseBlock{
			&FuseBlock{
				data: []uint32{0, 0, 0, 0, 0, 0, 0, 0},
				diff: []uint32{0, 0, 0, 0, 0, 0, 0, 0},
			},
			&FuseBlock{
				data: []uint32{0, 0, 0, 0, 0, 0, 0, 0},
				diff: []uint32{0, 0, 0, 0, 0, 0, 0, 0},
			},
		}}
		if err := f.SetKeyValue(in, c.cs); err != nil {
			if c.out != nil {
				t.Fatalf("%d: %d %s: expected %s, got error %s", i, c.cs, c.in, printBlock(c.out), err)
			}
		} else {
			out := f.blocks[1].diff
			if c.out == nil {
				t.Fatalf("%d: %d %s: expected error, got %s", i, c.cs, c.in, printBlock(out))
			}
			for i, _ := range c.out {
				if out[i] != c.out[i] {
					t.Fatalf("%d: %d %s: \nexpected: %s,\ngot     : %s", i, c.cs, c.in, printBlock(c.out), printBlock(out))
				}
			}
		}
	}
}
