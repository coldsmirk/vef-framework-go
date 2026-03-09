package password

import (
	"github.com/coldsmirk/vef-framework-go/cryptox"
)

type cipherEncoder struct {
	cipher  cryptox.Cipher
	encoder Encoder
}

// NewCipherEncoder creates a new cipher-based password encoder that decrypts passwords before encoding.
// The cipher decrypts encrypted passwords, and the encoder performs the actual password encoding.
// Both cipher and encoder are required parameters.
func NewCipherEncoder(cipher cryptox.Cipher, encoder Encoder) Encoder {
	return &cipherEncoder{
		cipher:  cipher,
		encoder: encoder,
	}
}

func (e *cipherEncoder) Encode(password string) (string, error) {
	if e.cipher == nil {
		return "", ErrCipherRequired
	}

	if e.encoder == nil {
		return "", ErrEncoderRequired
	}

	plainPassword, err := e.cipher.Decrypt(password)
	if err != nil {
		return "", err
	}

	return e.encoder.Encode(plainPassword)
}

func (e *cipherEncoder) Matches(password, encodedPassword string) bool {
	if e.cipher == nil || e.encoder == nil {
		return false
	}

	plainPassword, err := e.cipher.Decrypt(password)
	if err != nil {
		return false
	}

	return e.encoder.Matches(plainPassword, encodedPassword)
}

func (e *cipherEncoder) UpgradeEncoding(encodedPassword string) bool {
	if e.encoder == nil {
		return false
	}

	return e.encoder.UpgradeEncoding(encodedPassword)
}
