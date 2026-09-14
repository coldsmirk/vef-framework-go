package middleware

import (
	"bytes"
	"fmt"

	"github.com/coldsmirk/vef-framework-go/api"
	"github.com/coldsmirk/vef-framework-go/config"
	"github.com/coldsmirk/vef-framework-go/cryptox"
)

// protectedBodyCodec owns the configured authenticated cipher and its wire
// marker. Both supported ciphers emit standard base64 containing a random
// 12-byte GCM nonce followed by ciphertext and the authentication tag.
type protectedBodyCodec struct {
	cipher   cryptox.Cipher
	encoding config.APIBodyEncoding
}

func newProtectedBodyCodec(cfg *config.APIBodyEncodingConfig) (*protectedBodyCodec, error) {
	encoding := cfg.EffectiveEncoding()

	var (
		cipher cryptox.Cipher
		err    error
	)

	switch encoding {
	case config.APIBodyEncodingAESGCMBase64:
		cipher, err = cryptox.NewAESFromBase64(cfg.Key, cryptox.WithAESMode(cryptox.AesModeGcm))
	case config.APIBodyEncodingSM4GCMBase64:
		cipher, err = cryptox.NewSM4FromBase64(cfg.Key, cryptox.WithSM4Mode(cryptox.Sm4ModeGcm))
	default:
		return nil, fmt.Errorf("%w %q", config.ErrInvalidAPIBodyEncoding, encoding)
	}

	if err != nil {
		return nil, fmt.Errorf("create %s API body codec: %w", encoding, err)
	}

	return &protectedBodyCodec{cipher: cipher, encoding: encoding}, nil
}

func (c *protectedBodyCodec) decode(raw []byte, limit int) ([]byte, error) {
	plaintext, err := c.cipher.Decrypt(string(bytes.TrimSpace(raw)))
	if err != nil {
		return nil, api.ErrBodyDecodeFailed
	}

	decoded := []byte(plaintext)
	if len(decoded) > limit {
		return nil, api.ErrBodyTooLarge
	}

	return decoded, nil
}

func (c *protectedBodyCodec) encode(raw []byte) ([]byte, error) {
	encoded, err := c.cipher.Encrypt(string(raw))
	if err != nil {
		return nil, fmt.Errorf("encode %s API response body: %w", c.encoding, err)
	}

	return []byte(encoded), nil
}
