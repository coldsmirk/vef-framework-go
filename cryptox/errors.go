package cryptox

import "errors"

var (
	ErrAtLeastOneKeyRequired        = errors.New("at least one of privateKey or publicKey must be provided")
	ErrPublicKeyRequiredForEncrypt  = errors.New("public key is required for encryption")
	ErrPrivateKeyRequiredForDecrypt = errors.New("private key is required for decryption")
	ErrPrivateKeyRequiredForSign    = errors.New("private key is required for signing")
	ErrPublicKeyRequiredForVerify   = errors.New("public key is required for verification")
	ErrFailedDecodePemBlock         = errors.New("failed to decode PEM block")
	ErrUnsupportedPemType           = errors.New("unsupported PEM type")
	ErrInvalidAesKeySize            = errors.New("invalid AES key size")
	ErrInvalidIvSizeCbc             = errors.New("invalid IV size for CBC mode")
	ErrCiphertextNotMultipleOfBlock = errors.New("ciphertext is not a multiple of the block size")
	ErrCiphertextTooShort           = errors.New("ciphertext too short")
	ErrDataEmpty                    = errors.New("data is empty")
	ErrInvalidPadding               = errors.New("invalid padding")
	ErrNotRsaPrivateKey             = errors.New("not an RSA private key")
	ErrNotRsaPublicKey              = errors.New("not an RSA public key")
	ErrNotEcdsaPrivateKey           = errors.New("not an ECDSA private key")
	ErrNotEcdsaPublicKey            = errors.New("not an ECDSA public key")
	ErrInvalidSm4KeySize            = errors.New("invalid SM4 key size")
	ErrInvalidSignature             = errors.New("invalid signature format")
)
