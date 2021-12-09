package kcp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
)

type Entropy interface {
	Init()
	Fill(nonce []byte)
}

type nonceAES128 struct {
	seed  [aes.BlockSize]byte
	block cipher.Block
}

func (n *nonceAES128) Init() {
	var key [16]byte

	_, _ = io.ReadFull(rand.Reader, key[:])
	_, _ = io.ReadFull(rand.Reader, n.seed[:])
	block, _ := aes.NewCipher(key[:])
	n.block = block
}

func (n *nonceAES128) Fill(nonce []byte) {
	if n.seed[0] == 0 {
		_, _ = io.ReadFull(rand.Reader, n.seed[:])
	}

	n.block.Encrypt(n.seed[:], n.seed[:])
	copy(nonce, n.seed[:])
}
