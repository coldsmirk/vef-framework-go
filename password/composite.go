package password

import (
	"fmt"
	"strings"
)

type compositeEncoder struct {
	defaultEncoderID EncoderID
	encoders         map[EncoderID]Encoder
}

// NewCompositeEncoder creates a composite encoder that supports multiple password formats.
// The defaultEncoderID specifies which encoder to use for new passwords.
// Encoders map contains encoder ID to Encoder implementations.
func NewCompositeEncoder(defaultEncoderID EncoderID, encoders map[EncoderID]Encoder) Encoder {
	return &compositeEncoder{
		defaultEncoderID: defaultEncoderID,
		encoders:         encoders,
	}
}

func (c *compositeEncoder) Encode(password string) (string, error) {
	encoder, ok := c.encoders[c.defaultEncoderID]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrDefaultEncoderNotFound, c.defaultEncoderID)
	}

	encoded, err := encoder.Encode(password)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("{%s}%s", c.defaultEncoderID, encoded), nil
}

func (c *compositeEncoder) Matches(password, encodedPassword string) bool {
	encoderID := c.extractEncoderID(encodedPassword)
	if encoderID == EncoderID("") {
		encoderID = c.defaultEncoderID
	}

	encoder, ok := c.encoders[encoderID]
	if !ok {
		return false
	}

	rawEncoded := c.stripPrefix(encodedPassword)

	return encoder.Matches(password, rawEncoded)
}

func (c *compositeEncoder) UpgradeEncoding(encodedPassword string) bool {
	encoderID := c.extractEncoderID(encodedPassword)

	if encoderID != EncoderID("") && encoderID != c.defaultEncoderID {
		return true
	}

	encoder, ok := c.encoders[c.defaultEncoderID]
	if !ok {
		return false
	}

	rawEncoded := c.stripPrefix(encodedPassword)

	return encoder.UpgradeEncoding(rawEncoded)
}

func (c *compositeEncoder) extractEncoderID(encodedPassword string) EncoderID {
	id, _ := c.parseEncoderPrefix(encodedPassword)

	return id
}

func (c *compositeEncoder) stripPrefix(encodedPassword string) string {
	_, content := c.parseEncoderPrefix(encodedPassword)

	return content
}

// parseEncoderPrefix extracts the encoder ID and remaining content from an encoded password.
// Returns empty EncoderID and original password if no valid prefix found.
func (*compositeEncoder) parseEncoderPrefix(encodedPassword string) (EncoderID, string) {
	if !strings.HasPrefix(encodedPassword, "{") {
		return EncoderID(""), encodedPassword
	}

	end := strings.Index(encodedPassword, "}")
	if end == -1 {
		return EncoderID(""), encodedPassword
	}

	return EncoderID(encodedPassword[1:end]), encodedPassword[end+1:]
}
