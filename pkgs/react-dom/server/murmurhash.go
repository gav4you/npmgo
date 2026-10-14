// murmurhash3 is translated from the copy of MurmurHash3 bundled in
// react-dom-server-legacy.node.production.js, which carries this notice:
//
//	JS Implementation of MurmurHash3 (r136) (as of May 20, 2011)
//
//	Copyright (c) 2011 Gary Court
//	Permission is hereby granted, free of charge, to any person obtaining a copy
//	of this software and associated documentation files (the "Software"), to deal
//	in the Software without restriction, including without limitation the rights
//	to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
//	copies of the Software, and to permit persons to whom the Software is
//	furnished to do so, subject to the following conditions:
//
//	The above copyright notice and this permission notice shall be included in
//	all copies or substantial portions of the Software.
//
//	THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
//	IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
//	FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
//	AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
//	LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
//	OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
//	SOFTWARE.

package server

import "unicode/utf16"

// murmurhash3 is murmurhash3_32_gc over the string's UTF-16 code units
// (charCodeAt(i) & 255, as the JavaScript reads them).
func murmurhash3(key string, seed uint32) uint32 {
	units := utf16.Encode([]rune(key))
	n := len(units)
	remainder := n & 3
	nbytes := n - remainder
	h1 := seed
	i := 0
	for i < nbytes {
		k1 := uint32(units[i]&255) | uint32(units[i+1]&255)<<8 | uint32(units[i+2]&255)<<16 | uint32(units[i+3]&255)<<24
		i += 4
		k1 *= 3432918353
		k1 = k1<<15 | k1>>17
		k1 *= 461845907
		h1 ^= k1
		h1 = h1<<13 | h1>>19
		h1 = h1*5 + 3864292196
	}
	var k1 uint32
	switch remainder {
	case 3:
		k1 ^= uint32(units[i+2]&255) << 16
		fallthrough
	case 2:
		k1 ^= uint32(units[i+1]&255) << 8
		fallthrough
	case 1:
		k1 ^= uint32(units[i] & 255)
		k1 *= 3432918353
		k1 = k1<<15 | k1>>17
		k1 *= 461845907
		h1 ^= k1
	}
	h1 ^= uint32(n)
	h1 ^= h1 >> 16
	h1 *= 2246822507
	h1 ^= h1 >> 13
	h1 *= 3266489909
	h1 ^= h1 >> 16
	return h1
}
